# UAV数据采集方案

## 概述

本文档介绍如何使用K8s LLM Monitor系统收集UAV（无人机）数据。

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                   Metrics Manager                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Node    │  │   Pod    │  │ Network  │  │   UAV    │   │
│  │Collector │  │Collector │  │Collector │  │Collector │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│       ↓             ↓             ↓             ↓          │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Unified Cache (Snapshot)                 │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                           ↓
                    HTTP API Endpoints
                           ↓
              ┌────────────┴────────────┐
              ↓                         ↓
    GET /api/v1/metrics/uav    GET /api/v1/metrics/uav/{node}
```

## 实现方案

### 方案1：集成到Metrics系统（已实现）⭐

**特点**：
- 复用现有的采集架构
- 统一的定期采集（默认30秒）
- 自动缓存和更新
- 与其他指标（Node/Pod/Network）一起管理

**优点**：
- ✅ 架构统一，易于维护
- ✅ 自动定期采集，无需额外代码
- ✅ 线程安全的缓存机制
- ✅ 与其他监控指标关联分析
- ✅ 支持并发采集多个UAV

**缺点**：
- ❌ 采集频率固定（可配置，但所有指标统一）
- ❌ 采集失败只记录日志，不影响其他指标

### 实现细节

#### 1. UAV Collector (`internal/metrics/sources/uav_metrics.go`)

```go
// UAVMetricsCollector 负责采集所有UAV Agent的数据
type UAVMetricsCollector struct {
    kubeClient  *kubernetes.Clientset  // 用于查找UAV Pod
    httpClient  *http.Client            // 用于HTTP请求
    namespace   string                  // UAV Agent所在namespace
    uavPodLabel string                  // Pod label selector
}

// 主要方法：
// - CollectUAVMetrics()         采集所有UAV
// - CollectSingleUAVMetrics()   采集单个UAV
// - GetHealthyUAVCount()        获取健康UAV数量
// - GetLowBatteryUAVs()         获取低电量UAV列表
```

**采集流程**：
1. 通过K8s API查找所有`app=uav-agent`的Running Pod
2. 并发向每个Pod的`:9090/api/v1/state`发送HTTP GET请求
3. 解析JSON响应，提取UAV状态
4. 汇总所有UAV数据，按节点名索引

#### 2. 集成到Manager (`internal/metrics/manager.go`)

```go
// Manager新增字段
type Manager struct {
    uavSource   UAVMetricsSource           // UAV数据源
    uavSnapshot map[string]interface{}     // UAV快照缓存
}

// 配置选项
type ManagerConfig struct {
    EnableUAV bool  // 是否启用UAV采集
}
```

#### 3. API接口 (`cmd/server/main.go`)

新增两个HTTP端点：

```bash
# 获取所有UAV状态
GET /api/v1/metrics/uav

# 获取指定节点的UAV状态
GET /api/v1/metrics/uav/{node_name}
```

## 使用方法

### 1. 启用UAV数据采集

确保在服务器启动时启用UAV采集（已默认启用）：

```go
managerConfig := metrics.ManagerConfig{
    EnableUAV: true,  // 启用UAV采集
    // ... 其他配置
}
```

### 2. 部署UAV Agent

```bash
# 部署UAV Agent DaemonSet
kubectl apply -f deployments/uav-agent-daemonset.yaml

# 验证部署
kubectl get pods -l app=uav-agent
```

### 3. 查询UAV数据

#### 通过HTTP API查询

```bash
# 获取所有UAV状态
curl http://localhost:8081/api/v1/metrics/uav | jq .

# 响应示例：
{
  "status": "success",
  "count": 3,
  "timestamp": "2025-10-11T10:00:00Z",
  "data": {
    "k3d-k8s-llm-monitor-server-0": {
      "uav_id": "UAV-k3d-k8s-llm-monitor-server-0",
      "node_name": "k3d-k8s-llm-monitor-server-0",
      "gps": {
        "latitude": 39.9042,
        "longitude": 116.4074,
        "altitude": 50.0,
        "satellite_count": 12
      },
      "battery": {
        "voltage": 22.2,
        "remaining_percent": 85.0
      },
      "flight": {
        "mode": "AUTO",
        "armed": true,
        "ground_speed": 5.0
      },
      "health": {
        "system_status": "OK"
      }
    },
    "k3d-k8s-llm-monitor-agent-0": { ... },
    "k3d-k8s-llm-monitor-agent-1": { ... }
  }
}

# 获取单个节点的UAV
curl http://localhost:8081/api/v1/metrics/uav/k3d-k8s-llm-monitor-server-0 | jq .
```

#### 编程方式查询

```go
// 获取所有UAV
uavMetrics := metricsManager.GetUAVMetrics()
for nodeName, state := range uavMetrics {
    fmt.Printf("Node: %s, Battery: %.1f%%\n",
        nodeName, state.Battery.RemainingPercent)
}

// 获取单个UAV
uavState, exists := metricsManager.GetSingleUAVMetrics("node-1")
if exists {
    fmt.Printf("Battery: %.1f%%\n", uavState.Battery.RemainingPercent)
}
```

### 4. 监控特定指标

#### 监控低电量UAV

```bash
# 使用jq过滤低电量UAV（<20%）
curl -s http://localhost:8081/api/v1/metrics/uav | \
  jq '.data | to_entries[] | select(.value.battery.remaining_percent < 20) | {node: .key, battery: .value.battery.remaining_percent}'
```

#### 监控GPS信号质量

```bash
# 监控卫星数量<10的UAV
curl -s http://localhost:8081/api/v1/metrics/uav | \
  jq '.data | to_entries[] | select(.value.gps.satellite_count < 10) | {node: .key, satellites: .value.gps.satellite_count}'
```

#### 监控系统健康状态

```bash
# 查找非OK状态的UAV
curl -s http://localhost:8081/api/v1/metrics/uav | \
  jq '.data | to_entries[] | select(.value.health.system_status != "OK") | {node: .key, status: .value.health.system_status}'
```

## 采集配置

### 采集频率

在`configs/config.yaml`中配置：

```yaml
metrics:
  enabled: true
  collect_interval: 30  # 秒，所有指标统一采集间隔
```

### UAV采集器配置

在代码中配置（可扩展到配置文件）：

```go
uavConfig := sources.UAVCollectorConfig{
    Namespace: "default",        // UAV Agent所在namespace
    UAVLabel:  "app=uav-agent",  // Pod标签
    Timeout:   5 * time.Second,  // HTTP请求超时
}
```

## 数据结构

### UAVState完整结构

```json
{
  "uav_id": "UAV-node-1",
  "node_name": "node-1",
  "system_time": "2025-10-11T10:00:00Z",
  "gps": {
    "latitude": 39.9042,
    "longitude": 116.4074,
    "altitude": 50.0,
    "relative_altitude": 50.0,
    "hdop": 1.0,
    "satellite_count": 12,
    "fix_type": 3,
    "ground_speed": 5.0,
    "course_over_ground": 90.0
  },
  "attitude": {
    "roll": 5.0,
    "pitch": 3.0,
    "yaw": 90.0,
    "roll_rate": 0.5,
    "pitch_rate": 0.3,
    "yaw_rate": 1.0
  },
  "flight": {
    "mode": "AUTO",
    "armed": true,
    "airspeed": 5.5,
    "ground_speed": 5.0,
    "vertical_speed": 0.5,
    "throttle_percent": 50.0
  },
  "battery": {
    "voltage": 22.2,
    "current": 10.5,
    "remaining_percent": 85.0,
    "remaining_capacity": 4250.0,
    "total_capacity": 5000.0,
    "temperature": 28.0,
    "cell_count": 6,
    "time_remaining": 1800
  },
  "mission": {
    "current_waypoint": 0,
    "total_waypoints": 0,
    "mission_state": "IDLE",
    "distance_to_wp": 0.0,
    "eta_to_wp": 0
  },
  "health": {
    "system_status": "OK",
    "sensors_health": {
      "gps": true,
      "compass": true,
      "accelerometer": true,
      "gyroscope": true,
      "barometer": true,
      "battery": true
    },
    "error_count": 0,
    "warning_count": 0,
    "messages": [],
    "last_heartbeat": "2025-10-11T10:00:00Z"
  }
}
```

## 性能考虑

### 采集性能

- **并发采集**：所有UAV并发采集，总时间≈单次HTTP请求时间
- **超时控制**：默认5秒超时，避免长时间等待
- **失败处理**：单个UAV失败不影响其他UAV采集
- **缓存机制**：采集结果缓存，读取无需访问K8s API

### 资源使用

假设10个节点，30秒采集间隔：
- HTTP请求：10个/30秒 = 0.33 req/s
- 内存使用：~10KB/UAV × 10 = 100KB
- 网络流量：~5KB/请求 × 10 = 50KB/30秒

## 扩展建议

### 1. 时序数据存储

将UAV数据写入时序数据库：

```go
// 使用Prometheus
uavBatteryGauge.WithLabelValues(nodeName).Set(state.Battery.RemainingPercent)

// 或使用InfluxDB
point := influxdb2.NewPoint("uav_battery",
    map[string]string{"node": nodeName},
    map[string]interface{}{"percent": state.Battery.RemainingPercent},
    time.Now())
```

### 2. 告警规则

```yaml
# Prometheus告警规则
groups:
- name: uav_alerts
  rules:
  - alert: UAVLowBattery
    expr: uav_battery_percent < 20
    annotations:
      summary: "UAV {{ $labels.node }} battery low: {{ $value }}%"

  - alert: UAVGPSLost
    expr: uav_gps_satellites < 6
    annotations:
      summary: "UAV {{ $labels.node }} GPS signal weak"
```

### 3. 历史数据查询

添加时间范围查询：

```go
// API: GET /api/v1/metrics/uav/history?start=xxx&end=xxx
func GetUAVHistory(start, end time.Time) []UAVState {
    // 从时序数据库查询
}
```

## 故障排查

### 采集失败

```bash
# 查看manager日志
tail -f server.log | grep "UAV metrics"

# 常见问题：
# 1. UAV Pod未Running
kubectl get pods -l app=uav-agent

# 2. 网络不通
kubectl exec -it <monitor-pod> -- curl http://<uav-pod-ip>:9090/health

# 3. UAV Agent未启动
kubectl logs -l app=uav-agent
```

### 数据缺失

检查采集是否启用：

```bash
# 查看manager配置
curl http://localhost:8081/api/v1/metrics/snapshot | jq '.data | keys'
# 应该看到uav相关数据
```

## 总结

通过集成到Metrics Manager系统，UAV数据采集实现了：

1. ✅ **自动化**：定期自动采集，无需手动触发
2. ✅ **统一管理**：与其他监控指标统一架构
3. ✅ **高性能**：并发采集，缓存机制
4. ✅ **易用性**：简单的HTTP API
5. ✅ **扩展性**：易于添加新的数据源和指标

这种方案特别适合：
- 需要持续监控UAV状态
- 与K8s集群监控数据关联分析
- 构建统一的监控面板
- 实现自动化告警和响应
