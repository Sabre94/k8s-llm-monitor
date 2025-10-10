# Web 界面查看集群指标

## 已完成的功能

✅ **后端 API**
- 集群整体指标: `GET /api/v1/metrics/cluster`
- 所有节点指标: `GET /api/v1/metrics/nodes`
- 单个节点指标: `GET /api/v1/metrics/nodes/{nodeName}`
- 所有Pod指标: `GET /api/v1/metrics/pods`
- 完整快照: `GET /api/v1/metrics/snapshot`

✅ **Web 界面**
- 实时显示集群状态
- 节点资源使用情况（CPU、内存、磁盘）
- Pod 资源使用情况
- 自动每30秒刷新数据

---

## 如何使用

### 1. 确保 Metrics Server 已部署

K8s 集群需要部署 Metrics Server 才能获取实时的 CPU/内存使用数据。

检查是否已部署：
```bash
kubectl get deploy -n kube-system metrics-server
```

如果没有部署，可以安装：
```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

对于本地开发环境（如 minikube），可能需要额外配置：
```bash
# minikube
minikube addons enable metrics-server

# kind
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

---

### 2. 启动服务器

```bash
# 构建
go build -o bin/server ./cmd/server

# 运行
./bin/server
```

服务器会：
1. 连接到 K8s 集群（使用 `~/.kube/config`）
2. 启动指标采集（默认每30秒采集一次）
3. 启动 HTTP 服务器（默认端口 8080）

启动日志示例：
```
2025-10-10 15:30:00 Starting K8s LLM Monitor...
2025-10-10 15:30:00 Server: 0.0.0.0:8080
2025-10-10 15:30:00 Successfully connected to Kubernetes cluster
2025-10-10 15:30:00 Metrics manager created successfully
2025-10-10 15:30:00 Metrics collection started (interval: 30 seconds)
2025-10-10 15:30:00 HTTP Server starting on 0.0.0.0:8080
```

---

### 3. 访问 Web 界面

打开浏览器，访问：

**http://localhost:8080/metrics.html**

你将看到：

#### 集群整体指标卡片
- 集群健康状态
- 健康节点数/总节点数
- 运行中的Pod数/总Pod数
- CPU 使用率（带进度条）
- 内存使用率（带进度条）

#### 节点列表表格
显示每个节点的：
- 节点名称
- 健康状态
- CPU 使用率
- 内存使用率
- 磁盘使用率

每个指标都有彩色进度条：
- 🟢 绿色：< 60%
- 🟡 黄色：60-80%
- 🔴 红色：> 80%

#### Pod 列表表格
显示前20个Pod的：
- Pod名称
- 命名空间
- 所在节点
- 运行状态
- CPU 使用量
- 内存使用量
- 重启次数

---

### 4. API 示例

#### 获取集群整体指标
```bash
curl http://localhost:8080/api/v1/metrics/cluster | jq
```

响应示例：
```json
{
  "status": "success",
  "data": {
    "timestamp": "2025-10-10T15:30:00Z",
    "total_nodes": 3,
    "healthy_nodes": 3,
    "total_pods": 25,
    "running_pods": 24,
    "total_cpu": 12000,
    "used_cpu": 3456,
    "cpu_usage_rate": 28.8,
    "total_memory": 17179869184,
    "used_memory": 8589934592,
    "memory_usage_rate": 50.0,
    "health_status": "healthy",
    "issues": []
  }
}
```

#### 获取所有节点指标
```bash
curl http://localhost:8080/api/v1/metrics/nodes | jq
```

#### 获取单个节点指标
```bash
curl http://localhost:8080/api/v1/metrics/nodes/node-1 | jq
```

#### 获取所有Pod指标
```bash
curl http://localhost:8080/api/v1/metrics/pods | jq
```

---

## 配置

编辑 `configs/config.yaml` 中的 metrics 部分：

```yaml
metrics:
  enabled: true              # 是否启用指标采集
  collect_interval: 30       # 采集间隔（秒）
  namespaces:                # 监控的命名空间
    - default
    - kube-system
  enable_node: true          # 启用节点指标
  enable_pod: true           # 启用Pod指标
  enable_network: false      # 启用网络指标（未实现）
  enable_custom: false       # 启用自定义CRD指标（未实现）
  cache_retention: 300       # 缓存保留时间（秒）
```

---

## 故障排查

### 问题1：无法连接到 K8s 集群

**症状**：服务器启动时显示警告
```
Warning: Failed to connect to k8s: ...
Running in development mode without K8s connection
```

**解决方法**：
1. 检查 kubeconfig 文件是否存在：`ls ~/.kube/config`
2. 检查集群连接：`kubectl cluster-info`
3. 确认配置文件中的 kubeconfig 路径正确

### 问题2：Metrics Server 不可用

**症状**：节点和Pod的使用量显示为 0

**解决方法**：
1. 检查 Metrics Server 是否运行：
```bash
kubectl get deploy -n kube-system metrics-server
kubectl get pods -n kube-system | grep metrics-server
```

2. 查看 Metrics Server 日志：
```bash
kubectl logs -n kube-system deploy/metrics-server
```

3. 测试 Metrics API：
```bash
kubectl top nodes
kubectl top pods
```

### 问题3：Web 界面显示错误

**症状**：浏览器显示 "加载失败" 错误

**解决方法**：
1. 检查服务器是否运行
2. 打开浏览器开发者工具，查看 Console 和 Network 标签页
3. 确认 API 是否可访问：`curl http://localhost:8080/health`

---

## 下一步

### 已计划的功能
- ⏳ 网络指标采集（集成现有RTT测试）
- ⏳ 自定义CRD指标扩展（GPU等）
- ⏳ 历史数据和趋势图表
- ⏳ 告警和通知
- ⏳ LLM智能分析集成

### 你可以做的
1. 查看实时的集群状态
2. 监控节点资源使用情况
3. 识别资源压力大的节点
4. 找出资源使用异常的Pod
5. 通过API集成到你的其他工具

---

## 截图预览

Web界面包含：
- 顶部紫色渐变header
- 5个指标卡片（集群状态、节点、Pod、CPU、内存）
- 节点列表表格
- Pod列表表格
- 所有数据每30秒自动刷新
- 响应式设计，支持各种屏幕尺寸

界面使用现代化的设计风格，带有：
- 卡片阴影和悬停效果
- 彩色进度条
- 状态徽章
- 清晰的排版和间距
