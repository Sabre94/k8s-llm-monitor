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

// NodeMetricsCollector Node指标采集器（使用K8s Metrics Server）
type NodeMetricsCollector struct {
	kubeClient    *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset
	logger        *logrus.Logger
}

// NewNodeMetricsCollector 创建Node指标采集器
func NewNodeMetricsCollector(kubeClient *kubernetes.Clientset, metricsClient *metricsclientset.Clientset) *NodeMetricsCollector {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &NodeMetricsCollector{
		kubeClient:    kubeClient,
		metricsClient: metricsClient,
		logger:        logger,
	}
}

// CollectNodeMetrics 采集所有节点的指标
func (c *NodeMetricsCollector) CollectNodeMetrics(ctx context.Context) (map[string]*metricstypes.NodeMetrics, error) {
	c.logger.Debug("Collecting node metricstypes...")

	// 1. 获取所有节点的基本信息
	nodes, err := c.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// 2. 获取节点的实时指标（CPU、内存使用情况）
	nodeMetrics, err := c.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Warnf("Failed to get node metrics from metrics server: %v (metrics may be incomplete)", err)
		// 如果Metrics Server不可用，仍然可以返回基础信息
		nodeMetrics = &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{}}
	}

	// 3. 创建指标映射（方便查找）
	metricsMap := make(map[string]*metricsv1beta1.NodeMetrics)
	for i := range nodeMetrics.Items {
		nm := &nodeMetrics.Items[i]
		metricsMap[nm.Name] = nm
	}

	// 4. 组合数据
	result := make(map[string]*metricstypes.NodeMetrics)
	for _, node := range nodes.Items {
		nodeMetric := c.buildNodeMetrics(&node, metricsMap[node.Name])
		result[node.Name] = nodeMetric
	}

	c.logger.Debugf("Successfully collected metrics for %d nodes", len(result))
	return result, nil
}

// CollectSingleNodeMetrics 采集单个节点的指标
func (c *NodeMetricsCollector) CollectSingleNodeMetrics(ctx context.Context, nodeName string) (*metricstypes.NodeMetrics, error) {
	// 1. 获取节点基本信息
	node, err := c.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// 2. 获取节点实时指标
	var nodeMetric *metricsv1beta1.NodeMetrics
	nm, err := c.metricsClient.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		c.logger.Warnf("Failed to get metrics for node %s: %v", nodeName, err)
	} else {
		nodeMetric = nm
	}

	// 3. 构建指标
	result := c.buildNodeMetrics(node, nodeMetric)
	return result, nil
}

// buildNodeMetrics 构建NodeMetrics对象
func (c *NodeMetricsCollector) buildNodeMetrics(node *corev1.Node, metric *metricsv1beta1.NodeMetrics) *metricstypes.NodeMetrics {
	now := time.Now()

	// CPU容量（转换为毫核）
	cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
	// 内存容量（bytes）
	memoryCapacity := node.Status.Capacity.Memory().Value()
	// 磁盘容量（ephemeral-storage）
	diskCapacity := int64(0)
	if ephemeralStorage := node.Status.Capacity.StorageEphemeral(); ephemeralStorage != nil {
		diskCapacity = ephemeralStorage.Value()
	}

	// CPU和内存使用情况
	cpuUsage := int64(0)
	memoryUsage := int64(0)
	if metric != nil {
		cpuUsage = metric.Usage.Cpu().MilliValue()
		memoryUsage = metric.Usage.Memory().Value()
	}

	// 磁盘使用情况（从allocatable计算）
	diskUsage := int64(0)
	if ephemeralStorage := node.Status.Allocatable.StorageEphemeral(); ephemeralStorage != nil {
		// 使用 capacity - allocatable 作为已使用量的估算
		diskUsage = diskCapacity - ephemeralStorage.Value()
		if diskUsage < 0 {
			diskUsage = 0
		}
	}

	// 计算使用率
	cpuUsageRate := 0.0
	if cpuCapacity > 0 {
		cpuUsageRate = float64(cpuUsage) / float64(cpuCapacity) * 100.0
	}

	memoryUsageRate := 0.0
	if memoryCapacity > 0 {
		memoryUsageRate = float64(memoryUsage) / float64(memoryCapacity) * 100.0
	}

	diskUsageRate := 0.0
	if diskCapacity > 0 {
		diskUsageRate = float64(diskUsage) / float64(diskCapacity) * 100.0
	}

	// 检查节点健康状态
	healthy := true
	var conditions []string
	for _, condition := range node.Status.Conditions {
		// Ready条件应该为True
		if condition.Type == corev1.NodeReady {
			if condition.Status != corev1.ConditionTrue {
				healthy = false
				conditions = append(conditions, fmt.Sprintf("NotReady: %s", condition.Message))
			}
		} else {
			// 其他压力条件应该为False
			if condition.Status == corev1.ConditionTrue {
				if condition.Type == corev1.NodeMemoryPressure ||
					condition.Type == corev1.NodeDiskPressure ||
					condition.Type == corev1.NodePIDPressure ||
					condition.Type == corev1.NodeNetworkUnavailable {
					healthy = false
					conditions = append(conditions, fmt.Sprintf("%s: %s", condition.Type, condition.Message))
				}
			}
		}
	}

	// 获取节点标签
	labels := make(map[string]string)
	for k, v := range node.Labels {
		labels[k] = v
	}

	return &metricstypes.NodeMetrics{
		NodeName:  node.Name,
		Timestamp: now,

		CPUCapacity:  cpuCapacity,
		CPUUsage:     cpuUsage,
		CPUUsageRate: cpuUsageRate,

		MemoryCapacity:  memoryCapacity,
		MemoryUsage:     memoryUsage,
		MemoryUsageRate: memoryUsageRate,

		DiskCapacity:  diskCapacity,
		DiskUsage:     diskUsage,
		DiskUsageRate: diskUsageRate,

		// 网络指标暂时为0，后续通过网络测试或CRD补充
		NetworkLatency:   0,
		NetworkBandwidth: 0,

		// GPU指标暂时为0，后续通过CRD补充
		GPUCount:       0,
		GPUModels:      []string{},
		GPUUsage:       []float64{},
		GPUMemoryTotal: []int64{},
		GPUMemoryUsed:  []int64{},

		Healthy:    healthy,
		Conditions: conditions,
		Labels:     labels,

		CustomMetrics: make(map[string]interface{}),
	}
}
