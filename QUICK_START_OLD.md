# 🚀 UAV Monitor - 快速开始指南

## 📋 概述

本指南帮助您在新的Kubernetes集群中快速部署UAV Monitor系统，实现GPS距离路由功能。

## 🎯 前置要求

- Kubernetes 1.21+ (k3d/minikube/RKE等)
- kubectl 已配置
- 可选: Docker (用于构建自定义镜像)

## ⚡ 5分钟快速部署

### 1. 创建命名空间

```bash
kubectl create namespace uav-demo
kubectl create namespace uav-system
```

### 2. 部署Istio Ambient模式

```bash
# 下载Istio
curl -L https://istio.io/downloadIstio | sh -s 1.27.3
export PATH="$PATH:$PWD/istio-1.27.3/bin"

# 安装Ambient模式
istioctl install --set profile=ambient -y

# 为UAV命名空间启用Ambient
kubectl label namespace uav-demo istio.io/rev=default
kubectl label namespace uav-system istio.io/rev=default
```

### 3. 部署CRD定义

```bash
# 部署UAV监控CRD
kubectl apply -f api/crd/uav-metrics.yaml

# 部署UAV路由CRD
kubectl apply -f api/crd/uav-routing.yaml

# 验证CRD
kubectl get crd | grep uav
```

### 4. 部署UAV节点

```bash
# 部署3个UAV节点 (使用预构建镜像)
kubectl apply -f infrastructure/kubernetes/uav-nodes.yaml

# 等待节点就绪
kubectl wait --for=condition=ready pod -l app=uav-node -n uav-demo --timeout=60s
```

### 5. 配置智能路由

```bash
# 部署路由规则
kubectl apply -f infrastructure/istio/ambient/routing-rules.yaml

# 验证路由配置
kubectl get virtualservices -n uav-demo
```

### 6. 测试路由功能

```bash
# 创建测试客户端
kubectl run test-client --image=curlimages/curl:latest -n uav-demo --restart=Never -- sleep=3600

# 测试GPS距离路由
kubectl exec -it test-client -n uav-demo -- \
  curl -H "x-source-location: downtown-la" \
  http://uav-smart-service.uav-demo.svc.cluster.local
```

## 📦 详细部署选项

### 选项1: 本地测试模式

```bash
cd cmd/scheduler
go build -o uav-scheduler
./uav-scheduler -config=config.yaml -dry-run=true -log-level=info
```

### 选项2: 完整Kubernetes部署

```bash
# 1. 构建镜像
docker build -t your-registry/uav-scheduler:latest -f cmd/scheduler/Dockerfile .

# 2. 推送镜像
docker push your-registry/uav-scheduler:latest

# 3. 修改部署文件中的镜像地址
# 编辑 deploy/quick-start.sh 中的镜像名称

# 4. 运行部署脚本
./deploy/quick-start.sh production
```

## 🎯 调度流程验证

当你看到类似以下的日志时，说明调度器正在工作：

```
{"level":"info","msg":"Demo: Single pod scheduling"}
{"level":"info","msg":"NSGA-II optimization completed","selected_nodes":["uav-node-3","uav-node-1","uav-node-2"]}
{"level":"info","msg":"Mission scheduling completed successfully","node_count":3}
```

这表示：
- ✅ NSGA-II算法成功优化
- ✅ 自动选择了3个最优UAV节点
- ✅ 节点集合已绑定到任务

## 🐛 常见问题

**Q: 调度器Pod无法启动？**
```bash
# 检查Pod状态
kubectl describe pod -l app=uav-scheduler

# 确保镜像已构建
docker images | grep uav-scheduler
```

**Q: 没有看到demo执行？**
```bash
# 检查配置中的demo设置
kubectl get configmap uav-scheduler-config -o yaml

# 手动重启调度器
kubectl rollout restart deployment/uav-scheduler
```

**Q: UAVMetric CRD不存在？**
```bash
# 检查CRD状态
kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com

# 重新创建CRD
kubectl apply -f docs/UAV_SCHEDULER_README.md
```

## 📚 更多信息

- 📖 **完整文档**: [docs/UAV_SCHEDULER_README.md](docs/UAV_SCHEDULER_README.md)
- 🔧 **配置详解**: [docs/UAV_SCHEDULER_README.md#配置详解](docs/UAV_SCHEDULER_README.md#配置详解)
- 🐛 **故障排除**: [docs/UAV_SCHEDULER_README.md#故障排除](docs/UAV_SCHEDULER_README.md#故障排除)

## 🎉 成功标志

当你看到以下情况时，说明部署成功：

1. ✅ 调度器Pod运行正常
2. ✅ 5个UAV节点数据创建成功
3. ✅ Demo模式自动执行
4. ✅ 日志显示"NSGA-II optimization completed"
5. ✅ 节点集合绑定成功

恭喜！你的UAV智能调度系统已经运行起来了！🚁✨