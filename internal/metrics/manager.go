package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/internal/metrics/sources"
	metricstypes "github.com/yourusername/k8s-llm-monitor/pkg/metrics"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
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
	uavSource     UAVMetricsSource

	// 缓存
	snapshot         *metricstypes.MetricsSnapshot
	uavSnapshot      map[string]interface{} // UAV状态快照
	uavLastHeartbeat map[string]time.Time   // UAV最后心跳时间
	snapshotMutex    sync.RWMutex

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
	Namespaces      []string      // 要监控的命名空间
	CollectInterval time.Duration // 采集间隔
	EnableNode      bool          // 是否启用节点指标采集
	EnablePod       bool          // 是否启用Pod指标采集
	EnableNetwork   bool          // 是否启用网络指标采集
	EnableCustom    bool          // 是否启用自定义指标采集
	EnableUAV       bool          // 是否启用UAV指标采集

	// 网络指标配置
	NetworkMaxPairs    int           // 网络测试最大Pod对数
	NetworkTestTimeout time.Duration // 网络测试超时时间
	K8sClient          interface{}   // K8s client（用于网络测试）
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
		interval:         config.CollectInterval,
		logger:           logger,
		stopChan:         make(chan struct{}),
		uavSnapshot:      make(map[string]interface{}),
		uavLastHeartbeat: make(map[string]time.Time),
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

	// 初始化网络指标采集器
	if config.EnableNetwork && config.K8sClient != nil {
		// 类型断言K8sClient
		if k8sClient, ok := config.K8sClient.(*k8s.Client); ok {
			networkConfig := sources.NetworkCollectorConfig{
				Namespaces:     config.Namespaces,
				MaxPodPairs:    config.NetworkMaxPairs,
				TestTimeout:    config.NetworkTestTimeout,
				EnableAutoTest: true,
			}
			manager.networkSource = sources.NewNetworkMetricsCollector(kubeClient, k8sClient, networkConfig)
			logger.Info("Network metrics collector enabled")
		} else {
			logger.Warn("Network metrics enabled but K8s client type incorrect")
		}
	}

	// 初始化UAV指标采集器
	if config.EnableUAV {
		uavConfig := sources.UAVCollectorConfig{
			Namespace: config.Namespaces[0], // 使用第一个namespace
			UAVLabel:  "app=uav-agent",
			Timeout:   5 * time.Second,
		}
		manager.uavSource = sources.NewUAVMetricsCollector(kubeClient, uavConfig)
		logger.Info("UAV metrics collector enabled")
	}

	// TODO: 自定义指标的初始化将在后续实现

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
	var nodeErr, podErr, networkErr error

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

	// 采集网络指标
	if m.networkSource != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			networkMetrics, err := m.networkSource.CollectNetworkMetrics(ctx)
			if err != nil {
				networkErr = err
				m.logger.Errorf("Failed to collect network metrics: %v", err)
				return
			}
			snapshot.NetworkMetrics = networkMetrics
		}()
	}

	// 采集UAV指标
	var uavMetrics map[string]interface{}
	if m.uavSource != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rawMetrics, err := m.uavSource.CollectUAVMetrics(ctx)
			if err != nil {
				m.logger.Errorf("Failed to collect UAV metrics: %v", err)
				return
			}
			now := time.Now().UTC()
			metrics := make(map[string]interface{}, len(rawMetrics))
			for nodeName, data := range rawMetrics {
				metrics[nodeName] = map[string]interface{}{
					"node_name":      nodeName,
					"status":         "active",
					"source":         "pull",
					"timestamp":      now,
					"last_heartbeat": now,
					"state":          data,
				}
			}
			uavMetrics = metrics
		}()
	}

	// TODO: 添加自定义指标采集

	wg.Wait()

	// 计算集群整体指标
	m.calculateClusterMetrics(snapshot)

	// 更新缓存
	m.snapshotMutex.Lock()
	m.snapshot = snapshot
	if uavMetrics != nil {
		m.uavSnapshot = uavMetrics
		if m.uavLastHeartbeat == nil {
			m.uavLastHeartbeat = make(map[string]time.Time)
		}
		for nodeName, raw := range uavMetrics {
			if entry, ok := raw.(map[string]interface{}); ok {
				switch ts := entry["last_heartbeat"].(type) {
				case time.Time:
					m.uavLastHeartbeat[nodeName] = ts
				case string:
					if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
						m.uavLastHeartbeat[nodeName] = parsed
					} else {
						m.uavLastHeartbeat[nodeName] = time.Now()
					}
				default:
					m.uavLastHeartbeat[nodeName] = time.Now()
				}
			} else {
				m.uavLastHeartbeat[nodeName] = time.Now()
			}
		}
	}
	m.snapshotMutex.Unlock()

	duration := time.Since(startTime)
	m.logger.Infof("Metrics collection completed in %v (nodes: %d, pods: %d, network: %d, uavs: %d)",
		duration, len(snapshot.NodeMetrics), len(snapshot.PodMetrics), len(snapshot.NetworkMetrics), len(uavMetrics))

	// 如果有错误，返回第一个错误
	if nodeErr != nil {
		return nodeErr
	}
	if podErr != nil {
		return podErr
	}
	if networkErr != nil {
		// 网络指标错误只记录日志，不中断采集
		m.logger.Warnf("Network metrics collection had errors: %v", networkErr)
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

// GetNetworkMetrics 获取网络指标
func (m *Manager) GetNetworkMetrics() []*metricstypes.NetworkMetrics {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()
	return m.snapshot.NetworkMetrics
}

// TestPodCommunication 测试指定Pod对的网络连通性（按需测试）
func (m *Manager) TestPodCommunication(ctx context.Context, sourcePod, targetPod string) (*metricstypes.NetworkMetrics, error) {
	if m.networkSource == nil {
		return nil, fmt.Errorf("network metrics collector not enabled")
	}

	// 直接调用接口方法
	return m.networkSource.TestPodConnectivity(ctx, sourcePod, targetPod)
}

// UpdateUAVReport 接收来自Agent的UAV状态上报
func (m *Manager) UpdateUAVReport(report *models.UAVReport) {
	if report == nil || report.NodeName == "" {
		return
	}

	reportTime := report.Timestamp
	if reportTime.IsZero() {
		reportTime = time.Now().UTC()
	}

	status := report.Status
	if status == "" {
		status = "active"
	}

	source := report.Source
	if source == "" {
		source = "agent"
	}

	entry := map[string]interface{}{
		"node_name":      report.NodeName,
		"uav_id":         report.UAVID,
		"status":         status,
		"source":         source,
		"timestamp":      reportTime,
		"last_heartbeat": reportTime,
	}

	if report.NodeIP != "" {
		entry["node_ip"] = report.NodeIP
	}

	if report.HeartbeatIntervalSeconds > 0 {
		entry["heartbeat_interval_seconds"] = report.HeartbeatIntervalSeconds
	}

	if len(report.Metadata) > 0 {
		entry["metadata"] = report.Metadata
	}

	if report.State != nil {
		stateCopy := *report.State
		entry["state"] = stateCopy
	}

	m.snapshotMutex.Lock()
	if m.uavSnapshot == nil {
		m.uavSnapshot = make(map[string]interface{})
	}
	m.uavSnapshot[report.NodeName] = entry
	if m.uavLastHeartbeat == nil {
		m.uavLastHeartbeat = make(map[string]time.Time)
	}
	m.uavLastHeartbeat[report.NodeName] = reportTime
	m.snapshotMutex.Unlock()

	m.logger.Debugf("UAV report ingested: node=%s uav=%s status=%s", report.NodeName, report.UAVID, status)
}

// GetUAVMetrics 获取所有UAV指标
func (m *Manager) GetUAVMetrics() map[string]interface{} {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()

	if len(m.uavSnapshot) == 0 {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{}, len(m.uavSnapshot))
	for key, value := range m.uavSnapshot {
		result[key] = value
	}
	return result
}

// GetSingleUAVMetrics 获取指定节点的UAV指标
func (m *Manager) GetSingleUAVMetrics(nodeName string) (interface{}, bool) {
	m.snapshotMutex.RLock()
	defer m.snapshotMutex.RUnlock()

	if m.uavSnapshot == nil {
		return nil, false
	}

	metric, exists := m.uavSnapshot[nodeName]
	if !exists {
		return nil, false
	}

	if entry, ok := metric.(map[string]interface{}); ok {
		clone := make(map[string]interface{}, len(entry))
		for key, value := range entry {
			clone[key] = value
		}
		return clone, true
	}

	return metric, true
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
