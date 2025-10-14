package sources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	metricstypes "github.com/yourusername/k8s-llm-monitor/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NetworkMetricsCollector 网络指标采集器
type NetworkMetricsCollector struct {
	kubeClient *kubernetes.Clientset
	k8sClient  *k8s.Client
	namespaces []string
	logger     *logrus.Logger

	// 配置
	maxPodPairs     int           // 最大测试Pod对数量（避免过多测试）
	testTimeout     time.Duration // 单次测试超时时间
	enableAutoTest  bool          // 是否自动选择测试对象
}

// NetworkCollectorConfig 网络采集器配置
type NetworkCollectorConfig struct {
	Namespaces     []string
	MaxPodPairs    int           // 默认10对
	TestTimeout    time.Duration // 默认10秒
	EnableAutoTest bool          // 默认true
}

// NewNetworkMetricsCollector 创建网络指标采集器
func NewNetworkMetricsCollector(kubeClient *kubernetes.Clientset, k8sClient *k8s.Client, config NetworkCollectorConfig) *NetworkMetricsCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 设置默认值
	if config.MaxPodPairs == 0 {
		config.MaxPodPairs = 10
	}
	if config.TestTimeout == 0 {
		config.TestTimeout = 10 * time.Second
	}
	if len(config.Namespaces) == 0 {
		config.Namespaces = []string{"default"}
	}

	return &NetworkMetricsCollector{
		kubeClient:     kubeClient,
		k8sClient:      k8sClient,
		namespaces:     config.Namespaces,
		logger:         logger,
		maxPodPairs:    config.MaxPodPairs,
		testTimeout:    config.TestTimeout,
		enableAutoTest: config.EnableAutoTest,
	}
}

// CollectNetworkMetrics 采集网络指标
func (c *NetworkMetricsCollector) CollectNetworkMetrics(ctx context.Context) ([]*metricstypes.NetworkMetrics, error) {
	c.logger.Debug("Collecting network metrics...")

	// 1. 获取需要测试的Pod对
	podPairs, err := c.selectPodPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select pod pairs: %w", err)
	}

	if len(podPairs) == 0 {
		c.logger.Info("No pod pairs found for network testing")
		return []*metricstypes.NetworkMetrics{}, nil
	}

	c.logger.Infof("Selected %d pod pairs for network testing", len(podPairs))

	// 2. 并发测试所有Pod对
	results := make([]*metricstypes.NetworkMetrics, 0, len(podPairs))
	resultsChan := make(chan *metricstypes.NetworkMetrics, len(podPairs))
	var wg sync.WaitGroup

	// 限制并发数，避免过载
	semaphore := make(chan struct{}, 3)

	for _, pair := range podPairs {
		wg.Add(1)
		go func(p PodPair) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行测试
			metric := c.testPodPair(ctx, p)
			resultsChan <- metric
		}(pair)
	}

	// 等待所有测试完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	for metric := range resultsChan {
		if metric != nil {
			results = append(results, metric)
		}
	}

	c.logger.Infof("Network metrics collection completed: %d tests", len(results))
	return results, nil
}

// PodPair 表示需要测试的Pod对
type PodPair struct {
	SourceNamespace string
	SourcePod       string
	SourceIP        string
	TargetNamespace string
	TargetPod       string
	TargetIP        string
}

// selectPodPairs 选择需要测试的Pod对
func (c *NetworkMetricsCollector) selectPodPairs(ctx context.Context) ([]PodPair, error) {
	if !c.enableAutoTest {
		return []PodPair{}, nil
	}

	var allPods []*corev1.Pod

	// 获取所有命名空间的Pod
	for _, namespace := range c.namespaces {
		pods, err := c.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: "status.phase=Running",
		})
		if err != nil {
			c.logger.Warnf("Failed to list pods in namespace %s: %v", namespace, err)
			continue
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			// 只选择有IP的Running Pod
			if pod.Status.PodIP != "" {
				allPods = append(allPods, pod)
			}
		}
	}

	if len(allPods) < 2 {
		return []PodPair{}, nil
	}

	// 选择Pod对进行测试
	// 策略：选择不同节点、不同命名空间的Pod对，更有代表性
	pairs := []PodPair{}

	for i := 0; i < len(allPods) && len(pairs) < c.maxPodPairs; i++ {
		for j := i + 1; j < len(allPods) && len(pairs) < c.maxPodPairs; j++ {
			source := allPods[i]
			target := allPods[j]

			// 优先选择不同节点的Pod
			if source.Spec.NodeName != target.Spec.NodeName {
				pairs = append(pairs, PodPair{
					SourceNamespace: source.Namespace,
					SourcePod:       source.Name,
					SourceIP:        source.Status.PodIP,
					TargetNamespace: target.Namespace,
					TargetPod:       target.Name,
					TargetIP:        target.Status.PodIP,
				})
			}
		}
	}

	// 如果没找到跨节点的Pod对，就选择同节点的
	if len(pairs) == 0 {
		for i := 0; i < len(allPods) && len(pairs) < c.maxPodPairs; i++ {
			for j := i + 1; j < len(allPods) && len(pairs) < c.maxPodPairs; j++ {
				source := allPods[i]
				target := allPods[j]

				pairs = append(pairs, PodPair{
					SourceNamespace: source.Namespace,
					SourcePod:       source.Name,
					SourceIP:        source.Status.PodIP,
					TargetNamespace: target.Namespace,
					TargetPod:       target.Name,
					TargetIP:        target.Status.PodIP,
				})
			}
		}
	}

	return pairs, nil
}

// testPodPair 测试单个Pod对的网络连通性
func (c *NetworkMetricsCollector) testPodPair(ctx context.Context, pair PodPair) *metricstypes.NetworkMetrics {
	testCtx, cancel := context.WithTimeout(ctx, c.testTimeout)
	defer cancel()

	metric := &metricstypes.NetworkMetrics{
		SourcePod:  fmt.Sprintf("%s/%s", pair.SourceNamespace, pair.SourcePod),
		TargetPod:  fmt.Sprintf("%s/%s", pair.TargetNamespace, pair.TargetPod),
		Timestamp:  time.Now(),
		Connected:  false,
		TestMethod: "mixed",
	}

	// 使用RTT Tester进行测试
	if c.k8sClient == nil {
		metric.Error = "K8s client not available"
		return metric
	}

	tester := k8s.NewRTTTester(c.k8sClient)

	// 测试Pod连通性（包含ping和HTTP测试）
	c.logger.Debugf("Testing connectivity: %s -> %s", metric.SourcePod, metric.TargetPod)

	testResult, err := tester.TestPodConnectivity(testCtx, metric.SourcePod, metric.TargetPod)
	if err != nil {
		metric.Error = fmt.Sprintf("connectivity test failed: %v", err)
		c.logger.Warnf("Connectivity test failed for %s -> %s: %v", metric.SourcePod, metric.TargetPod, err)
		return metric
	}

	// 转换测试结果
	if testResult.SuccessRate > 0 {
		metric.Connected = true
		metric.RTT = testResult.AverageRTT

		// 从RTT结果中获取丢包率（取ping测试的丢包率）
		for _, rtt := range testResult.RTTResults {
			if rtt.Method == "ping" && rtt.Success {
				metric.PacketLoss = rtt.PacketLoss
				metric.TestMethod = "ping"
				break
			}
		}

		// 如果有HTTP测试成功，使用HTTP的RTT
		for _, rtt := range testResult.RTTResults {
			if rtt.Method == "http" && rtt.Success {
				metric.RTT = rtt.RTT
				metric.TestMethod = "http"
				break
			}
		}

		c.logger.Debugf("Test success: %s -> %s, RTT=%.2fms, Method=%s, Loss=%.1f%%",
			metric.SourcePod, metric.TargetPod, metric.RTT, metric.TestMethod, metric.PacketLoss)
	} else {
		metric.Error = "all tests failed"
		c.logger.Warnf("All tests failed for %s -> %s", metric.SourcePod, metric.TargetPod)
	}

	return metric
}

// hasHTTPService 检查Pod是否暴露HTTP服务
func (c *NetworkMetricsCollector) hasHTTPService(ctx context.Context, namespace, podName string) bool {
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// 检查容器端口
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.ContainerPort == 80 || port.ContainerPort == 8080 {
				return true
			}
		}
	}

	return false
}

// TestPodConnectivity 测试指定的Pod对（供API调用，实现NetworkMetricsSource接口）
func (c *NetworkMetricsCollector) TestPodConnectivity(ctx context.Context, sourcePod, targetPod string) (*metricstypes.NetworkMetrics, error) {
	// 解析Pod名称（namespace/pod）
	sourceNs, sourceName, err := parsePodName(sourcePod)
	if err != nil {
		return nil, fmt.Errorf("invalid source pod name: %w", err)
	}

	targetNs, targetName, err := parsePodName(targetPod)
	if err != nil {
		return nil, fmt.Errorf("invalid target pod name: %w", err)
	}

	// 获取Pod信息
	sourcePodObj, err := c.kubeClient.CoreV1().Pods(sourceNs).Get(ctx, sourceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get source pod: %w", err)
	}

	targetPodObj, err := c.kubeClient.CoreV1().Pods(targetNs).Get(ctx, targetName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get target pod: %w", err)
	}

	pair := PodPair{
		SourceNamespace: sourceNs,
		SourcePod:       sourceName,
		SourceIP:        sourcePodObj.Status.PodIP,
		TargetNamespace: targetNs,
		TargetPod:       targetName,
		TargetIP:        targetPodObj.Status.PodIP,
	}

	return c.testPodPair(ctx, pair), nil
}

// parsePodName 解析Pod名称（namespace/pod-name）
func parsePodName(fullName string) (namespace, podName string, err error) {
	parts := make([]string, 0, 2)
	current := ""
	for _, ch := range fullName {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid pod name format, expected namespace/pod-name")
	}

	return parts[0], parts[1], nil
}
