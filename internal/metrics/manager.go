package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/internal/metrics/sources"
	metricstypes "github.com/yourusername/k8s-llm-monitor/pkg/metrics"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Manager 统一的指标管理器
type Manager struct {
	// 数据源
	nodeSource    NodeMetricsSource
	podSource     PodMetricsSource
	networkSource NetworkMetricsSource
	customSource  CustomMetricsSource

	// 缓存
	snapshot      *metricstypes.MetricsSnapshot
	snapshotMutex sync.RWMutex

	// 配置
	interval time.Duration
	logger   *logrus.Logger

	// 控制
	stopChan chan struct{}
	running  bool
	runMutex sync.Mutex
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Namespaces       []string      // 要监控的命名空间
	CollectInterval  time.Duration // 采集间隔
	EnableNode       bool          // 是否启用节点指标采集
	EnablePod        bool          // 是否启用Pod指标采集
	EnableNetwork    bool          // 是否启用网络指标采集
	EnableCustom     bool          // 是否启用自定义指标采集
}

// NewManager 创建指标管理器
func NewManager(restConfig *rest.Config, config ManagerConfig) (*Manager, error) {
	// 创建K8s客户端
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// 创建Metrics客户端
	metricsClient, err := metricsclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	manager := &Manager{
		interval: config.CollectInterval,
		logger:   logger,
		stopChan: make(chan struct{}),
		snapshot: &metricstypes.MetricsSnapshot{
			Timestamp:      time.Now(),
			NodeMetrics:    make(map[string]*metricstypes.NodeMetrics),
			PodMetrics:     make(map[string]*metricstypes.PodMetrics),
			NetworkMetrics: []*metricstypes.NetworkMetrics{},
			ClusterMetrics: &metricstypes.ClusterMetrics{},
		},
	}

	// 初始化数据源
	if config.EnableNode {
		manager.nodeSource = sources.NewNodeMetricsCollector(kubeClient, metricsClient)
		logger.Info("Node metrics collector enabled")
	}

	if config.EnablePod {
		manager.podSource = sources.NewPodMetricsCollector(kubeClient, metricsClient, config.Namespaces)
		logger.Info("Pod metrics collector enabled")
	}

	// TODO: 网络指标和自定义指标的初始化将在后续实现

	return manager, nil
}

// Start 启动定期采集
func (m *Manager) Start(ctx context.Context) error {
	m.runMutex.Lock()
	if m.running {
		m.runMutex.Unlock()
		return fmt.Errorf("metrics manager is already running")
	}
	m.running = true
	m.runMutex.Unlock()

	m.logger.Infof("Starting metrics manager with interval: %v", m.interval)

	// 立即采集一次
	if err := m.Collect(ctx); err != nil {
		m.logger.Errorf("Initial metrics collection failed: %v", err)
	}

	// 定期采集
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Metrics manager stopped by context")
			m.runMutex.Lock()
			m.running = false
			m.runMutex.Unlock()
			return ctx.Err()

		case <-m.stopChan:
			m.logger.Info("Metrics manager stopped")
			m.runMutex.Lock()
			m.running = false
			m.runMutex.Unlock()
			return nil

		case <-ticker.C:
			if err := m.Collect(ctx); err != nil {
				m.logger.Errorf("Failed to collect metrics: %v", err)
			}
		}
	}
}

// Stop 停止采集
func (m *Manager) Stop() error {
	m.runMutex.Lock()
	defer m.runMutex.Unlock()

	if !m.running {
		return fmt.Errorf("metrics manager is not running")
	}

	close(m.stopChan)
	return nil
}

// Collect 执行一次指标采集
func (m *Manager) Collect(ctx context.Context) error {
	m.logger.Debug("Collecting metricstypes...")
	startTime := time.Now()

	snapshot := &metricstypes.MetricsSnapshot{
		Timestamp:      startTime,
		NodeMetrics:    make(map[string]*metricstypes.NodeMetrics),
		PodMetrics:     make(map[string]*metricstypes.PodMetrics),
		NetworkMetrics: []*metricstypes.NetworkMetrics{},
		ClusterMetrics: &metricstypes.ClusterMetrics{Timestamp: startTime},
	}

	var wg sync.WaitGroup
	var nodeErr, podErr error

	// 并发采集各类指标
	if m.nodeSource != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodeMetrics, err := m.nodeSource.CollectNodeMetrics(ctx)
			if err != nil {
				nodeErr = err
				m.logger.Errorf("Failed to collect node metrics: %v", err)
				return
			}
			snapshot.NodeMetrics = nodeMetrics
		}()
	}

	if m.podSource != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			podMetrics, err := m.podSource.CollectPodMetrics(ctx)
			if err != nil {
				podErr = err
				m.logger.Errorf("Failed to collect pod metrics: %v", err)
				return
			}
			snapshot.PodMetrics = podMetrics
		}()
	}

	// TODO: 添加网络和自定义指标采集

	wg.Wait()

	// 计算集群整体指标
	m.calculateClusterMetrics(snapshot)

	// 更新缓存
	m.snapshotMutex.Lock()
	m.snapshot = snapshot
	m.snapshotMutex.Unlock()

	duration := time.Since(startTime)
	m.logger.Infof("Metrics collection completed in %v (nodes: %d, pods: %d)",
		duration, len(snapshot.NodeMetrics), len(snapshot.PodMetrics))

	// 如果有错误，返回第一个错误
	if nodeErr != nil {
		return nodeErr
	}
	if podErr != nil {
		return podErr
	}

	return nil
}

// GetLatestSnapshot 获取最新的指标快照
func (m *Manager) GetLatestSnapshot() *metricstypes.MetricsSnapshot {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()
	return m.snapshot
}

// GetNodeMetrics 获取指定节点的指标
func (m *Manager) GetNodeMetrics(nodeName string) (*metricstypes.NodeMetrics, error) {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()

	if metric, exists := m.snapshot.NodeMetrics[nodeName]; exists {
		return metric, nil
	}
	return nil, fmt.Errorf("metrics not found for node: %s", nodeName)
}

// GetPodMetrics 获取指定Pod的指标
func (m *Manager) GetPodMetrics(namespace, podName string) (*metricstypes.PodMetrics, error) {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, podName)
	if metric, exists := m.snapshot.PodMetrics[key]; exists {
		return metric, nil
	}
	return nil, fmt.Errorf("metrics not found for pod: %s/%s", namespace, podName)
}

// GetClusterMetrics 获取集群整体指标
func (m *Manager) GetClusterMetrics() *metricstypes.ClusterMetrics {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()
	return m.snapshot.ClusterMetrics
}

// calculateClusterMetrics 计算集群整体指标
func (m *Manager) calculateClusterMetrics(snapshot *metricstypes.MetricsSnapshot) {
	cluster := snapshot.ClusterMetrics

	// 统计节点
	cluster.TotalNodes = len(snapshot.NodeMetrics)
	cluster.HealthyNodes = 0
	for _, node := range snapshot.NodeMetrics {
		if node.Healthy {
			cluster.HealthyNodes++
		}
	}

	// 统计Pod
	cluster.TotalPods = len(snapshot.PodMetrics)
	cluster.RunningPods = 0
	for _, pod := range snapshot.PodMetrics {
		if pod.Phase == "Running" {
			cluster.RunningPods++
		}
	}

	// 计算资源汇总
	cluster.TotalCPU = 0
	cluster.UsedCPU = 0
	cluster.TotalMemory = 0
	cluster.UsedMemory = 0
	cluster.TotalGPUs = 0
	cluster.AvailableGPUs = 0

	for _, node := range snapshot.NodeMetrics {
		cluster.TotalCPU += node.CPUCapacity
		cluster.UsedCPU += node.CPUUsage
		cluster.TotalMemory += node.MemoryCapacity
		cluster.UsedMemory += node.MemoryUsage

		cluster.TotalGPUs += node.GPUCount
		// 简单估算可用GPU（使用率<50%的GPU）
		for _, usage := range node.GPUUsage {
			if usage < 50.0 {
				cluster.AvailableGPUs++
			}
		}
	}

	// 计算使用率
	if cluster.TotalCPU > 0 {
		cluster.CPUUsageRate = float64(cluster.UsedCPU) / float64(cluster.TotalCPU) * 100.0
	}
	if cluster.TotalMemory > 0 {
		cluster.MemoryUsageRate = float64(cluster.UsedMemory) / float64(cluster.TotalMemory) * 100.0
	}

	// 判断健康状态
	cluster.Issues = []string{}
	if cluster.HealthyNodes < cluster.TotalNodes {
		cluster.Issues = append(cluster.Issues, fmt.Sprintf("%d nodes are unhealthy", cluster.TotalNodes-cluster.HealthyNodes))
	}
	if cluster.CPUUsageRate > 80 {
		cluster.Issues = append(cluster.Issues, fmt.Sprintf("High CPU usage: %.1f%%", cluster.CPUUsageRate))
	}
	if cluster.MemoryUsageRate > 80 {
		cluster.Issues = append(cluster.Issues, fmt.Sprintf("High memory usage: %.1f%%", cluster.MemoryUsageRate))
	}

	// 设置健康状态
	if len(cluster.Issues) == 0 {
		cluster.HealthStatus = "healthy"
	} else if cluster.CPUUsageRate > 90 || cluster.MemoryUsageRate > 90 || cluster.HealthyNodes < cluster.TotalNodes/2 {
		cluster.HealthStatus = "critical"
	} else {
		cluster.HealthStatus = "warning"
	}
}
