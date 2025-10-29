# 🚁 K8s UAV Monitor - 智能无人机集群监控系统

基于Kubernetes和Istio的智能无人机集群监控系统，支持GPS距离路由、实时监控和智能调度。

## 🎯 核心特性

- 🌍 **GPS距离路由**: 基于真实GPS数据的智能路由决策
- 🤖 **智能调度**: 多目标优化调度算法 (NSGA-II、距离优化等)
- 📊 **实时监控**: 实时收集和分析UAV集群状态
- 🔧 **边缘计算**: 分布式路由决策，适合边缘部署场景
- 🌐 **服务网格**: 基于Istio Ambient模式的流量管理
- 📱 **可视化界面**: Web管理界面和实时监控面板

## 📁 项目结构

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
│   └── dashboard/             # 监控面板
├── 🖥️ frontend/               # 前端界面
│   └── web/                   # Web管理界面
├── 🔧 infrastructure/         # 基础设施
│   ├── istio/                 # Istio服务网格配置
│   └── kubernetes/           # K8s部署配置
└── 📚 docs/                   # 文档
```

## 🚀 快速开始

### 前置要求

- Kubernetes 1.21+
- Istio 1.27+ (Ambient模式)
- Go 1.25+
- Docker

### 1. 部署CRD (API层)

```bash
# 部署UAV Metrics CRD
kubectl apply -f api/crd/uav-metrics.yaml

# 部署UAV Routing CRD
kubectl apply -f api/crd/uav-routing.yaml

# 验证CRD部署
kubectl get crd | grep uav
```

### 2. 部署Istio Ambient模式

```bash
# 下载并安装Istio
curl -L https://istio.io/downloadIstio | sh -
export PATH="$PATH:$PWD/istio-1.27.3/bin"

# 安装Istio Ambient模式
istioctl install --set profile=ambient -y

# 验证Istio部署
kubectl get pods -n istio-system
```

### 3. 部署UAV集群

```bash
# 部署UAV代理 (GPS服务)
kubectl apply -f infrastructure/kubernetes/uav-agent-daemonset.yaml

# 部署UAV节点
kubectl apply -f infrastructure/kubernetes/uav-nodes.yaml

# 验证部署
kubectl get pods -n uav-demo
```

### 4. 部署智能调度器

```bash
# 构建调度器镜像
docker build -t uav-scheduler:latest -f infrastructure/kubernetes/Dockerfile.scheduler .

# 部署调度器
kubectl apply -f infrastructure/kubernetes/scheduler-deployment.yaml

# 验证调度器
kubectl get pods -n uav-system
```

### 5. 配置智能路由

```bash
# 配置GPS距离路由
kubectl apply -f infrastructure/istio/ambient/routing-rules.yaml

# 验证路由配置
kubectl get virtualservices -n uav-demo
```

## 🎮 使用示例

### GPS距离路由演示

```bash
# 查看UAV节点状态
kubectl get uavmetrics -n uav-demo

# 查看路由决策
kubectl get uavroutings -n uav-demo

# 测试路由
kubectl exec -it routing-test-client -n uav-demo -- \
  curl -H "x-source-location: downtown-la" uav-smart-service.uav-demo.svc.cluster.local
```

### 智能调度演示

```bash
# 创建调度任务
kubectl apply -f examples/mission-request.yaml

# 查看调度结果
kubectl get pods -n uav-system
```

## 🔧 开发指南

### 添加新的调度算法

1. 在 `scheduler/algorithms/` 中实现新算法
2. 实现 `Algorithm` 接口
3. 注册到调度器中

### 添加新的路由策略

1. 在 `scheduler/routing/` 中扩展路由计算
2. 更新距离计算逻辑
3. 配置Istio VirtualService

### 添加新的监控指标

1. 在 `monitoring/metrics/` 中添加指标收集器
2. 更新Prometheus配置
3. 添加监控面板

## 📊 监控和观察

### 访问监控面板

```bash
# 端口转发
kubectl port-forward svc/uav-dashboard 8080:8080 -n uav-demo

# 访问Web界面
open http://localhost:8080
```

### 查看实时数据

```bash
# 查看UAV状态
kubectl get uavmetrics -o wide -n uav-demo

# 查看路由决策
kubectl describe uavrouting <routing-name> -n uav-demo

# 查看Istio路由
istioctl proxy-config routes -n uav-demo
```

## 🌟 架构特点

### 分布式路由决策
- 每个UAV节点基于自身GPS位置计算最优路由
- 支持实时位置更新和动态路由调整
- 适合边缘计算场景，减少网络延迟

### 多目标优化调度
- 支持NSGA-II多目标优化算法
- 综合考虑距离、电池、延迟等因素
- 支持自定义调度策略

### Istio Ambient集成
- 无需sidecar注入，资源占用低
- 支持L4和L7路由控制
- 实时配置更新

## 🤝 贡献指南

1. Fork项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启Pull Request

## 📄 许可证

MIT License - 查看 [LICENSE](LICENSE) 文件了解详情

## 🆘 支持

- 📖 [完整文档](docs/)
- 🐛 [问题报告](https://github.com/yourusername/k8s-llm-monitor/issues)
- 💬 [讨论区](https://github.com/yourusername/k8s-llm-monitor/discussions)

---

🚁 **让无人机集群更智能，让边缘计算更高效！**