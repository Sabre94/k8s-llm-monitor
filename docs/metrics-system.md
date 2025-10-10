# 指标采集系统文档

## 概述

为 K8s LLM Monitor 实现了一个独立的指标采集系统，作为智能观测平台的数据基础设施。该系统为 LLM 分析提供完整的集群可观测性数据。

## 架构设计

### 核心理念

指标采集系统作为**独立的基础模块**，与调度器、LLM 分析等上层功能解耦，遵循单一职责原则。

```
┌─────────────────────────────────────────────────────┐
│              上层应用                                │
│  - LLM 智能分析                                      │
│  - 自然语言查询                                       │
│  - 调度器（未来）                                     │
│  - API 查询                                          │
└─────────────────────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────┐
│          Metrics Manager (统一管理器)                │
│  - 协调各数据源                                       │
│  - 定期采集                                          │
│  - 缓存管理                                          │
│  - 集群汇总                                          │
└─────────────────────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────┐
│              数据源 (Sources)                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │   Node   │  │   Pod    │  │ Network  │          │
│  │ Metrics  │  │ Metrics  │  │ Metrics  │          │
│  └──────────┘  └──────────┘  └──────────┘          │
└─────────────────────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────┐
│          Kubernetes API / Metrics Server             │
└─────────────────────────────────────────────────────┘
```

## 已实现功能

### 1. 数据模型 (`internal/metrics/models.go`)

#### NodeMetrics - 节点硬件指标
```go
type NodeMetrics struct {
    NodeName  string
    Timestamp time.Time

    // CPU 指标
    CPUCapacity  int64   // CPU总核心数（毫核）
    CPUUsage     int64   // CPU使用量（毫核）
    CPUUsageRate float64 // CPU使用率 (0-100)

    // 内存指标
    MemoryCapacity  int64
    MemoryUsage     int64
    MemoryUsageRate float64

    // 磁盘指标
    DiskCapacity  int64
    DiskUsage     int64
    DiskUsageRate float64

    // 网络指标（预留）
    NetworkLatency   float64
    NetworkBandwidth float64

    // GPU 指标（预留，通过CRD扩展）
    GPUCount       int
    GPUModels      []string
    GPUUsage       []float64
    GPUMemoryTotal []int64
    GPUMemoryUsed  []int64

    // 健康状态
    Healthy    bool
    Conditions []string

    // 节点标签
    Labels map[string]string

    // 自定义扩展
    CustomMetrics map[string]interface{}
}
```

**辅助方法**：
- `GetAvailableResources()` - 计算可用资源
- `IsUnderPressure()` - 检查资源压力状态
- `MeetsConstraints()` - 检查是否满足约束条件（为未来调度器准备）

#### PodMetrics - Pod 资源使用指标
```go
type PodMetrics struct {
    PodName   string
    Namespace string
    NodeName  string
    Timestamp time.Time

    // 资源使用（实际）
    CPUUsage    int64
    MemoryUsage int64

    // 资源限制
    CPURequest    int64
    CPULimit      int64
    MemoryRequest int64
    MemoryLimit   int64

    // 使用率（相对于limit）
    CPUUsageRate    float64
    MemoryUsageRate float64

    // Container级别指标
    Containers []ContainerMetrics

    // Pod状态
    Phase      string
    Ready      bool
    Restarts   int32
    StartTime  time.Time
}
```

**辅助方法**：
- `GetResourceUtilization()` - 计算资源利用率（相对于 request）
- `IsOverLimit()` - 检查是否接近资源限制

#### NetworkMetrics - 网络指标（预留）
```go
type NetworkMetrics struct {
    SourcePod   string
    TargetPod   string
    Timestamp   time.Time

    Connected bool
    RTT        float64
    PacketLoss float64
    Bandwidth  float64
    TestMethod string
}
```

#### ClusterMetrics - 集群整体指标
```go
type ClusterMetrics struct {
    Timestamp time.Time

    // 集群资源总量
    TotalNodes      int
    HealthyNodes    int
    TotalPods       int
    RunningPods     int

    // 资源汇总
    TotalCPU        int64
    UsedCPU         int64
    CPUUsageRate    float64

    TotalMemory     int64
    UsedMemory      int64
    MemoryUsageRate float64

    TotalGPUs       int
    AvailableGPUs   int

    // 健康状态
    HealthStatus    string   // healthy, warning, critical
    Issues          []string
}
```

#### MetricsSnapshot - 指标快照
用于时间序列存储的完整快照：
```go
type MetricsSnapshot struct {
    Timestamp      time.Time
    NodeMetrics    map[string]*NodeMetrics
    PodMetrics     map[string]*PodMetrics
    NetworkMetrics []*NetworkMetrics
    ClusterMetrics *ClusterMetrics
}
```

---

### 2. 数据源实现

#### NodeMetricsCollector (`internal/metrics/sources/node_metrics.go`)

**功能**：
- 从 K8s Metrics Server 采集节点 CPU/内存使用数据
- 从 Node API 获取节点容量、状态、标签
- 计算使用率、检查健康状态
- 识别资源压力条件（MemoryPressure, DiskPressure 等）

**方法**：
- `CollectNodeMetrics(ctx)` - 采集所有节点指标
- `CollectSingleNodeMetrics(ctx, nodeName)` - 采集单个节点指标

#### PodMetricsCollector (`internal/metrics/sources/pod_metrics.go`)

**功能**：
- 从 K8s Metrics Server 采集 Pod 资源使用数据
- 从 Pod API 获取 Pod 状态、requests/limits
- 支持多 namespace 监控
- Container 级别的指标采集

**方法**：
- `CollectPodMetrics(ctx)` - 采集所有 Pod 指标
- `CollectNamespacePodMetrics(ctx, namespace)` - 采集指定 namespace 的 Pod 指标

---

### 3. 统一管理器 (`internal/metrics/manager.go`)

**MetricsManager** 是指标系统的统一入口：

**功能**：
- 协调多个数据源的采集
- 定期自动采集（可配置间隔）
- 并发采集提高性能
- 缓存最新数据
- 自动计算集群整体指标

**核心方法**：
```go
// 启动定期采集
Start(ctx context.Context) error

// 停止采集
Stop() error

// 手动触发采集
Collect(ctx context.Context) error

// 获取最新快照
GetLatestSnapshot() *MetricsSnapshot

// 获取节点指标
GetNodeMetrics(nodeName string) (*NodeMetrics, error)

// 获取Pod指标
GetPodMetrics(namespace, podName string) (*PodMetrics, error)

// 获取集群指标
GetClusterMetrics() *ClusterMetrics
```

**特性**：
- 线程安全的缓存机制
- 并发采集各类指标
- 自动计算集群健康状态
- 优雅启动和停止

---

### 4. 配置系统

#### 配置结构 (`internal/config/config.go`)
```go
type MetricsConfig struct {
    Enabled         bool     // 是否启用指标采集
    CollectInterval int      // 采集间隔（秒）
    Namespaces      []string // 要监控的命名空间
    EnableNode      bool     // 启用节点指标
    EnablePod       bool     // 启用Pod指标
    EnableNetwork   bool     // 启用网络指标
    EnableCustom    bool     // 启用自定义CRD指标
    CacheRetention  int      // 缓存保留时间（秒）
}
```

#### 配置文件 (`configs/config.yaml`)
```yaml
metrics:
  enabled: true
  collect_interval: 30
  namespaces:
    - default
    - kube-system
  enable_node: true
  enable_pod: true
  enable_network: false
  enable_custom: false
  cache_retention: 300
```

---

## 目录结构

```
internal/
├── metrics/                      # 指标采集模块
│   ├── models.go                 # 数据模型定义
│   ├── collector.go              # 采集器接口定义
│   ├── manager.go                # 统一管理器
│   └── sources/                  # 数据源实现
│       ├── node_metrics.go       # 节点指标采集器
│       └── pod_metrics.go        # Pod指标采集器
│
pkg/models/
└── scheduler.go                  # 调度器相关模型（预留）
```

---

## 使用示例

### 基础用法

```go
import (
    "context"
    "time"

    "github.com/yourusername/k8s-llm-monitor/internal/config"
    "github.com/yourusername/k8s-llm-monitor/internal/metrics"
)

// 1. 加载配置
cfg, _ := config.Load("./configs/config.yaml")

// 2. 创建 K8s REST 配置
restConfig, _ := clientcmd.BuildConfigFromFlags("", cfg.K8s.Kubeconfig)

// 3. 创建指标管理器
managerConfig := metrics.ManagerConfig{
    Namespaces:      cfg.Metrics.Namespaces,
    CollectInterval: time.Duration(cfg.Metrics.CollectInterval) * time.Second,
    EnableNode:      cfg.Metrics.EnableNode,
    EnablePod:       cfg.Metrics.EnablePod,
    EnableNetwork:   cfg.Metrics.EnableNetwork,
    EnableCustom:    cfg.Metrics.EnableCustom,
}

manager, _ := metrics.NewManager(restConfig, managerConfig)

// 4. 启动定期采集
ctx := context.Background()
go manager.Start(ctx)

// 5. 获取指标数据
snapshot := manager.GetLatestSnapshot()
nodeMetrics, _ := manager.GetNodeMetrics("node-1")
podMetrics, _ := manager.GetPodMetrics("default", "my-pod")
clusterMetrics := manager.GetClusterMetrics()
```

---

## 为 LLM 提供的数据能力

有了这些指标，LLM 可以回答以下类型的问题：

### 节点相关
- ✅ "node-3 的 CPU 使用率是多少？"
- ✅ "哪些节点负载最高？"
- ✅ "node-2 是否有资源压力？"
- ✅ "集群中有多少个健康节点？"

### Pod 相关
- ✅ "哪些 Pod 的内存使用率超过了 limit 的 90%？"
- ✅ "default namespace 中有多少个 Pod 正在运行？"
- ✅ "为什么 my-pod 一直重启？"（结合重启次数和资源使用）
- ✅ "Pod xyz 分配在哪个节点上？资源使用情况如何？"

### 集群相关
- ✅ "集群整体的 CPU 使用率是多少？"
- ✅ "集群健康状态如何？有哪些问题？"
- ✅ "集群还有多少可用资源？"
- ✅ "当前有多少 Pod 不在 Running 状态？"

---

## 扩展能力（已预留接口）

### 1. 网络指标扩展
- 接口已定义：`NetworkMetricsSource`
- 可集成现有的 RTT 测试功能
- 支持 Pod 间通信质量分析

### 2. 自定义 CRD 指标
- 接口已定义：`CustomMetricsSource`
- 数据模型中的 `CustomMetrics` 字段
- 可通过 CRD 扩展 GPU、磁盘 I/O 等指标

### 3. 时间序列存储
- `MetricsSnapshot` 结构支持时间序列
- 可集成到现有的 `internal/storage` 模块
- 支持历史趋势分析

---

## 依赖

已添加到 `go.mod`：
```
k8s.io/metrics v0.34.1
```

该依赖提供了访问 K8s Metrics Server 的客户端。

---

## 下一步计划

### 短期（核心功能完善）
1. ✅ 节点和 Pod 指标采集（已完成）
2. ⏳ 网络指标集成（复用现有 RTT 测试）
3. ⏳ 设计 CRD 用于自定义指标扩展
4. ⏳ 集成到现有的 HTTP API

### 中期（增强观测能力）
5. ⏳ 时间序列数据存储
6. ⏳ 历史趋势分析
7. ⏳ 指标告警机制
8. ⏳ LLM 上下文集成

### 长期（高级功能）
9. ⏳ Agent 部署用于深度硬件监控
10. ⏳ 基于指标的智能调度器
11. ⏳ 预测性分析和容量规划

---

## 与现有系统的集成

### 利用现有模块
- ✅ **K8s Client** (`internal/k8s/client.go`) - 复用 Kubernetes 连接
- ✅ **配置系统** (`internal/config/`) - 扩展配置支持
- ⏳ **RTT Tester** (`internal/k8s/rtt_tester.go`) - 可用于网络指标
- ⏳ **Storage** (`internal/storage/`) - 可用于时间序列存储
- ⏳ **LLM** (`internal/llm/`) - 为智能分析提供数据

### API 集成建议
在 `cmd/server/main.go` 添加以下端点：
- `GET /api/v1/metrics/cluster` - 集群整体指标
- `GET /api/v1/metrics/nodes` - 所有节点指标
- `GET /api/v1/metrics/nodes/{name}` - 单个节点指标
- `GET /api/v1/metrics/pods` - 所有 Pod 指标
- `GET /api/v1/metrics/pods/{namespace}/{name}` - 单个 Pod 指标
- `GET /api/v1/metrics/snapshot` - 完整快照

---

## 总结

✅ **已完成**：
- 完整的数据模型设计
- Node 和 Pod 指标采集器实现
- 统一的指标管理器
- 配置系统集成
- 集群整体指标汇总

📋 **架构特点**：
- 模块化、可扩展
- 独立于上层应用
- 线程安全、高性能
- 支持多数据源

🎯 **价值**：
为 LLM 智能观测平台提供了完整的数据基础，支持自然语言查询集群状态、资源使用、健康状况等。
