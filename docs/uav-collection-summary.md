# UAV数据采集方案总结

## 问题：如何收集UAV数据？

你有一个无人机模拟系统，每个K8s节点上运行一个UAV Agent，现在需要收集这些数据。

## 解决方案对比

### 方案1：集成到Metrics系统（✅ 已实现，推荐）

**架构**：
```
Metrics Manager
  ├── NodeCollector
  ├── PodCollector
  ├── NetworkCollector
  └── UAVCollector ← 新增
```

**优点**：
- ✅ 统一架构，易于维护
- ✅ 自动定期采集（默认30秒）
- ✅ 内置缓存和并发处理
- ✅ 与其他监控数据关联分析
- ✅ 线程安全

**缺点**：
- ❌ 采集频率固定（所有指标统一）
- ❌ 与Metrics系统耦合

**适用场景**：
- 需要持续监控UAV状态
- 与集群监控数据一起分析
- 构建统一监控面板

---

### 方案2：独立采集服务（未实现）

**架构**：
```
UAV Monitor Service (独立进程)
  └── 定期轮询所有UAV Agent
```

**优点**：
- ✅ 解耦，不依赖Metrics系统
- ✅ 采集频率独立配置
- ✅ 可单独扩展和优化

**缺点**：
- ❌ 需要额外的进程管理
- ❌ 需要自己实现缓存、并发
- ❌ 与其他监控数据割裂

**适用场景**：
- UAV监控与集群监控完全独立
- 需要不同的采集策略
- 专门的UAV管理系统

---

### 方案3：Prometheus Exporter（未实现）

**架构**：
```
UAV Exporter (每个节点)
  └── 暴露Prometheus metrics
      └── Prometheus定期scrape
```

**优点**：
- ✅ 标准的监控方案
- ✅ 强大的查询和告警能力
- ✅ 长期历史数据存储
- ✅ 丰富的生态系统（Grafana等）

**缺点**：
- ❌ 需要运行Prometheus
- ❌ 需要编写Exporter
- ❌ 学习曲线

**适用场景**：
- 已有Prometheus环境
- 需要长期存储和分析
- 需要复杂的告警规则

---

### 方案4：直接查询（最简单，但不推荐）

**架构**：
```
你的代码 → 直接HTTP调用 → UAV Agent
```

**优点**：
- ✅ 最简单直接
- ✅ 无需额外组件

**缺点**：
- ❌ 每次都要查询K8s API
- ❌ 每次都要发起HTTP请求
- ❌ 无缓存，性能差
- ❌ 需要自己处理并发、错误

**代码示例**：
```go
// 获取所有UAV Pod
pods, _ := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
    LabelSelector: "app=uav-agent",
})

// 逐个查询
for _, pod := range pods.Items {
    resp, _ := http.Get(fmt.Sprintf("http://%s:9090/api/v1/state", pod.Status.PodIP))
    // 处理响应...
}
```

**适用场景**：
- 偶尔手动查询
- 临时脚本
- 调试目的

---

## 推荐方案：方案1（已实现）

### 实现文件

```
internal/metrics/sources/uav_metrics.go  # UAV采集器
internal/metrics/manager.go              # Manager集成
internal/metrics/collector.go            # 接口定义
cmd/server/main.go                       # API端点
```

### 使用方式

#### 1. 部署UAV Agent
```bash
kubectl apply -f deployments/uav-agent-daemonset.yaml
```

#### 2. 启动监控服务（自动启用UAV采集）
```bash
go run cmd/server/main.go
```

#### 3. 查询UAV数据

**通过HTTP API**：
```bash
# 所有UAV
curl http://localhost:8081/api/v1/metrics/uav | jq .

# 单个UAV
curl http://localhost:8081/api/v1/metrics/uav/node-name | jq .
```

**通过代码**：
```go
// 获取所有UAV
uavMetrics := manager.GetUAVMetrics()

// 获取单个UAV
uavState, exists := manager.GetSingleUAVMetrics("node-1")
```

### API端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/metrics/uav` | GET | 获取所有UAV状态 |
| `/api/v1/metrics/uav/{node}` | GET | 获取指定节点的UAV |

### 配置

在`configs/config.yaml`中：
```yaml
metrics:
  enabled: true
  collect_interval: 30  # 秒
```

### 测试

```bash
# 运行测试脚本
./scripts/test_uav_collection.sh
```

---

## 数据流程

```
1. Manager启动
   ↓
2. 创建UAVCollector
   ↓
3. 定期采集（30秒）
   ↓
4. UAVCollector查询K8s API获取UAV Pod列表
   ↓
5. 并发向所有UAV Agent发送HTTP请求
   ↓
6. 收集并缓存数据
   ↓
7. 通过API提供给用户
```

---

## 性能指标

假设：
- 10个节点
- 30秒采集间隔
- 每个UAV响应5KB数据

**资源使用**：
- HTTP请求：10次/30秒 = 0.33 req/s
- 内存：~100KB（缓存）
- 网络：50KB/30秒 = 1.67KB/s

**响应时间**：
- 并发采集：≈单次HTTP耗时（~100-500ms）
- API查询：<10ms（从缓存读取）

---

## 扩展方向

### 短期（1周内）
1. ✅ 基本采集功能（已完成）
2. 📝 时序数据存储（Prometheus/InfluxDB）
3. 📝 告警规则（低电量、GPS失效）

### 中期（1个月内）
1. 📝 历史数据查询API
2. 📝 UAV轨迹记录和回放
3. 📝 性能优化（批量查询）

### 长期（3个月内）
1. 📝 机器学习预测（电池寿命、故障预测）
2. 📝 3D可视化界面
3. 📝 蜂群协同分析

---

## 常见问题

### Q1: 采集频率能否调整？
A: 可以，在`config.yaml`中修改`collect_interval`，但会影响所有指标。

### Q2: 能否只采集部分UAV？
A: 可以，修改`UAVCollectorConfig.UAVLabel`来过滤。

### Q3: 如何处理采集失败？
A: 单个UAV失败只记录日志，不影响其他UAV和其他指标采集。

### Q4: 数据保留多久？
A: 当前只保留最新一次采集结果。如需历史数据，建议集成Prometheus。

### Q5: 能否实时监控？
A: 采集间隔最小可设置为1秒，但建议>=10秒以降低系统负载。

---

## 总结

| 特性 | 方案1（推荐） | 方案2 | 方案3 | 方案4 |
|------|--------------|-------|-------|-------|
| 实现难度 | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐ |
| 维护成本 | 低 | 中 | 低（用Prometheus） | 高 |
| 性能 | 高 | 中 | 高 | 低 |
| 扩展性 | 好 | 好 | 优秀 | 差 |
| 学习曲线 | 平缓 | 平缓 | 陡峭 | 平缓 |

**选择建议**：
- 🎯 **大多数情况**：使用方案1（已实现）
- 🔧 **需要独立管理UAV**：考虑方案2
- 📊 **已有Prometheus环境**：考虑方案3
- 🚀 **快速原型/临时查询**：使用方案4

---

## 快速开始

```bash
# 1. 部署UAV Agent
kubectl apply -f deployments/uav-agent-daemonset.yaml

# 2. 启动监控服务
go run cmd/server/main.go

# 3. 测试
./scripts/test_uav_collection.sh

# 4. 查询数据
curl http://localhost:8081/api/v1/metrics/uav | jq .
```

完整文档：
- 详细实现：`docs/uav-data-collection.md`
- 使用指南：`docs/uav-simulator-guide.md`
