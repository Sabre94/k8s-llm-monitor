package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/pkg/uav"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// UAVMetricsCollector UAV指标采集器
type UAVMetricsCollector struct {
	kubeClient  *kubernetes.Clientset
	namespace   string
	logger      *logrus.Logger
	httpClient  *http.Client
	uavPodLabel string // 用于识别UAV Agent Pod的label
}

// UAVCollectorConfig UAV采集器配置
type UAVCollectorConfig struct {
	Namespace   string        // UAV Agent所在的namespace
	UAVLabel    string        // UAV Pod的label selector (默认: app=uav-agent)
	Timeout     time.Duration // HTTP请求超时时间
}

// NewUAVMetricsCollector 创建UAV指标采集器
func NewUAVMetricsCollector(kubeClient *kubernetes.Clientset, config UAVCollectorConfig) *UAVMetricsCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 设置默认值
	if config.Namespace == "" {
		config.Namespace = "default"
	}
	if config.UAVLabel == "" {
		config.UAVLabel = "app=uav-agent"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	return &UAVMetricsCollector{
		kubeClient:  kubeClient,
		namespace:   config.Namespace,
		logger:      logger,
		uavPodLabel: config.UAVLabel,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CollectUAVMetrics 采集所有UAV的指标
func (c *UAVMetricsCollector) CollectUAVMetrics(ctx context.Context) (map[string]interface{}, error) {
	c.logger.Debug("Collecting UAV metrics...")

	// 1. 获取所有UAV Agent Pod
	pods, err := c.kubeClient.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.uavPodLabel,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list UAV agent pods: %w", err)
	}

	if len(pods.Items) == 0 {
		c.logger.Warn("No running UAV agent pods found")
		return make(map[string]interface{}), nil
	}

	c.logger.Infof("Found %d UAV agent pods", len(pods.Items))

	// 2. 并发采集所有UAV的状态
	results := make(map[string]interface{})
	resultsChan := make(chan uavResult, len(pods.Items))
	var wg sync.WaitGroup

	for i := range pods.Items {
		wg.Add(1)
		go func(pod *corev1.Pod) {
			defer wg.Done()

			state, err := c.collectSingleUAV(ctx, pod)
			resultsChan <- uavResult{
				nodeName: pod.Spec.NodeName,
				state:    state,
				err:      err,
			}
		}(&pods.Items[i])
	}

	// 等待所有采集完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	for result := range resultsChan {
		if result.err != nil {
			c.logger.Warnf("Failed to collect UAV metrics from node %s: %v", result.nodeName, result.err)
			continue
		}
		if result.state != nil {
			results[result.nodeName] = result.state
		}
	}

	c.logger.Infof("UAV metrics collection completed: %d/%d successful", len(results), len(pods.Items))
	return results, nil
}

// uavResult 采集结果
type uavResult struct {
	nodeName string
	state    *uav.UAVState
	err      error
}

// collectSingleUAV 采集单个UAV的状态
func (c *UAVMetricsCollector) collectSingleUAV(ctx context.Context, pod *corev1.Pod) (*uav.UAVState, error) {
	// 使用Pod IP直接访问（在集群内部部署时）
	// 或者使用Headless Service访问
	podIP := pod.Status.PodIP
	if podIP == "" {
		return nil, fmt.Errorf("pod %s has no IP", pod.Name)
	}

	url := fmt.Sprintf("http://%s:9090/api/v1/state", podIP)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 解析响应
	var apiResp struct {
		Status string         `json:"status"`
		Data   *uav.UAVState  `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Data == nil {
		return nil, fmt.Errorf("no data in response")
	}

	c.logger.Debugf("Collected UAV metrics from %s (node: %s)", pod.Name, pod.Spec.NodeName)
	return apiResp.Data, nil
}

// CollectSingleUAVMetrics 采集指定节点的UAV指标
func (c *UAVMetricsCollector) CollectSingleUAVMetrics(ctx context.Context, nodeName string) (interface{}, error) {
	// 查找该节点上的UAV Agent Pod
	pods, err := c.kubeClient.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.uavPodLabel,
		FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list UAV agent pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no UAV agent found on node %s", nodeName)
	}

	return c.collectSingleUAV(ctx, &pods.Items[0])
}

// GetUAVByNode 按节点名获取UAV状态（便捷方法）
func (c *UAVMetricsCollector) GetUAVByNode(ctx context.Context, nodeName string) (interface{}, error) {
	return c.CollectSingleUAVMetrics(ctx, nodeName)
}

// GetHealthyUAVCount 获取健康的UAV数量
func (c *UAVMetricsCollector) GetHealthyUAVCount(ctx context.Context) (int, error) {
	states, err := c.CollectUAVMetrics(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, stateInterface := range states {
		if state, ok := stateInterface.(*uav.UAVState); ok {
			if state.Health.SystemStatus == "OK" {
				count++
			}
		}
	}

	return count, nil
}

// GetLowBatteryUAVs 获取低电量的UAV列表
func (c *UAVMetricsCollector) GetLowBatteryUAVs(ctx context.Context, threshold float64) ([]string, error) {
	states, err := c.CollectUAVMetrics(ctx)
	if err != nil {
		return nil, err
	}

	var lowBatteryUAVs []string
	for nodeName, stateInterface := range states {
		if state, ok := stateInterface.(*uav.UAVState); ok {
			if state.Battery.RemainingPercent < threshold {
				lowBatteryUAVs = append(lowBatteryUAVs, nodeName)
			}
		}
	}

	return lowBatteryUAVs, nil
}

// SendCommandToUAV 向指定节点的UAV发送命令
func (c *UAVMetricsCollector) SendCommandToUAV(ctx context.Context, nodeName, command string, payload interface{}) error {
	// 查找该节点上的UAV Agent Pod
	pods, err := c.kubeClient.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.uavPodLabel,
		FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", nodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to list UAV agent pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no UAV agent found on node %s", nodeName)
	}

	pod := &pods.Items[0]
	url := fmt.Sprintf("http://%s:9090/api/v1/command/%s", pod.Status.PodIP, command)

	// 创建请求
	var req *http.Request
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		req, err = http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Body = http.NoBody
		_ = body // TODO: 使用body
	} else {
		var err error
		req, err = http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("command failed with status: %d", resp.StatusCode)
	}

	return nil
}
