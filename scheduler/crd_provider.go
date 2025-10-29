package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
)

// CRDDataProvider 从CRD获取数据的数据提供者
type CRDDataProvider struct {
	name      string
	config    *K8sConfig
	clientset *kubernetes.Clientset
	restConfig *rest.Config
	cache     *DataCache
	logger    *logrus.Logger
}

// NewCRDDataProvider 创建CRD数据提供者
func NewCRDDataProvider(name string, config *K8sConfig) (*CRDDataProvider, error) {
	// 创建Kubernetes客户端配置
	var restConfig *rest.Config
	var err error

	if config.InCluster {
		restConfig, err = rest.InClusterConfig()
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	// 创建clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	provider := &CRDDataProvider{
		name:       name,
		config:     config,
		clientset:  clientset,
		restConfig: restConfig,
		cache:      NewDataCache(config.Namespace),
		logger:     Log().WithField("provider", name).Logger,
	}

	return provider, nil
}

// Name 返回数据提供者名称
func (p *CRDDataProvider) Name() string {
	return p.name
}

// FetchData 获取数据
func (p *CRDDataProvider) FetchData(ctx context.Context, req *DataRequest) (*DataResponse, error) {
	p.logger.WithFields(logrus.Fields{
		"source":    req.Source,
		"namespace": req.Namespace,
		"fields":    req.Fields,
	}).Debug("Fetching data from CRD")

	// 检查缓存
	if cacheData := p.cache.Get(req); cacheData != nil {
		p.logger.Debug("Returning cached data")
		return cacheData, nil
	}

	// 从CRD获取UAV数据
	uavData, err := p.fetchUAVMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch UAV metrics: %w", err)
	}

	// 获取集群状态
	clusterState, err := p.fetchClusterState(ctx)
	if err != nil {
		p.logger.WithError(err).Warn("Failed to fetch cluster state")
	}

	// 构建响应
	response := &DataResponse{
		Source:       req.Source,
		Count:        len(uavData),
		UAVData:      uavData,
		ClusterState: clusterState,
		Metadata: map[string]interface{}{
			"namespace": req.Namespace,
			"source":    req.Source,
		},
		Timestamp: time.Now(),
	}

	// 缓存结果
	p.cache.Set(req, response)

	return response, nil
}

// WatchData 监听数据变化
func (p *CRDDataProvider) WatchData(ctx context.Context, callback DataCallback) error {
	p.logger.Info("Starting to watch CRD data changes")

	go func() {
		ticker := time.NewTicker(time.Second * 30) // 30秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Info("Stopping CRD data watch")
				return
			case <-ticker.C:
				// 定期获取数据并调用回调
				req := &DataRequest{
					Source:    "uav-metrics-crd",
					Namespace: p.config.Namespace,
					Fields:    []string{"id", "battery", "latency", "utilization", "gps", "radius", "status"},
				}

				data, err := p.FetchData(ctx, req)
				if err != nil {
					p.logger.WithError(err).Error("Failed to fetch data during watch")
					continue
				}

				if err := callback(data); err != nil {
					p.logger.WithError(err).Error("Data callback failed")
				}
			}
		}
	}()

	return nil
}

// fetchUAVMetrics 从CRD获取UAV指标数据
func (p *CRDDataProvider) fetchUAVMetrics(ctx context.Context, req *DataRequest) ([]UAVData, error) {
	// 这里模拟从UAVMetric CRD获取数据
	// 在实际实现中，你需要使用自定义的client来访问CRD

	// 模拟数据 - 实际应该从CRD获取
	simulatedData := []UAVData{
		{
			ID:            "uav-node-1",
			Name:          "UAV Worker Node 1",
			IPAddress:     "192.168.1.10",
			Battery:       85.5,
			Latency:       45.2,
			Utilization:   35.8,
			GPS:           [2]float64{34.043392, -118.266096},
			Radius:        480.9,
			Status:        "ready",
			LastHeartbeat: time.Now().Add(-time.Second * 10),
			Labels: map[string]string{
				"type":     "uav",
				"zone":     "zone-a",
				"hardware": "standard",
			},
		},
		{
			ID:            "uav-node-2",
			Name:          "UAV Worker Node 2",
			IPAddress:     "192.168.1.11",
			Battery:       72.3,
			Latency:       78.5,
			Utilization:   42.1,
			GPS:           [2]float64{34.044353, -118.253013},
			Radius:        399.4,
			Status:        "ready",
			LastHeartbeat: time.Now().Add(-time.Second * 15),
			Labels: map[string]string{
				"type":     "uav",
				"zone":     "zone-b",
				"hardware": "high-performance",
			},
		},
		{
			ID:            "uav-node-3",
			Name:          "UAV Worker Node 3",
			IPAddress:     "192.168.1.12",
			Battery:       91.2,
			Latency:       32.8,
			Utilization:   28.5,
			GPS:           [2]float64{34.035058, -118.247302},
			Radius:        305.6,
			Status:        "ready",
			LastHeartbeat: time.Now().Add(-time.Second * 5),
			Labels: map[string]string{
				"type":     "uav",
				"zone":     "zone-a",
				"hardware": "standard",
			},
		},
		{
			ID:            "uav-node-4",
			Name:          "UAV Worker Node 4",
			IPAddress:     "192.168.1.13",
			Battery:       68.9,
			Latency:       95.3,
			Utilization:   55.7,
			GPS:           [2]float64{34.030712, -118.272819},
			Radius:        489.6,
			Status:        "ready",
			LastHeartbeat: time.Now().Add(-time.Second * 8),
			Labels: map[string]string{
				"type":     "uav",
				"zone":     "zone-c",
				"hardware": "high-performance",
			},
		},
		{
			ID:            "uav-node-5",
			Name:          "UAV Worker Node 5",
			IPAddress:     "192.168.1.14",
			Battery:       55.4,
			Latency:       120.5,
			Utilization:   68.2,
			GPS:           [2]float64{34.050073, -118.273802},
			Radius:        447.0,
			Status:        "ready",
			LastHeartbeat: time.Now().Add(-time.Second * 20),
			Labels: map[string]string{
				"type":     "uav",
				"zone":     "zone-b",
				"hardware": "standard",
			},
		},
	}

	// 应用过滤条件
	filteredData := p.applyFilters(simulatedData, req)

	p.logger.WithField("count", len(filteredData)).Debug("Fetched UAV data from CRD")
	return filteredData, nil
}

// fetchClusterState 获取集群状态
func (p *CRDDataProvider) fetchClusterState(ctx context.Context) (*ClusterState, error) {
	// 获取节点信息
	nodes, err := p.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	totalNodes := len(nodes.Items)
	readyNodes := 0

	for _, node := range nodes.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}
	}

	// 获取Pod信息
	pods, err := p.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	totalPods := len(pods.Items)
	runningPods := 0

	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			runningPods++
		}
	}

	// 获取指标数据（需要metrics-server）
	// 这里使用模拟数据
	cpuUsage := 45.2
	memoryUsage := 62.8
	networkLatency := 15.3
	packetLoss := 0.1

	clusterState := &ClusterState{
		TotalNodes:     totalNodes,
		ReadyNodes:     readyNodes,
		TotalPods:      totalPods,
		RunningPods:    runningPods,
		CPUUsage:       cpuUsage,
		MemoryUsage:    memoryUsage,
		NetworkLatency: networkLatency,
		PacketLoss:     packetLoss,
	}

	return clusterState, nil
}

// applyFilters 应用过滤条件
func (p *CRDDataProvider) applyFilters(data []UAVData, req *DataRequest) []UAVData {
	if req.Labels == nil && len(req.Fields) == 0 {
		return data
	}

	var filtered []UAVData

	for _, uav := range data {
		// 标签过滤
		if req.Labels != nil {
			match := true
			for key, value := range req.Labels {
				if uav.Labels[key] != value {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// 字段过滤（这里简化处理）
		if len(req.Fields) > 0 {
			// 确保必要字段存在
			requiredFields := map[string]bool{
				"id":          false,
				"battery":     false,
				"latency":     false,
				"utilization": false,
				"gps":         false,
				"radius":      false,
			}

			for _, field := range req.Fields {
				if _, exists := requiredFields[field]; exists {
					requiredFields[field] = true
				}
			}

			// 简单检查：如果要求了某个字段但UAV没有对应值，则跳过
			if requiredFields["battery"] && uav.Battery == 0 {
				continue
			}
			if requiredFields["latency"] && uav.Latency == 0 {
				continue
			}
			if requiredFields["utilization"] && uav.Utilization == 0 {
				continue
			}
			if requiredFields["gps"] && (uav.GPS[0] == 0 && uav.GPS[1] == 0) {
				continue
			}
			if requiredFields["radius"] && uav.Radius == 0 {
				continue
			}
		}

		filtered = append(filtered, uav)
	}

	return filtered
}

// DataCache 数据缓存
type DataCache struct {
	namespace string
	cache     map[string]*DataCacheEntry
	logger    *logrus.Logger
}

// DataCacheEntry 数据缓存条目
type DataCacheEntry struct {
	data      *DataResponse
	timestamp time.Time
	ttl       time.Duration
}

// NewDataCache 创建数据缓存
func NewDataCache(namespace string) *DataCache {
	return &DataCache{
		namespace: namespace,
		cache:     make(map[string]*DataCacheEntry),
		logger:    Log().WithField("component", "data-cache").Logger,
	}
}

// Get 获取缓存数据
func (c *DataCache) Get(req *DataRequest) *DataResponse {
	key := c.cacheKey(req)

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Since(entry.timestamp) > entry.ttl {
		delete(c.cache, key)
		c.logger.WithField("key", key).Debug("Cache entry expired")
		return nil
	}

	c.logger.WithField("key", key).Debug("Cache hit")
	return entry.data
}

// Set 设置缓存数据
func (c *DataCache) Set(req *DataRequest, data *DataResponse) {
	key := c.cacheKey(req)

	entry := &DataCacheEntry{
		data:      data,
		timestamp: time.Now(),
		ttl:       time.Minute * 2, // 2分钟TTL
	}

	c.cache[key] = entry
	c.logger.WithField("key", key).Debug("Data cached")
}

// cacheKey 生成缓存键
func (c *DataCache) cacheKey(req *DataRequest) string {
	var builder strings.Builder
	builder.WriteString(req.Source)
	builder.WriteString(":")
	builder.WriteString(req.Namespace)

	// 添加标签到键中
	if len(req.Labels) > 0 {
		builder.WriteString(":labels:")
		for k, v := range req.Labels {
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(v)
			builder.WriteString(",")
		}
	}

	// 添加字段到键中
	if len(req.Fields) > 0 {
		builder.WriteString(":fields:")
		builder.WriteString(strings.Join(req.Fields, ","))
	}

	return builder.String()
}

// Clear 清理过期缓存
func (c *DataCache) Clear() {
	now := time.Now()
	for key, entry := range c.cache {
		if now.Sub(entry.timestamp) > entry.ttl {
			delete(c.cache, key)
		}
	}
}

// Size 返回缓存大小
func (c *DataCache) Size() int {
	return len(c.cache)
}