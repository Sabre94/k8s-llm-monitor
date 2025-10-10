package models

import (
	"time"
)

// PodInfo 包含Pod的基本信息
type PodInfo struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Status     string            `json:"status"`
	NodeName   string            `json:"node_name"`
	IP         string            `json:"ip"`
	Labels     map[string]string `json:"labels"`
	StartTime  time.Time         `json:"start_time"`
	Containers []ContainerInfo   `json:"containers"`
}

// ContainerInfo 包含容器信息
type ContainerInfo struct {
	Name  string            `json:"name"`
	Image string            `json:"image"`
	State string            `json:"state"`
	Ready bool              `json:"ready"`
	Env   map[string]string `json:"env"`
}

// ServiceInfo 包含服务信息
type ServiceInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	ClusterIP string            `json:"cluster_ip"`
	Ports     []ServicePort     `json:"ports"`
	Selector  map[string]string `json:"selector"`
}

// ServicePort 服务端口信息
type ServicePort struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

// EventInfo 包含事件信息
type EventInfo struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Count     int32     `json:"count"`
}

// NetworkPolicyInfo 包含网络策略信息
type NetworkPolicyInfo struct {
	Name        string              `json:"name"`
	Namespace   string              `json:"namespace"`
	PodSelector map[string]string   `json:"pod_selector"`
	Ingress     []NetworkPolicyRule `json:"ingress"`
	Egress      []NetworkPolicyRule `json:"egress"`
}

// NetworkPolicyRule 网络策略规则
type NetworkPolicyRule struct {
	Ports []PortRule `json:"ports"`
	From  []PeerRule `json:"from"`
	To    []PeerRule `json:"to"`
}

// PortRule 端口规则
type PortRule struct {
	Protocol string `json:"protocol"`
	Port     int32  `json:"port"`
}

// PeerRule 对等体规则
type PeerRule struct {
	PodSelector       map[string]string `json:"pod_selector"`
	NamespaceSelector map[string]string `json:"namespace_selector"`
}

// AnalysisRequest 分析请求
type AnalysisRequest struct {
	Type       string                 `json:"type"` // "pod_communication", "anomaly_detection", "root_cause"
	Parameters map[string]interface{} `json:"parameters"`
	Context    map[string]interface{} `json:"context"`
}

// AnalysisResponse 分析响应
type AnalysisResponse struct {
	RequestID string                 `json:"request_id"`
	Status    string                 `json:"status"` // "success", "error", "processing"
	Result    map[string]interface{} `json:"result"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// CommunicationAnalysis 通信分析结果
type CommunicationAnalysis struct {
	PodA       string   `json:"pod_a"`
	PodB       string   `json:"pod_b"`
	Status     string   `json:"status"` // "connected", "disconnected", "unknown"
	Issues     []string `json:"issues"`
	Solutions  []string `json:"solutions"`
	Confidence float64  `json:"confidence"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	OverallHealth string                 `json:"overall_health"`
	Components    map[string]interface{} `json:"components"`
	Issues        []string               `json:"issues"`
	Suggestions   []string               `json:"suggestions"`
	LastUpdate    time.Time              `json:"last_update"`
}

// CRDInfo CRD信息
type CRDInfo struct {
	Name         string            `json:"name"`
	Group        string            `json:"group"`
	Kind         string            `json:"kind"`
	Scope        string            `json:"scope"`        // Cluster or Namespaced
	Versions     []string          `json:"versions"`
	Plural       string            `json:"plural"`
	Singular     string            `json:"singular"`
	Established  bool              `json:"established"`
	Stored       bool              `json:"stored"`
	CreationTime time.Time         `json:"creation_time"`
}

// CustomResourceInfo 自定义资源信息
type CustomResourceInfo struct {
	Kind         string                 `json:"kind"`
	Name         string                 `json:"name"`
	Namespace    string                 `json:"namespace"`
	Group        string                 `json:"group"`
	Version      string                 `json:"version"`
	Spec         map[string]interface{} `json:"spec"`
	Status       map[string]interface{} `json:"status"`
	Generation   int64                  `json:"generation"`
	CreationTime time.Time              `json:"creation_time"`
	UpdateTime   time.Time              `json:"update_time"`
}

// CRDEvent CRD事件
type CRDEvent struct {
	Type        string                 `json:"type"`        // Added, Modified, Deleted
	Kind        string                 `json:"kind"`
	Group       string                 `json:"group"`
	Version     string                 `json:"version"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Object      map[string]interface{} `json:"object"`
	Timestamp   time.Time              `json:"timestamp"`
}

// RTTResult RTT测试结果
type RTTResult struct {
	Success      bool      `json:"success"`
	RTT          float64   `json:"rtt_ms"`      // RTT时间（毫秒)
	PacketLoss   float64   `json:"packet_loss"` // 丢包率（百分比）
	ErrorMessage string    `json:"error_message"`
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"` // 测试方法：ping, http, etc.
}

// NetworkTestResult 网络测试结果
type NetworkTestResult struct {
	PodA        string      `json:"pod_a"`
	PodB        string      `json:"pod_b"`
	RTTResults  []RTTResult `json:"rtt_results"`
	AverageRTT  float64     `json:"average_rtt_ms"`
	SuccessRate float64     `json:"success_rate"`
	TestCount   int         `json:"test_count"`
	Latency     string      `json:"latency_assessment"` // 延迟评估：excellent, good, poor, very_poor
}
