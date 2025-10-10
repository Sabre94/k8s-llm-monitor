package models

import (
	"time"
)

// NodeMetrics 节点硬件指标
type NodeMetrics struct {
	NodeName  string    `json:"node_name"`
	Timestamp time.Time `json:"timestamp"`

	// CPU指标
	CPUCapacity  int64   `json:"cpu_capacity"`   // CPU总核心数（毫核，1000=1核）
	CPUUsage     int64   `json:"cpu_usage"`      // CPU使用量（毫核）
	CPUUsageRate float64 `json:"cpu_usage_rate"` // CPU使用率 (0-100)

	// 内存指标
	MemoryCapacity  int64   `json:"memory_capacity"`   // 总内存 (bytes)
	MemoryUsage     int64   `json:"memory_usage"`      // 内存使用量 (bytes)
	MemoryUsageRate float64 `json:"memory_usage_rate"` // 内存使用率 (0-100)

	// 网络指标（来自RTT测试或CRD）
	NetworkLatency   float64 `json:"network_latency"`   // 平均延迟 (ms)
	NetworkBandwidth float64 `json:"network_bandwidth"` // 带宽 (Mbps) - 可选

	// GPU指标（来自CRD）
	GPUCount       int       `json:"gpu_count"`        // GPU数量
	GPUModels      []string  `json:"gpu_models"`       // GPU型号列表
	GPUUsage       []float64 `json:"gpu_usage"`        // 每个GPU的使用率 (0-100)
	GPUMemoryTotal []int64   `json:"gpu_memory_total"` // 每个GPU的总显存 (bytes)
	GPUMemoryUsed  []int64   `json:"gpu_memory_used"`  // 每个GPU的已用显存 (bytes)

	// 磁盘指标（来自CRD或Node API）
	DiskCapacity  int64   `json:"disk_capacity"`   // 磁盘总容量 (bytes)
	DiskUsage     int64   `json:"disk_usage"`      // 磁盘使用量 (bytes)
	DiskUsageRate float64 `json:"disk_usage_rate"` // 磁盘使用率 (0-100)
	DiskIOPS      float64 `json:"disk_iops"`       // 磁盘IOPS - 可选

	// 健康状态
	Healthy    bool     `json:"healthy"`    // 节点是否健康
	Conditions []string `json:"conditions"` // 节点异常条件（如MemoryPressure, DiskPressure等）

	// 节点标签（用于调度约束）
	Labels map[string]string `json:"labels"`

	// 扩展字段（来自CRD的自定义指标）
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// NodeScore 节点评分
type NodeScore struct {
	NodeName  string    `json:"node_name"`
	Timestamp time.Time `json:"timestamp"`

	// 总分 (0-100)
	TotalScore float64 `json:"total_score"`

	// 分项得分 (0-100)
	CPUScore     float64 `json:"cpu_score"`
	MemoryScore  float64 `json:"memory_score"`
	NetworkScore float64 `json:"network_score"`
	GPUScore     float64 `json:"gpu_score"`
	DiskScore    float64 `json:"disk_score"`

	// 评分详情
	Available bool   `json:"available"` // 是否可用于调度
	Reason    string `json:"reason"`    // 评分理由或不可用原因
}

// ScheduleRequest 调度请求
type ScheduleRequest struct {
	// 工作负载类型（用于选择合适的评分策略）
	WorkloadType string `json:"workload_type"` // 如: "cpu-intensive", "gpu-intensive", "balanced"

	// 需要的节点数量（0表示根据评分动态决定）
	RequiredNodes int `json:"required_nodes"`

	// 调度策略名称
	Strategy string `json:"strategy"` // 如: "best_fit", "weighted_round_robin", "load_balancing"

	// 资源约束
	Constraints ResourceConstraints `json:"constraints"`

	// 偏好设置（可选）
	Preferences map[string]interface{} `json:"preferences,omitempty"`

	// 是否只返回评分，不实际分配
	DryRun bool `json:"dry_run"`
}

// ResourceConstraints 资源约束
type ResourceConstraints struct {
	// 最小资源要求
	MinCPUCores  int `json:"min_cpu_cores"`  // 最小CPU核心数
	MinMemoryGB  int `json:"min_memory_gb"`  // 最小内存（GB）
	MinGPUs      int `json:"min_gpus"`       // 最小GPU数量
	MinDiskGB    int `json:"min_disk_gb"`    // 最小磁盘空间（GB）
	MaxLatencyMs int `json:"max_latency_ms"` // 最大网络延迟（毫秒）

	// 必须满足的条件
	RequireGPU bool `json:"require_gpu"` // 是否必须有GPU

	// 节点选择器（标签匹配）
	NodeLabels map[string]string `json:"node_labels,omitempty"`

	// 排除的节点
	ExcludeNodes []string `json:"exclude_nodes,omitempty"`
}

// ScheduleResult 调度结果
type ScheduleResult struct {
	// 分配的节点列表
	AllocatedNodes []string `json:"allocated_nodes"`

	// 每个节点的评分
	Scores map[string]float64 `json:"scores"`

	// 所有节点的详细评分（包括未选中的）
	DetailedScores map[string]*NodeScore `json:"detailed_scores,omitempty"`

	// 使用的调度策略
	Strategy string `json:"strategy"`

	// 调度理由
	Reason string `json:"reason"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 是否成功
	Success bool `json:"success"`

	// 错误信息（如果失败）
	Error string `json:"error,omitempty"`
}

// ScoringWeights 评分权重配置
type ScoringWeights struct {
	CPU     float64 `json:"cpu" mapstructure:"cpu"`
	Memory  float64 `json:"memory" mapstructure:"memory"`
	Network float64 `json:"network" mapstructure:"network"`
	GPU     float64 `json:"gpu" mapstructure:"gpu"`
	Disk    float64 `json:"disk" mapstructure:"disk"`
}

// Validate 验证权重总和是否为1.0
func (w *ScoringWeights) Validate() bool {
	total := w.CPU + w.Memory + w.Network + w.GPU + w.Disk
	// 允许0.01的误差
	return total >= 0.99 && total <= 1.01
}

// Normalize 归一化权重使其总和为1.0
func (w *ScoringWeights) Normalize() {
	total := w.CPU + w.Memory + w.Network + w.GPU + w.Disk
	if total > 0 {
		w.CPU /= total
		w.Memory /= total
		w.Network /= total
		w.GPU /= total
		w.Disk /= total
	}
}

// GetAvailableResources 计算节点可用资源
func (m *NodeMetrics) GetAvailableResources() (cpuCores float64, memoryGB float64, diskGB float64) {
	cpuCores = float64(m.CPUCapacity-m.CPUUsage) / 1000.0 // 转换为核心数
	memoryGB = float64(m.MemoryCapacity-m.MemoryUsage) / 1024 / 1024 / 1024
	diskGB = float64(m.DiskCapacity-m.DiskUsage) / 1024 / 1024 / 1024
	return
}

// MeetsConstraints 检查节点是否满足资源约束
func (m *NodeMetrics) MeetsConstraints(constraints ResourceConstraints) bool {
	// 检查健康状态
	if !m.Healthy {
		return false
	}

	// 检查CPU
	availableCPU := float64(m.CPUCapacity-m.CPUUsage) / 1000.0
	if availableCPU < float64(constraints.MinCPUCores) {
		return false
	}

	// 检查内存
	availableMemoryGB := float64(m.MemoryCapacity-m.MemoryUsage) / 1024 / 1024 / 1024
	if availableMemoryGB < float64(constraints.MinMemoryGB) {
		return false
	}

	// 检查GPU
	if constraints.RequireGPU && m.GPUCount == 0 {
		return false
	}
	if m.GPUCount < constraints.MinGPUs {
		return false
	}

	// 检查磁盘
	if constraints.MinDiskGB > 0 {
		availableDiskGB := float64(m.DiskCapacity-m.DiskUsage) / 1024 / 1024 / 1024
		if availableDiskGB < float64(constraints.MinDiskGB) {
			return false
		}
	}

	// 检查网络延迟
	if constraints.MaxLatencyMs > 0 && m.NetworkLatency > float64(constraints.MaxLatencyMs) {
		return false
	}

	// 检查节点标签
	if len(constraints.NodeLabels) > 0 {
		for key, value := range constraints.NodeLabels {
			if m.Labels[key] != value {
				return false
			}
		}
	}

	return true
}
