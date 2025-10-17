package models

import "time"

// SchedulingWorkload 描述待调度任务
type SchedulingWorkload struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type,omitempty"`
}

// SchedulingRequestSpec 请求规格
type SchedulingRequestSpec struct {
	Workload          SchedulingWorkload `json:"workload"`
	MinBatteryPercent float64            `json:"minBatteryPercent,omitempty"`
	PreferredNodes    []string           `json:"preferredNodes,omitempty"`
	Annotations       map[string]string  `json:"annotations,omitempty"`
	CreatedAt         *time.Time         `json:"createdAt,omitempty"`
}

// SchedulingRequestStatus 请求结果
type SchedulingRequestStatus struct {
	Phase        string     `json:"phase,omitempty"`
	AssignedNode string     `json:"assignedNode,omitempty"`
	AssignedUAV  string     `json:"assignedUAV,omitempty"`
	Score        float64    `json:"score,omitempty"`
	Message      string     `json:"message,omitempty"`
	LastUpdated  *time.Time `json:"lastUpdated,omitempty"`
}

// SchedulingCandidate 评估候选项
type SchedulingCandidate struct {
	NodeName      string
	UAVID         string
	Battery       float64
	LastHeartbeat time.Time
	Score         float64
}
