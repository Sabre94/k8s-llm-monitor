package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	metricstypes "github.com/yourusername/k8s-llm-monitor/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// PodMetricsCollector Pod指标采集器
type PodMetricsCollector struct {
	kubeClient    *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset
	namespaces    []string // 要监控的命名空间列表
	logger        *logrus.Logger
}

// NewPodMetricsCollector 创建Pod指标采集器
func NewPodMetricsCollector(kubeClient *kubernetes.Clientset, metricsClient *metricsclientset.Clientset, namespaces []string) *PodMetricsCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 如果没有指定namespace，默认监控所有
	if len(namespaces) == 0 {
		namespaces = []string{""} // 空字符串表示所有namespace
	}

	return &PodMetricsCollector{
		kubeClient:    kubeClient,
		metricsClient: metricsClient,
		namespaces:    namespaces,
		logger:        logger,
	}
}

// CollectPodMetrics 采集所有Pod指标
func (c *PodMetricsCollector) CollectPodMetrics(ctx context.Context) (map[string]*metricstypes.PodMetrics, error) {
	c.logger.Debug("Collecting pod metricstypes...")

	result := make(map[string]*metricstypes.PodMetrics)

	// 遍历所有要监控的namespace
	for _, namespace := range c.namespaces {
		podMetrics, err := c.CollectNamespacePodMetrics(ctx, namespace)
		if err != nil {
			c.logger.Warnf("Failed to collect pod metrics for namespace %s: %v", namespace, err)
			continue
		}

		// 合并结果
		for k, v := range podMetrics {
			result[k] = v
		}
	}

	c.logger.Debugf("Successfully collected metrics for %d pods", len(result))
	return result, nil
}

// CollectNamespacePodMetrics 采集指定namespace的Pod指标
func (c *PodMetricsCollector) CollectNamespacePodMetrics(ctx context.Context, namespace string) (map[string]*metricstypes.PodMetrics, error) {
	// 1. 获取Pod列表
	pods, err := c.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	// 2. 获取Pod的实时指标
	podMetrics, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Warnf("Failed to get pod metrics from metrics server for namespace %s: %v (metrics may be incomplete)", namespace, err)
		podMetrics = &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{}}
	}

	// 3. 创建指标映射
	metricsMap := make(map[string]*metricsv1beta1.PodMetrics)
	for i := range podMetrics.Items {
		pm := &podMetrics.Items[i]
		metricsMap[pm.Name] = pm
	}

	// 4. 组合数据
	result := make(map[string]*metricstypes.PodMetrics)
	for _, pod := range pods.Items {
		podMetric := c.buildPodMetrics(&pod, metricsMap[pod.Name])
		key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		result[key] = podMetric
	}

	return result, nil
}

// buildPodMetrics 构建PodMetrics对象
func (c *PodMetricsCollector) buildPodMetrics(pod *corev1.Pod, metric *metricsv1beta1.PodMetrics) *metricstypes.PodMetrics {
	now := time.Now()

	// 计算Pod的资源请求和限制
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit int64
	for _, container := range pod.Spec.Containers {
		if req := container.Resources.Requests.Cpu(); req != nil {
			cpuRequest += req.MilliValue()
		}
		if lim := container.Resources.Limits.Cpu(); lim != nil {
			cpuLimit += lim.MilliValue()
		}
		if req := container.Resources.Requests.Memory(); req != nil {
			memoryRequest += req.Value()
		}
		if lim := container.Resources.Limits.Memory(); lim != nil {
			memoryLimit += lim.Value()
		}
	}

	// 计算Pod的实际使用量
	var cpuUsage, memoryUsage int64
	var containerMetrics []metricstypes.ContainerMetrics
	if metric != nil {
		for _, container := range metric.Containers {
			cpuUsage += container.Usage.Cpu().MilliValue()
			memoryUsage += container.Usage.Memory().Value()

			// 查找对应的container spec以获取requests/limits
			var containerCPURequest, containerCPULimit int64
			var containerMemoryRequest, containerMemoryLimit int64
			for _, c := range pod.Spec.Containers {
				if c.Name == container.Name {
					if req := c.Resources.Requests.Cpu(); req != nil {
						containerCPURequest = req.MilliValue()
					}
					if lim := c.Resources.Limits.Cpu(); lim != nil {
						containerCPULimit = lim.MilliValue()
					}
					if req := c.Resources.Requests.Memory(); req != nil {
						containerMemoryRequest = req.Value()
					}
					if lim := c.Resources.Limits.Memory(); lim != nil {
						containerMemoryLimit = lim.Value()
					}
					break
				}
			}

			containerMetrics = append(containerMetrics, metricstypes.ContainerMetrics{
				Name:          container.Name,
				CPUUsage:      container.Usage.Cpu().MilliValue(),
				MemoryUsage:   container.Usage.Memory().Value(),
				CPURequest:    containerCPURequest,
				CPULimit:      containerCPULimit,
				MemoryRequest: containerMemoryRequest,
				MemoryLimit:   containerMemoryLimit,
			})
		}
	}

	// 计算使用率（相对于limit）
	cpuUsageRate := 0.0
	if cpuLimit > 0 {
		cpuUsageRate = float64(cpuUsage) / float64(cpuLimit) * 100.0
	}

	memoryUsageRate := 0.0
	if memoryLimit > 0 {
		memoryUsageRate = float64(memoryUsage) / float64(memoryLimit) * 100.0
	}

	// 计算Pod重启次数
	var restarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		restarts += containerStatus.RestartCount
	}

	// 检查Pod是否就绪
	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	// 获取Pod启动时间
	startTime := time.Time{}
	if pod.Status.StartTime != nil {
		startTime = pod.Status.StartTime.Time
	}

	return &metricstypes.PodMetrics{
		PodName:   pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Timestamp: now,

		CPUUsage:    cpuUsage,
		MemoryUsage: memoryUsage,

		CPURequest:    cpuRequest,
		CPULimit:      cpuLimit,
		MemoryRequest: memoryRequest,
		MemoryLimit:   memoryLimit,

		CPUUsageRate:    cpuUsageRate,
		MemoryUsageRate: memoryUsageRate,

		Containers: containerMetrics,

		Phase:     string(pod.Status.Phase),
		Ready:     ready,
		Restarts:  restarts,
		StartTime: startTime,
	}
}
