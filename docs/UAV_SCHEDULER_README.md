# UAV智能调度系统 - 部署指南

## 🚀 项目概述

基于Kubernetes的UAV（无人机）智能调度系统，采用NSGA-II多目标优化算法，为UAV任务提供最优节点集合选择和调度服务。

### 核心特性

- 🎯 **智能调度**: 基于NSGA-II多目标优化算法
- 🔌 **算法可插拔**: 支持算法热插拔和扩展
- 📊 **CRD数据集成**: 从UAVMetric自定义资源获取实时数据
- 🚁 **节点集合绑定**: 直接绑定最优节点集合到任务Pod
- ⚡ **高性能**: 内置缓存机制，支持并发调度
- 🐛 **可观测**: 完整的日志和监控支持

## 📋 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   任务Pod       │───▶│   UAV调度器     │───▶│  节点集合绑定    │
│  (侦察任务)     │    │                 │    │  (3个最优节点)   │
└─────────────────┘    │  ┌─────────────┐│    └─────────────────┘
                       │  │ NSGA-II算法  ││
┌─────────────────┐    │  │             ││    ┌─────────────────┐
│  UAVMetric CRD  │───▶│  └─────────────┘│    │   Kubernetes    │
│  (UAV实时数据)   │    │  ┌─────────────┐│    │   集群节点      │
└─────────────────┘    │  │ CRD数据提供者││    │  (uav-node-*)   │
                       │  └─────────────┘│    └─────────────────┘
                       └─────────────────┘
```

## 🎯 调度流程

### 传统调度 vs UAV智能调度

```
❌ 传统调度:
Pod → 调度器 → 单个节点 → 绑定

✅ UAV智能调度:
任务Pod → NSGA-II优化 → 节点集合 → 直接绑定
```

### 核心工作流程

1. **任务接收**: 接收UAV任务调度请求
2. **数据获取**: 从CRD获取所有可用UAV节点实时数据
3. **多目标优化**: NSGA-II算法基于以下目标进行优化：
   - 最大化电池电量
   - 最小化网络延迟
   - 平衡节点利用率
   - 控制节点数量
4. **节点选择**: 自动计算最优节点数量和具体节点选择
5. **集合绑定**: 将整个节点集合绑定到任务Pod

## 🚀 快速开始

### 前置要求

- Kubernetes集群 (v1.20+)
- Go 1.19+
- Python 3.8+ (NSGA-II算法)
- kubectl配置正确
- 集群管理员权限

### 1. 克隆项目

```bash
git clone <repository-url>
cd k8s-llm-monitor
```

### 2. 编译调度器

```bash
cd cmd/scheduler
go mod tidy
go build -o uav-scheduler
```

### 3. 配置系统

复制并编辑配置文件：

```bash
cp cmd/scheduler/config.yaml cmd/scheduler/my-config.yaml
```

编辑配置文件：

```yaml
# 基础配置
name: "uav-scheduler"
listen_addr: ":8080"

# Kubernetes配置
k8s:
  namespace: "default"
  in_cluster: false
  kubeconfig: "/Users/xiabin/.kube/config"  # 修改为你的kubeconfig路径

# 算法配置
algorithms:
  nsga2:
    population_size: 50      # 种群大小
    max_generations: 20      # 最大迭代次数
    crossover_prob: 0.9      # 交叉概率
    grid_density: 40         # 网格密度
    python_path: "python3"   # Python解释器路径
    script_path: "RUN2.py"   # NSGA-II脚本路径

  demo:
    enabled: true             # 启用演示模式
    delay: "5s"               # 演示启动延迟

# 数据提供者配置
data_providers:
  crd:
    enabled: true
    watch_interval: "30s"    # 数据监听间隔
    cache_ttl: "2m"          # 缓存TTL

# 日志配置
logging:
  level: "info"              # 日志级别: debug, info, warn, error
  format: "json"             # 日志格式: json, text
  output: "stdout"           # 输出: stdout, stderr, file
```

### 4. 运行测试（干运行模式）

```bash
./uav-scheduler -config=my-config.yaml -dry-run=true -log-level=info
```

预期输出：
```
{"level":"info","msg":"Starting UAV Scheduler"}
{"level":"info","msg":"NSGA-II algorithm registered"}
{"level":"info","msg":"Demo mode is enabled"}
{"level":"info","msg":"Demo: Single pod scheduling"}
{"level":"info","msg":"NSGA-II optimization completed"}
{"level":"info","msg":"Mission scheduling completed successfully"}
```

### 5. 生产部署

```bash
# 实际运行模式（需要在Kubernetes集群中）
./uav-scheduler -config=my-config.yaml -log-level=info
```

## 📦 Kubernetes部署指南

### 第一步：创建UAVMetric CRD

创建CRD定义文件 `deploy/uavmetric-crd.yaml`：

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: uavmetrics.monitor.k8s-llm-monitor.com
spec:
  group: monitor.k8s-llm-monitor.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              nodeId:
                type: string
                description: "UAV节点唯一标识"
              battery:
                type: number
                minimum: 0
                maximum: 100
                description: "电池电量百分比"
              latency:
                type: number
                minimum: 0
                description: "网络延迟(ms)"
              utilization:
                type: number
                minimum: 0
                maximum: 100
                description: "CPU利用率百分比"
              gps:
                type: array
                items:
                  type: number
                minItems: 2
                maxItems: 2
                description: "GPS坐标 [纬度, 经度]"
              radius:
                type: number
                minimum: 0
                description: "UAV覆盖半径(米)"
              status:
                type: string
                enum: ["ready", "busy", "maintenance", "offline"]
                description: "节点状态"
  scope: Namespaced
  names:
    plural: uavmetrics
    singular: uavmetric
    kind: UAVMetric
```

部署CRD：
```bash
kubectl apply -f deploy/uavmetric-crd.yaml
```

### 第二步：创建示例UAV节点数据

创建 `deploy/uav-nodes.yaml`：

```yaml
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-1
  namespace: default
  labels:
    type: uav
    zone: zone-a
    hardware: standard
spec:
  nodeId: "uav-node-1"
  battery: 85.5
  latency: 45.2
  utilization: 35.8
  gps: [34.043392, -118.266096]
  radius: 480.9
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-2
  namespace: default
  labels:
    type: uav
    zone: zone-b
    hardware: high-performance
spec:
  nodeId: "uav-node-2"
  battery: 72.3
  latency: 78.5
  utilization: 42.1
  gps: [34.044353, -118.253013]
  radius: 399.4
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-3
  namespace: default
  labels:
    type: uav
    zone: zone-a
    hardware: standard
spec:
  nodeId: "uav-node-3"
  battery: 91.2
  latency: 32.8
  utilization: 28.5
  gps: [34.035058, -118.247302]
  radius: 305.6
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-4
  namespace: default
  labels:
    type: uav
    zone: zone-c
    hardware: high-performance
spec:
  nodeId: "uav-node-4"
  battery: 68.9
  latency: 95.3
  utilization: 55.7
  gps: [34.030712, -118.272819]
  radius: 489.6
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-5
  namespace: default
  labels:
    type: uav
    zone: zone-b
    hardware: standard
spec:
  nodeId: "uav-node-5"
  battery: 55.4
  latency: 120.5
  utilization: 68.2
  gps: [34.050073, -118.273802]
  radius: 447.0
  status: "ready"
```

部署UAV节点：
```bash
kubectl apply -f deploy/uav-nodes.yaml
```

### 第三步：创建配置ConfigMap

创建 `deploy/configmap.yaml`：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: uav-scheduler-config
  namespace: default
data:
  config.yaml: |
    # UAV Scheduler Configuration

    # 基础配置
    name: "uav-scheduler"
    listen_addr: ":8080"

    # Kubernetes配置
    k8s:
      namespace: "default"
      in_cluster: true
      kubeconfig: ""

    # 算法配置
    algorithms:
      nsga2:
        population_size: 50
        max_generations: 20
        crossover_prob: 0.9
        grid_density: 40
        python_path: "python3"
        script_path: "RUN2.py"
        temp_dir: "/tmp"

      demo:
        enabled: false  # 生产环���关闭demo

    # 数据提供者配置
    data_providers:
      crd:
        enabled: true
        watch_interval: "30s"
        cache_ttl: "2m"

    # 缓存配置
    cache:
      enabled: true
      ttl: "5m"
      max_size: 1000

    # 日志配置
    logging:
      level: "info"
      format: "json"
      output: "stdout"

    # 性能配置
    performance:
      max_concurrent_schedules: 10
      schedule_timeout: "30s"
      cleanup_interval: "1m"

    # 监控配置
    monitoring:
      metrics_enabled: true
      health_check_enabled: true
      status_port: 8081
```

### 第四步：创建RBAC权限

创建 `deploy/rbac.yaml`：

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: uav-scheduler
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: uav-scheduler
rules:
- apiGroups: [""]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["monitor.k8s-llm-monitor.com"]
  resources: ["uavmetrics"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: uav-scheduler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: uav-scheduler
subjects:
- kind: ServiceAccount
  name: uav-scheduler
  namespace: default
```

### 第五步：部署调度器

创建 `deploy/deployment.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-scheduler
  namespace: default
  labels:
    app: uav-scheduler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: uav-scheduler
  template:
    metadata:
      labels:
        app: uav-scheduler
    spec:
      serviceAccountName: uav-scheduler
      containers:
      - name: uav-scheduler
        image: your-registry/uav-scheduler:latest  # 替换为你的镜像
        imagePullPolicy: Always
        command: ["./uav-scheduler"]
        args: ["-config=/etc/config/config.yaml", "-log-level=info"]
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 8081
          name: status
        volumeMounts:
        - name: config
          mountPath: /etc/config
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: uav-scheduler-config
```

### 第六步：创建服务

创建 `deploy/service.yaml`：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: uav-scheduler-service
  namespace: default
  labels:
    app: uav-scheduler
spec:
  selector:
    app: uav-scheduler
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  - port: 8081
    targetPort: 8081
    name: status
  type: ClusterIP
```

### 部署所有组件

```bash
# 依次部署所有组件
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml

# 检查部署状态
kubectl get pods -l app=uav-scheduler
kubectl logs -f deployment/uav-scheduler
```

## 🎯 使用示例

### 创建侦察任务Pod

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: surveillance-mission-1
  namespace: default
  annotations:
    uav-scheduler/algorithm: "nsga2"
    uav-scheduler/task-type: "surveillance"
    uav-scheduler/target-coverage: "0.8"
    uav-scheduler/priority: "high"
    uav-scheduler/objectives: "battery,latency,utilization,count"
    uav-scheduler/max-uavs: "5"
spec:
  restartPolicy: Never
  containers:
  - name: mission-container
    image: busybox
    command: ["/bin/sh", "-c", "echo 'Mission started' && sleep 3600"]
EOF
```

### 监控调度结果

```bash
# 查看调度日志
kubectl logs -f deployment/uav-scheduler

# 查看UAV节点数据
kubectl get uavmetrics
kubectl describe uavmetric uav-node-1

# 查看任务状态
kubectl get pod surveillance-mission-1 -o yaml

# 查看任务Pod的调度结果
kubectl get pod surveillance-mission-1 -o jsonpath='{.metadata.annotations.uav-scheduler\.result}'
```

## 🔧 配置详解

### NSGA-II算法参数调优

| 参数 | 默认值 | 建议范围 | 说明 |
|------|--------|----------|------|
| `population_size` | 50 | 20-100 | 种群大小，影响解的多样性 |
| `max_generations` | 20 | 10-50 | 最大迭代次数，影响收敛精度 |
| `crossover_prob` | 0.9 | 0.7-0.95 | 交叉概率，影响算法探索能力 |
| `grid_density` | 40 | 20-80 | 网格密度，影响覆盖率计算精度 |

### 任务约束参数

| 参数 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `target_coverage` | float | 目标覆盖率 | 0.8 (80%) |
| `max_uavs` | int | 最大UAV数量 | 5 |
| `min_battery` | float | 最小电池电量 | 50.0 |
| `max_latency` | float | 最大延迟 | 200.0 |

## 📊 性能监控

### 关键性能指标

- **调度延迟**: <100ms (包含数据获取和算法优化)
- **算法执行时间**: ~50ms (NSGA-II优化)
- **缓存命中率**: >80% (减少重复数据获取)
- **内存使用**: <512MB (标准配置)
- **CPU使用**: <500m (高负载情况)

### 监控命令

```bash
# 查看调度器状态
curl http://uav-scheduler-service:8081/health

# 查看调度器指标
curl http://uav-scheduler-service:8081/metrics

# 查看调度器统计信息
curl http://uav-scheduler-service:8081/status
```

## 🐛 故障排除

### 常见问题

#### 1. 调度器无法启动
```bash
# 检查Pod状态
kubectl get pods -l app=uav-scheduler

# 查看详细错误
kubectl describe pod <pod-name>
kubectl logs <pod-name>

# 检查配置
kubectl get configmap uav-scheduler-config -o yaml
```

#### 2. CRD数据获取失败
```bash
# 检查CRD是否已创建
kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com

# 检查UAVMetric资源
kubectl get uavmetrics -A

# 检查RBAC权限
kubectl auth can-i get uavmetrics --as=system:serviceaccount:default:uav-scheduler
```

#### 3. NSGA-II算法执行失败
```bash
# 检查Python环境（如果在容器内）
kubectl exec -it <pod-name> -- python3 --version

# 检查算法脚本
kubectl exec -it <pod-name> -- ls -la /app/RUN2.py

# 查看算法执行日志
kubectl logs <pod-name> | grep "NSGA-II"
```

#### 4. Pod绑定失败
```bash
# 检查集群资源
kubectl top nodes
kubectl describe nodes

# 检查RBAC权限
kubectl auth can-i create pods --as=system:serviceaccount:default:uav-scheduler

# 检查网络策略
kubectl get networkpolicies -A
```

### 调试技巧

```bash
# 启用调试模式
kubectl patch deployment uav-scheduler -p '{"spec":{"template":{"spec":{"containers":[{"name":"uav-scheduler","args":["-config=/etc/config/config.yaml","-log-level=debug"]}]}}}}'

# 查看实时日志
kubectl logs -f deployment/uav-scheduler | grep -E "(error|warn|debug)"

# 进入容器调试
kubectl exec -it <pod-name> -- /bin/sh
```

## 🔐 生产环境建议

### 安全配置

1. **网络隔离**: 使用NetworkPolicy限制调度器访问
2. **RBAC最小权限**: 只授予必要的Kubernetes权限
3. **镜像安全**: 使用扫描过的镜像，设置安全上下文
4. **密钥管理**: 使用Kubernetes Secret管理敏感配置

### 高可用部署

```yaml
# 高可用部署示例
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-scheduler-ha
spec:
  replicas: 3  # 多副本
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - uav-scheduler
              topologyKey: kubernetes.io/hostname
```

### 监控和告警

```yaml
# Prometheus监控规则示例
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: uav-scheduler-alerts
spec:
  groups:
  - name: uav-scheduler
    rules:
    - alert: UAVSchedulerDown
      expr: up{job="uav-scheduler"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "UAV调度器不可用"
    - alert: HighSchedulingLatency
      expr: uav_scheduler_duration_seconds > 0.1
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "调度延迟过高"
```

## 🤝 开发指南

### 本地开发环境

```bash
# 前置要求
go version  # >= 1.19
python3 --version  # >= 3.8
kubectl version

# 编译
cd cmd/scheduler
go mod tidy
go build -o uav-scheduler

# 本地测试
./uav-scheduler -config=config.yaml -dry-run=true -log-level=debug
```

### 添加新算法

1. 实现`Algorithm`接口：
```go
type YourAlgorithm struct {
    name   string
    config map[string]interface{}
    logger *logrus.Logger
}

func (a *YourAlgorithm) Name() string {
    return a.name
}

func (a *YourAlgorithm) Optimize(ctx context.Context, req *OptimizeRequest) (*OptimizeResult, error) {
    // 实现你的优化逻辑
    return &OptimizeResult{
        SelectedNodes:  []string{"node1", "node2"},
        CoverageRatio: 0.85,
        Score:          90.0,
    }, nil
}

func (a *YourAlgorithm) Validate(config map[string]interface{}) error {
    // 验证算法配置
    return nil
}
```

2. 注册算法：
```go
// 在main.go中
algorithm := NewYourAlgorithm("your-algorithm", config)
if err := sched.RegisterAlgorithm(algorithm); err != nil {
    logrus.WithError(err).Fatal("Failed to register algorithm")
}
```

### 贡献流程

1. Fork项目
2. 创建功能分支
3. 提交代码
4. 创建Pull Request
5. 代码审查和合并

---

**注意**: 这是一个实验性的UAV调度系统，请在测试环境中充分验证后再用于生产环境。