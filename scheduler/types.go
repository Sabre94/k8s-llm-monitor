package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// Algorithm 调度算法接口
type Algorithm interface {
	// Name 返回算法名称
	Name() string

	// Optimize 执行优化，返回最优节点集合
	Optimize(ctx context.Context, req *OptimizeRequest) (*OptimizeResult, error)

	// Validate 验证算法配置
	Validate(config map[string]interface{}) error
}

// DataProvider 数据提供者接口
type DataProvider interface {
	// Name 返回数据提供者名称
	Name() string

	// FetchData 获取数据
	FetchData(ctx context.Context, req *DataRequest) (*DataResponse, error)

	// WatchData 监听数据���化
	WatchData(ctx context.Context, callback DataCallback) error
}

// PodBinder Pod绑定器接口
type PodBinder interface {
	// BindPod 绑定Pod到节点
	BindPod(ctx context.Context, podName, nodeName string) error

	// BindPodGroup 绑定Pod组到节点集合
	BindPodGroup(ctx context.Context, bindings []PodBinding) error

	// BindNodeSet 绑定任务到节点集合 (核心功能)
	BindNodeSet(ctx context.Context, podName, podNamespace string, nodeSet []string, optResult *OptimizeResult) error
}

// OptimizeRequest 优化请求
type OptimizeRequest struct {
	// 任务信息
	TaskType        string            `json:"task_type"`
	Priority        string            `json:"priority"`

	// 优化目标
	Objectives      []string          `json:"objectives"`
	TargetCoverage  float64           `json:"target_coverage"`
	MaxUAVs         int               `json:"max_uavs"`

	// 约束条件
	Constraints     map[string]interface{} `json:"constraints"`

	// 输入数据
	UAVData         []UAVData          `json:"uav_data"`

	// 请求时间
	Timestamp       time.Time          `json:"timestamp"`

	// 扩展配置
	Extra           map[string]interface{} `json:"extra"`
}

// UAVData 无人机数据
type UAVData struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	IPAddress       string    `json:"ip_address"`

	// 性能指标
	Battery         float64   `json:"battery"`
	Latency         float64   `json:"latency"`
	Utilization     float64   `json:"utilization"`

	// 位置信息
	GPS             [2]float64 `json:"gps"`
	Radius          float64   `json:"radius"`

	// 状态信息
	Status          string    `json:"status"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`

	// 扩展字段
	Labels          map[string]string `json:"labels"`
	Annotations     map[string]string `json:"annotations"`
}

// OptimizeResponse 优化响应
type OptimizeResult struct {
	// 算法信息
	AlgorithmName   string            `json:"algorithm_name"`
	ExecutionTime   time.Duration     `json:"execution_time"`

	// 优化结果
	SelectedNodes   []string          `json:"selected_nodes"`
	Score           float64           `json:"score"`
	Objectives      map[string]float64 `json:"objectives"`

	// 覆盖信息
	CoverageArea    float64           `json:"coverage_area"`
	CoverageRatio   float64           `json:"coverage_ratio"`

	// Pareto前沿（多目标优化）
	ParetoFront     []ParetoSolution  `json:"pareto_front,omitempty"`

	// 元数据
	Metadata        map[string]interface{} `json:"metadata"`

	// 时间戳
	Timestamp       time.Time          `json:"timestamp"`
}

// ParetoSolution Pareto解
type ParetoSolution struct {
	SelectedNodes   []string          `json:"selected_nodes"`
	Objectives      map[string]float64 `json:"objectives"`
	Score           float64           `json:"score"`
	CoverageRatio   float64           `json:"coverage_ratio"`
	Rank            int               `json:"rank"`
	CrowdingDistance float64          `json:"crowding_distance"`
}

// DataRequest 数据请求
type DataRequest struct {
	// 数据源类型
	Source          string            `json:"source"`

	// 过滤条件
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels"`

	// 数据字段
	Fields          []string          `json:"fields"`

	// 时间范围
	Since           *time.Time        `json:"since,omitempty"`
	Until           *time.Time        `json:"until,omitempty"`

	// 扩展参数
	Extra           map[string]interface{} `json:"extra"`
}

// DataResponse 数据响应
type DataResponse struct {
	// 数据信息
	Source          string            `json:"source"`
	Count           int               `json:"count"`

	// UAV数据
	UAVData         []UAVData          `json:"uav_data"`

	// 集群状态
	ClusterState    *ClusterState     `json:"cluster_state,omitempty"`

	// 元数据
	Metadata        map[string]interface{} `json:"metadata"`

	// 时间戳
	Timestamp       time.Time          `json:"timestamp"`
}

// ClusterState 集群状态
type ClusterState struct {
	TotalNodes      int               `json:"total_nodes"`
	ReadyNodes      int               `json:"ready_nodes"`
	TotalPods       int               `json:"total_pods"`
	RunningPods     int               `json:"running_pods"`

	// 资源使用情况
	CPUUsage        float64           `json:"cpu_usage"`
	MemoryUsage     float64           `json:"memory_usage"`

	// 网络状态
	NetworkLatency  float64           `json:"network_latency"`
	PacketLoss      float64           `json:"packet_loss"`
}

// PodBinding Pod绑定信息
type PodBinding struct {
	PodName         string            `json:"pod_name"`
	PodNamespace    string            `json:"pod_namespace"`
	NodeName        string            `json:"node_name"`
	Reason          string            `json:"reason"`
}

// DataCallback 数据变化回调函数
type DataCallback func(data *DataResponse) error

// Scheduler 调度器主接口
type Scheduler interface {
	// Start 启动调度器
	Start(ctx context.Context) error

	// Stop 停止调度器
	Stop() error

	// SchedulePod 调度单个Pod
	SchedulePod(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error)

	// SchedulePodGroup 调度Pod组
	SchedulePodGroup(ctx context.Context, req *ScheduleGroupRequest) (*ScheduleGroupResult, error)

	// RegisterAlgorithm 注册算法
	RegisterAlgorithm(algorithm Algorithm) error

	// RegisterDataProvider 注册数据提供者
	RegisterDataProvider(provider DataProvider) error

	// SetPodBinder 设置Pod绑定器
	SetPodBinder(binder PodBinder) error

	// GetStatus 获取调度器状态
	GetStatus() *SchedulerStatus
}

// ScheduleRequest 调度请求
type ScheduleRequest struct {
	PodName         string            `json:"pod_name"`
	PodNamespace    string            `json:"pod_namespace"`
	AlgorithmName   string            `json:"algorithm_name"`
	Requirements    *OptimizeRequest  `json:"requirements"`
	Options         map[string]interface{} `json:"options"`
}

// ScheduleGroupRequest 调度组请求
type ScheduleGroupRequest struct {
	Pods            []PodRef          `json:"pods"`
	AlgorithmName   string            `json:"algorithm_name"`
	Requirements    *OptimizeRequest  `json:"requirements"`
	Options         map[string]interface{} `json:"options"`
}

// PodRef Pod引用
type PodRef struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Priority        int               `json:"priority"`
	Requirements    map[string]interface{} `json:"requirements"`
}

// ScheduleResult 调度结果
type ScheduleResult struct {
	Success         bool              `json:"success"`
	PodName         string            `json:"pod_name"`
	AssignedNode    string            `json:"assigned_node,omitempty"`        // 单节点绑定（兼容性）
	AssignedNodeSet []string          `json:"assigned_node_set,omitempty"`    // 节点集合绑定（核心功能）
	AlgorithmName   string            `json:"algorithm_name"`
	ExecutionTime   time.Duration     `json:"execution_time"`
	Reason          string            `json:"reason"`
	Message         string            `json:"message"`
	OptimizationResult *OptimizeResult `json:"optimization_result,omitempty"`
}

// ScheduleGroupResult 调度组结果
type ScheduleGroupResult struct {
	Success         bool              `json:"success"`
	TotalPods       int               `json:"total_pods"`
	SuccessfulPods  int               `json:"successful_pods"`
	Bindings        []PodBinding      `json:"bindings"`
	AlgorithmName   string            `json:"algorithm_name"`
	ExecutionTime   time.Duration     `json:"execution_time"`
	OptimizationResult *OptimizeResult `json:"optimization_result,omitempty"`
	Errors          []string          `json:"errors,omitempty"`
}

// SchedulerStatus 调度器状态
type SchedulerStatus struct {
	Status          string            `json:"status"`
	StartTime       time.Time          `json:"start_time"`
	TotalSchedules  int64             `json:"total_schedules"`
	SuccessRate     float64           `json:"success_rate"`

	// 算法状态
	Algorithms      map[string]AlgorithmStatus `json:"algorithms"`

	// 数据提供者状态
	DataProviders   map[string]ProviderStatus     `json:"data_providers"`

	// 缓存状态
	CacheSize       int               `json:"cache_size"`
	CacheHitRate    float64           `json:"cache_hit_rate"`
}

// AlgorithmStatus 算法状态
type AlgorithmStatus struct {
	Name            string            `json:"name"`
	Enabled         bool              `json:"enabled"`
	TotalRuns       int64             `json:"total_runs"`
	AvgRuntime      time.Duration     `json:"avg_runtime"`
	SuccessRate     float64           `json:"success_rate"`
	LastError       string            `json:"last_error,omitempty"`
	LastRun         time.Time         `json:"last_run"`
}

// ProviderStatus 数据提供者状态
type ProviderStatus struct {
	Name            string            `json:"name"`
	Enabled         bool              `json:"enabled"`
	Connected       bool              `json:"connected"`
	LastUpdate      time.Time         `json:"last_update"`
	DataCount       int               `json:"data_count"`
	LastError       string            `json:"last_error,omitempty"`
}

// Config 调度器配置
type Config struct {
	// 基础配置
	Name            string            `yaml:"name" json:"name"`
	ListenAddr      string            `yaml:"listen_addr" json:"listen_addr"`

	// 算法配置
	Algorithms      map[string]interface{} `yaml:"algorithms" json:"algorithms"`

	// 数据提供者配置
	DataProviders   map[string]interface{} `yaml:"data_providers" json:"data_providers"`

	// 缓存配置
	Cache           CacheConfig       `yaml:"cache" json:"cache"`

	// 日志配置
	Logging         LoggingConfig     `yaml:"logging" json:"logging"`

	// Kubernetes配置
	K8sConfig       K8sConfig         `yaml:"k8s" json:"k8s"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled         bool              `yaml:"enabled" json:"enabled"`
	TTL             time.Duration     `yaml:"ttl" json:"ttl"`
	MaxSize         int               `yaml:"max_size" json:"max_size"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level           string            `yaml:"level" json:"level"`
	Format          string            `yaml:"format" json:"format"`
	Output          string            `yaml:"output" json:"output"`
}

// K8sConfig Kubernetes配置
type K8sConfig struct {
	Kubeconfig      string            `yaml:"kubeconfig" json:"kubeconfig"`
	Namespace       string            `yaml:"namespace" json:"namespace"`
	InCluster       bool              `yaml:"in_cluster" json:"in_cluster"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Name:           "uav-scheduler",
		ListenAddr:     ":8080",
		Algorithms:     make(map[string]interface{}),
		DataProviders:  make(map[string]interface{}),
		Cache: CacheConfig{
			Enabled:        true,
			TTL:            time.Minute * 5,
			MaxSize:        1000,
		},
		Logging: LoggingConfig{
			Level:          "info",
			Format:         "json",
			Output:         "stdout",
		},
		K8sConfig: K8sConfig{
			Namespace:      "default",
			InCluster:      true,
		},
	}
}

// Error 错误定义
var (
	ErrAlgorithmNotFound = fmt.Errorf("algorithm not found")
	ErrDataProviderNotFound = fmt.Errorf("data provider not found")
	ErrInvalidRequest = fmt.Errorf("invalid request")
	ErrNoAvailableNodes = fmt.Errorf("no available nodes")
	ErrOptimizationFailed = fmt.Errorf("optimization failed")
	ErrPodBindingFailed = fmt.Errorf("pod binding failed")
)

// Log 日志工具
func Log() *logrus.Logger {
	return logrus.StandardLogger()
}

// ToJSON 转换为JSON字符串
func ToJSON(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}