# 🚁 K8s UAV Monitor 项目结构

## 📁 整体架构

```
k8s-llm-monitor/
├── 📦 api/                    # API接口层
│   ├── crd/                   # CRD定义和管理
│   └── rest/                  # REST API服务
├── 🤖 scheduler/              # 智能调度器
│   ├── algorithms/            # 调度算法
│   ├── routing/              # 路由计算引擎
│   └── decision/             # 决策引擎
├── 🚁 uav-agent/              # UAV代理程序
│   ├── gps/                   # GPS数据收集
│   ├── telemetry/            # 遥测数据
│   └── control/               # 控制接口
├── 🌐 monitoring/             # 监控系统
│   ├── metrics/               # 指标收集
│   ├── dashboard/             # 监控面板
│   └── alerts/                # 告警系统
├── 🖥️ frontend/               # 前端界面
│   ├── web/                   # Web管理界面
│   └── mobile/                # 移动端应用
├── 🔧 infrastructure/         # 基础设施
│   ├── istio/                 # Istio服务网格配置
│   ├── kubernetes/           # K8s部署配置
│   └── networking/           # 网络配置
├── 📚 docs/                   # 文档
├── 🧪 demos/                  # 演示程序
└── 🛠️ tools/                  # 工具脚本
```

## 🎯 各模块详细说明

### 1. 📦 API接口层 (`api/`)

#### `api/crd/` - CRD定义和管理
```yaml
# 自定义资源定义
api/crd/
├── uav-metrics.yaml          # UAV指标CRD
├── uav-routing.yaml          # 路由配置CRD
└── cluster-config.yaml       # 集群配置CRD
```

**用途**: 定义UAV相关的Kubernetes自定义资源
**主要功能**:
- UAVMetrics: 存储GPS、电池、延迟等数据
- UAVRouting: 存储路由决策结果
- ClusterConfig: 集群配置信息

#### `api/rest/` - REST API服务
```go
api/rest/
├── main.go                   # API服务入口
├── handlers/                 # HTTP处理器
│   ├── uav.go              # UAV相关API
│   ├── routing.go          # 路由API
│   └── metrics.go          # 指标API
└── middleware/              # 中间件
```

**用途**: 提供RESTful API接口
**主要功能**:
- UAV数据查询和管理
- 路由决策API
- 实时指标获取

### 2. 🤖 智能调度器 (`scheduler/`)

#### `scheduler/algorithms/` - 调度算法
```go
scheduler/algorithms/
├── nsga2.go                  # NSGA-II多目标优化
├── greedy.go                 # 贪心算法
├── distance.go               # 距离优化算法
└── battery.go                # 电池优化算法
```

**用途**: 各种UAV调度算法实现
**主要功能**:
- 多目标优化调度
- 实时决策算法
- 负载均衡策略

#### `scheduler/routing/` - 路由计算引擎
```go
scheduler/routing/
├── distance_calculator.go    # GPS距离计算
├── routing_engine.go         # 路由引擎
├── route_optimizer.go        # 路由优化
└── edge_router.go            # 边缘路由器
```

**用途**: 基于GPS的智能路由计算
**主要功能**:
- 实时GPS距离计算
- 最优路由决策
- 边缘分布式路由

#### `scheduler/decision/` - 决策引擎
```go
scheduler/decision/
├── engine.go                 # 决策引擎核心
├── scorer.go                 # 多因子评分
└── selector.go               # 节点选择器
```

**用途**: 综合决策引擎
**主要功能**:
- 多维度评分算法
- 智能节点选择
- 决策结果缓存

### 3. 🚁 UAV代理程序 (`uav-agent/`)

#### `uav-agent/gps/` - GPS数据收集
```go
uav-agent/gps/
├── collector.go               # GPS数据收集器
├── simulator.go              # GPS模拟器
└── realtime.go               # 实时GPS处理
```

**用途**: UAV GPS数据处理
**主要功能**:
- GPS数据收集和标准化
- 实时位��更新
- GPS数据验证

#### `uav-agent/telemetry/` - 遥测数据
```go
uav-agent/telemetry/
├── metrics.go                 # 指标收集
├── reporter.go               # 数据上报
└── cache.go                  # 数据缓存
```

**用途**: 遥测数据处理
**主要功能**:
- 实时指标收集
- 数据压缩和传输
- 本地缓存管理

#### `uav-agent/control/` - 控制接口
```go
uav-agent/control/
├── api.go                     # HTTP控制API
├── commands.go                # 命令处理
└── status.go                  # 状态管理
```

**用途**: UAV控制接口
**主要功能**:
- HTTP控制接口
- 飞行命令执行
- 状态同步

### 4. 🌐 监控系统 (`monitoring/`)

#### `monitoring/metrics/` - 指标收集
```go
monitoring/metrics/
├── collector.go               # 指标收集器
├── prometheus.go             # Prometheus集成
└── custom.go                  # 自定义指标
```

**用途**: 系统指标收集
**主要功能**:
- 实时指标收集
- Prometheus格式导出
- 自定义业务指标

#### `monitoring/dashboard/` - 监控面板
```javascript
monitoring/dashboard/
├── web/                       # Web面板
├── components/                # UI组件
└── charts/                   # 图表组件
```

**用途**: 可视化监控界面
**主要功能**:
- 实时数据展示
- 历史趋势分析
- 告警界面

### 5. 🖥️ 前端界面 (`frontend/`)

#### `frontend/web/` - Web管理界面
```javascript
frontend/web/
├── src/
│   ├── components/           # React组件
│   ├── pages/                # 页面组件
│   ├── services/             # API服务
│   └── utils/                # 工具函数
└── public/                   # 静态资源
```

**用途**: Web管理界面
**主要功能**:
- UAV集群管理
- 路由配置界面
- 实时监控面板

### 6. 🔧 基础设施 (`infrastructure/`)

#### `infrastructure/istio/` - Istio配置
```yaml
infrastructure/istio/
├── ambient/                  # Ambient模式配置
├── virtual-services/          # 虚拟服务
└── destination-rules/        # 目标规则
```

**用途**: Istio服务网格配置
**主要功能**:
- Ambient模式部署
- 智能路由规则
- 服务治理配置

#### `infrastructure/kubernetes/` - K8s部署
```yaml
infrastructure/kubernetes/
├── crd/                       # CRD定义
├── deployments/               # 部署配置
├── services/                  # 服务定义
└── configmaps/                # 配置映射
```

**用途**: Kubernetes部署配置
**主要功能**:
- 完整部署清单
- 配置管理
- 命名空间管理

## 🚀 使用指南

### 1. 部署CRD (API层)
```bash
kubectl apply -f infrastructure/kubernetes/crd/
```

### 2. 部署调度器
```bash
kubectl apply -f infrastructure/kubernetes/deployments/scheduler/
```

### 3. 部署UAV代理
```bash
kubectl apply -f infrastructure/kubernetes/deployments/uav-agent/
```

### 4. 部署监控系统
```bash
kubectl apply -f infrastructure/kubernetes/deployments/monitoring/
```

### 5. 部署前端
```bash
kubectl apply -f infrastructure/kubernetes/deployments/frontend/
```

### 6. 配置Istio路由
```bash
kubectl apply -f infrastructure/istio/ambient/
```

## 🎯 模块间关系

```
前端界面
    ↓ API调用
API接口层
    ↓ 数据处理
调度器核心
    ↓ 决策结果
基础设施层
    ↓ 部署执行
UAV代理
    ↓ 数据收集
监控系统
    ↓ 实时监控
前端界面 (闭环)
```

## 📚 开发指南

1. **添加新算法**: 在 `scheduler/algorithms/` 中实现
2. **添加新API**: 在 `api/rest/handlers/` 中添加处理器
3. **添加新指标**: 在 `monitoring/metrics/` 中扩展
4. **添加新功能**: 在相应模块中开发，遵循现有结构

这样的结构让你清楚地知道：
- 🎯 **要改路由算法** → 修改 `scheduler/routing/`
- 🎯 **要加新API** → 修改 `api/rest/`
- 🎯 **要改前端** → 修改 `frontend/web/`
- 🎯 **要改监控** → 修改 `monitoring/`
- 🎯 **要改部署** → 修改 `infrastructure/`