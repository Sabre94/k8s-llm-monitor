# UAV Scheduler - 基于NSGA-II的无人机调度器

这是一个基于NSGA-II多目标优化算法的无人机调度器，可以从CRD获取数据并智能调度Pod到最优的UAV节点。

## 🚀 功能特性

- **可插拔算法**: 支持NSGA-II等多种调度算法
- **CRD数据源**: 从UAVMetric CRD获取实时无人机状态数据
- **智能调度**: 基于电池、延迟、利用率等多目标优化
- **Pod绑定**: 自动将Pod绑定到最优节点
- **缓存机制**: 提高调度性能
- **监控指标**: 提供详细的调度统计信息

## 📁 项目结构

```
cmd/scheduler/
├── main.go                 # 主程序入口
├── config.yaml            # 配置文件
├── go.mod                 # Go模块定义
├── README.md              # 文档
└── pkg/scheduler/         # 调度器包
    ├── types.go           # 类型定义和接口
    ├── scheduler.go       # 调度器主实现
    ├── nsga2_algorithm.go # NSGA-II算法集成
    ├── crd_provider.go    # CRD数据提供者
    ├── pod_binder.go      # Pod绑定器
    └── cache.go           # 缓存实现
```

## 🔧 快速开始

### 1. 环境要求

- Go 1.21+
- Kubernetes集群访问权限
- Python 3.x (用于NSGA-II算法)
- RUN2.py脚本 (NSGA-II算法实现)

### 2. 安装依赖

```bash
cd cmd/scheduler
go mod tidy
```

### 3. 配置

编辑 `config.yaml` 文件：

```yaml
# 基础配置
name: "uav-scheduler"
listen_addr: ":8080"

# Kubernetes配置
k8s:
  namespace: "default"
  in_cluster: false
  kubeconfig: "/path/to/kubeconfig"

# 算法配置
algorithms:
  nsga2:
    population_size: 50
    max_generations: 20
    crossover_prob: 0.9
    python_path: "python3"
    script_path: "RUN2.py"

# 演示配置
demo:
  enabled: true
```

### 4. 运行

#### 开发模式 (Dry Run)
```bash
go run main.go -config=config.yaml -dry-run=true -log-level=debug
```

#### 生产模式
```bash
go run main.go -config=config.yaml -log-level=info
```

#### 构建运行
```bash
go build -o uav-scheduler main.go
./uav-scheduler -config=config.yaml
```

## 🎯 使用方式

### 1. 单个Pod调度

```go
req := &scheduler.ScheduleRequest{
    PodName:      "my-uav-pod",
    PodNamespace: "default",
    AlgorithmName: "nsga2",
    Requirements: &scheduler.OptimizeRequest{
        TaskType:       "sustain",
        TargetCoverage: 0.8,
        MaxUAVs:        5,
        Objectives:     []string{"battery", "latency", "utilization", "count"},
        Constraints: map[string]interface{}{
            "min_battery": 50.0,
            "max_latency": 200.0,
        },
    },
}

result, err := scheduler.SchedulePod(ctx, req)
```

### 2. Pod组调度

```go
pods := []scheduler.PodRef{
    {Name: "pod-1", Namespace: "default", Priority: 1},
    {Name: "pod-2", Namespace: "default", Priority: 2},
}

req := &scheduler.ScheduleGroupRequest{
    Pods:          pods,
    AlgorithmName: "nsga2",
    Requirements: &scheduler.OptimizeRequest{
        TaskType:       "emergency",
        TargetCoverage: 0.9,
        MaxUAVs:        8,
    },
}

result, err := scheduler.SchedulePodGroup(ctx, req)
```

### 3. CRD数据格式

UAVMetric CRD示例：

```yaml
apiVersion: monitoring.io/v1
kind: UAVMetric
metadata:
  name: uav-node-1
  namespace: default
spec:
  nodeId: "uav-node-1"
  battery: 85.5
  latency: 45.2
  utilization: 35.8
  gps: [34.043392, -118.266096]
  radius: 480.9
  status: "ready"
  labels:
    type: "uav"
    zone: "zone-a"
```

## 🔌 算法扩展

### 添加新算法

1. 实现 `Algorithm` 接口：

```go
type MyAlgorithm struct {
    name   string
    config map[string]interface{}
}

func (a *MyAlgorithm) Name() string {
    return a.name
}

func (a *MyAlgorithm) Optimize(ctx context.Context, req *OptimizeRequest) (*OptimizeResult, error) {
    // 实现你的优化逻辑
    return result, nil
}

func (a *MyAlgorithm) Validate(config map[string]interface{}) error {
    // 验证配置
    return nil
}
```

2. 注册算法：

```go
myAlgo := NewMyAlgorithm("my-algorithm", config)
scheduler.RegisterAlgorithm(myAlgo)
```

## 📊 监控和指标

### 获取调度器状态

```go
status := scheduler.GetStatus()
fmt.Printf("Total schedules: %d\n", status.TotalSchedules)
fmt.Printf("Success rate: %.2f%%\n", status.SuccessRate*100)
```

### 算法性能统计

```json
{
  "algorithms": {
    "nsga2": {
      "name": "nsga2",
      "enabled": true,
      "total_runs": 150,
      "avg_runtime": "2.5s",
      "success_rate": 0.95,
      "last_run": "2024-01-15T10:30:00Z"
    }
  }
}
```

## ⚙️ 配置选项

### 算法配置

- **population_size**: NSGA-II种群大小 (默认: 50)
- **max_generations**: 最大迭代次数 (默认: 20)
- **crossover_prob**: 交叉概率 (默认: 0.9)
- **grid_density**: 网格密度 (默认: 40)

### 缓存配置

- **enabled**: 是否启用缓存 (默认: true)
- **ttl**: 缓存TTL (默认: 5m)
- **max_size**: 最大缓存条目数 (默认: 1000)

### 性能配置

- **max_concurrent_schedules**: 最大并发调度数 (默认: 10)
- **schedule_timeout**: 调度超时时间 (默认: 30s)

## 🐛 故障排除

### 常见问题

1. **NSGA-II脚本执行失败**
   - 检查Python路径和脚本路径配置
   - 确保RUN2.py文件存在且可执行

2. **CRD数据获取失败**
   - 检查Kubernetes连接配置
   - 确认CRD已正确部署

3. **Pod绑定失败**
   - 检查节点是否存在且可用
   - 确认RBAC权限配置正确

### 调试模式

```bash
go run main.go -config=config.yaml -log-level=debug -dry-run=true
```

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📄 许可证

MIT License