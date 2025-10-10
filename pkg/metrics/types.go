package metrics

import (
	"time"
)

// NodeMetrics Node 硬件指标
type NodeMetrics struct {
	NodeName  string    `json:"node_name"`
	Timestamp time.Time `json:"timestamp"`

	// CPU 指标
	CPUCapacity  int64   `json:"cpu_capacity"`   // CPU总核心数（毫核，1000=1核）
	CPUUsage     int64   `json:"cpu_usage"`      // CPU使用量（毫核）
	CPUUsageRate float64 `json:"cpu_usage_rate"` // CPU使用率 (0-100)

	// 内存指标
	MemoryCapacity  int64   `json:"memory_capacity"`   // 总内存 (bytes)
	MemoryUsage     int64   `json:"memory_usage"`      // 内存使用量 (bytes)
	MemoryUsageRate float64 `json:"memory_usage_rate"` // 内存使用率 (0-100)

	// 磁盘指标
	DiskCapacity  int64   `json:"disk_capacity"`   // 磁盘总容量 (bytes)
	DiskUsage     int64   `json:"disk_usage"`      // 磁盘使用量 (bytes)
	DiskUsageRate float64 `json:"disk_usage_rate"` // 磁盘使用率 (0-100)

	// 网络指标（来自自定义CRD或测试）
	NetworkLatency   float64 `json:"network_latency"`   // 平均延迟 (ms)
	NetworkBandwidth float64 `json:"network_bandwidth"` // 带宽 (Mbps) - 可选

	// GPU 指标（来自CRD扩展）
	GPUCount       int       `json:"gpu_count"`        // GPU数量
	GPUModels      []string  `json:"gpu_models"`       // GPU型号列表
	GPUUsage       []float64 `json:"gpu_usage"`        // 每个GPU的使用率 (0-100)
	GPUMemoryTotal []int64   `json:"gpu_memory_total"` // 每个GPU的总显存 (MB)
	GPUMemoryUsed  []int64   `json:"gpu_memory_used"`  // 每个GPU的已用显存 (MB)

	// 健康状态
	Healthy    bool     `json:"healthy"`    // 节点是否健康
	Conditions []string `json:"conditions"` // 节点异常条件（如MemoryPressure, DiskPressure等）

	// 节点标签
	Labels map[string]string `json:"labels"`

	// 扩展字段（来自CRD的自定义指标）
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// PodMetrics Pod 资源使用指标
type PodMetrics struct {
	PodName   string    `json:"pod_name"`
	Namespace string    `json:"namespace"`
	NodeName  string    `json:"node_name"`
	Timestamp time.Time `json:"timestamp"`

	// 资源使用（实际使用量）
	CPUUsage    int64 `json:"cpu_usage"`    // CPU使用量（毫核）
	MemoryUsage int64 `json:"memory_usage"` // 内存使用量 (bytes)

	// 资源限制
	CPURequest    int64 `json:"cpu_request"`    // CPU请求（毫核）
	CPULimit      int64 `json:"cpu_limit"`      // CPU限制（毫核）
	MemoryRequest int64 `json:"memory_request"` // 内存请求 (bytes)
	MemoryLimit   int64 `json:"memory_limit"`   // 内存限制 (bytes)

	// 使用率（相对于limit）
	CPUUsageRate    float64 `json:"cpu_usage_rate"`    // CPU使用率 (0-100)
	MemoryUsageRate float64 `json:"memory_usage_rate"` // 内存使用率 (0-100)

	// Container级别指标
	Containers []ContainerMetrics `json:"containers"`

	// Pod状态
	Phase      string `json:"phase"`       // Running, Pending, Failed, etc.
	Ready      bool   `json:"ready"`       // 是否就绪
	Restarts   int32  `json:"restarts"`    // 重启次数
	StartTime  time.Time `json:"start_time"` // 启动时间
}

// ContainerMetrics Container 资源使用指标
type ContainerMetrics struct {
	Name        string `json:"name"`
	CPUUsage    int64  `json:"cpu_usage"`    // CPU使用量（毫核）
	MemoryUsage int64  `json:"memory_usage"` // 内存使用量 (bytes)

	CPURequest    int64 `json:"cpu_request"`
	CPULimit      int64 `json:"cpu_limit"`
	MemoryRequest int64 `json:"memory_request"`
	MemoryLimit   int64 `json:"memory_limit"`
}

// NetworkMetrics 网络指标（Pod间通信）
type NetworkMetrics struct {
	SourcePod   string    `json:"source_pod"`
	TargetPod   string    `json:"target_pod"`
	Timestamp   time.Time `json:"timestamp"`

	// 连通性
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`

	// 延迟指标
	RTT        float64 `json:"rtt_ms"`       // 往返时延 (ms)
	PacketLoss float64 `json:"packet_loss"`  // 丢包率 (0-100)

	// 带宽（可选，需要额外测试）
	Bandwidth float64 `json:"bandwidth_mbps,omitempty"` // Mbps

	// 测试方法
	TestMethod string `json:"test_method"` // ping, http, tcp, etc.
}

// ClusterMetrics 集群整体指标摘要
type ClusterMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// 集群资源总量
	TotalNodes      int   `json:"total_nodes"`
	HealthyNodes    int   `json:"healthy_nodes"`
	TotalPods       int   `json:"total_pods"`
	RunningPods     int   `json:"running_pods"`

	// 资源汇总
	TotalCPU        int64   `json:"total_cpu"`       // 毫核
	UsedCPU         int64   `json:"used_cpu"`        // 毫核
	CPUUsageRate    float64 `json:"cpu_usage_rate"`  // 0-100

	TotalMemory     int64   `json:"total_memory"`    // bytes
	UsedMemory      int64   `json:"used_memory"`     // bytes
	MemoryUsageRate float64 `json:"memory_usage_rate"` // 0-100

	// GPU汇总（如果有）
	TotalGPUs       int     `json:"total_gpus"`
	AvailableGPUs   int     `json:"available_gpus"`

	// 健康状态
	HealthStatus    string   `json:"health_status"` // healthy, warning, critical
	Issues          []string `json:"issues,omitempty"`
}

// MetricsSnapshot 指标快照（用于时间序列存储）
type MetricsSnapshot struct {
	Timestamp      time.Time                `json:"timestamp"`
	NodeMetrics    map[string]*NodeMetrics  `json:"node_metrics"`
	PodMetrics     map[string]*PodMetrics   `json:"pod_metrics"`     // key: namespace/pod-name
	NetworkMetrics []*NetworkMetrics        `json:"network_metrics"`
	ClusterMetrics *ClusterMetrics          `json:"cluster_metrics"`
}

// GetAvailableResources 计算Node可用资源
func (n *NodeMetrics) GetAvailableResources() (cpuCores float64, memoryGB float64, diskGB float64) {
	cpuCores = float64(n.CPUCapacity-n.CPUUsage) / 1000.0
	memoryGB = float64(n.MemoryCapacity-n.MemoryUsage) / 1024 / 1024 / 1024
	diskGB = float64(n.DiskCapacity-n.DiskUsage) / 1024 / 1024 / 1024
	return
}

// IsUnderPressure 检查Node是否处于资源压力状态
func (n *NodeMetrics) IsUnderPressure() bool {
	// CPU或内存使用率超过80%认为有压力
	return n.CPUUsageRate > 80.0 || n.MemoryUsageRate > 80.0 || n.DiskUsageRate > 90.0
}

// GetResourceUtilization 获取Pod资源利用率（相对于request）
func (p *PodMetrics) GetResourceUtilization() (cpuUtil, memUtil float64) {
	if p.CPURequest > 0 {
		cpuUtil = float64(p.CPUUsage) / float64(p.CPURequest) * 100.0
	}
	if p.MemoryRequest > 0 {
		memUtil = float64(p.MemoryUsage) / float64(p.MemoryRequest) * 100.0
	}
	return
}

// IsOverLimit 检查Pod是否接近或超过资源限制
func (p *PodMetrics) IsOverLimit() bool {
	if p.CPULimit > 0 && p.CPUUsage >= int64(float64(p.CPULimit)*0.9) {
		return true
	}
	if p.MemoryLimit > 0 && p.MemoryUsage >= int64(float64(p.MemoryLimit)*0.9) {
		return true
	}
	return false
}

// GetQuality 获取网络质量评估
func (n *NetworkMetrics) GetQuality() string {
	if !n.Connected {
		return "disconnected"
	}
	if n.RTT < 10 {
		return "excellent"
	} else if n.RTT < 50 {
		return "good"
	} else if n.RTT < 100 {
		return "fair"
	}
	return "poor"
}
