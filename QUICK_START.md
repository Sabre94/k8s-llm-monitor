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

## 🧪 验证部署

### 检查Pod状态

```bash
kubectl get pods -n uav-demo
kubectl get pods -n istio-system
```

### 检查路由决策

```bash
# 查看UAV节点状态
kubectl get uavmetrics -n uav-demo

# 查看路由配置
kubectl get uavroutings -n uav-demo
```

### 测试不同位置的路由

```bash
# 测试从圣塔莫尼卡的路由
kubectl exec -it test-client -n uav-demo -- \
  curl -H "x-source-location: santa-monica" \
  http://uav-smart-service.uav-demo.svc.cluster.local

# 测试从帕萨迪纳的路由
kubectl exec -it test-client -n uav-demo -- \
  curl -H "x-source-location: pasadena" \
  http://uav-smart-service.uav-demo.svc.cluster.local
```

## 🎮 实际使用场景

### 场景1: 添加新UAV节点

```bash
# 扩展UAV节点到5个
kubectl scale deployment uav-node-1 --replicas=2 -n uav-demo
kubectl scale deployment uav-node-2 --replicas=2 -n uav-demo
kubectl scale deployment uav-node-3 --replicas=1 -n uav-demo

# 验证新节点
kubectl get pods -l app=uav-node -n uav-demo -o wide
```

### 场景2: 自定义GPS位置

```yaml
# 编辑 uav-nodes.yaml 中的GPS坐标
apiVersion: v1
kind: ConfigMap
metadata:
  name: uav-gps-config
data:
  locations: |
    - name: "new-york"
      lat: 40.7128
      lon: -74.0060
    - name: "chicago"
      lat: 41.8781
      lon: -87.6298
```

### 场景3: 修改路由策略

```yaml
# 修改 routing-rules.yaml 中的权重分配
spec:
  http:
  - match:
    - headers:
        x-source-location:
          exact: "downtown-la"
    route:
    - destination:
        host: uav-node-1
      weight: 70        # 增加到70%
    - destination:
        host: uav-node-2
      weight: 20        # 减少到20%
    - destination:
        host: uav-node-3
      weight: 10        # 减少到10%
```

## 🔧 故障排除

### 常见问题

#### 1. Pod卡在Init状态
```bash
# 检查Istio CNI
kubectl get pods -n istio-system | grep cni

# 重启ztunnel
kubectl delete pods -n istio-system -l app=ztunnel
```

#### 2. 路由不工作
```bash
# 检查VirtualService
kubectl describe virtualservice uav-smart-service -n uav-demo

# 检查路由配置
istioctl proxy-config routes -n uav-demo $(kubectl get pods -n uav-demo -l app=uav-node-1 -o jsonpath='{.items[0].metadata.name}')
```

#### 3. CRD无法创建资源
```bash
# 检查CRD状态
kubectl get crd uavmetrics.monitoring.io -o yaml

# 重新部署CRD
kubectl delete crd uavmetrics.monitoring.io
kubectl apply -f api/crd/uav-metrics.yaml
```

## 🚀 下一步

现在您已经成功部署了UAV Monitor系统！接下来可以：

1. 📊 **部署监控系统**: 部署Prometheus和Grafana进行监控
2. 🌐 **部署Web界面**: 部署前端管理界面
3. 🔧 **自定义算法**: 实现自己的调度算法
4. 📱 **移动端集成**: 开发移动端应用

详细的配置和扩展选项请参考：
- 📖 [完整文档](docs/)
- 🏗️ [项目结构](PROJECT_STRUCTURE.md)
- 🔄 [集群迁移](MIGRATION_GUIDE.md)

## 🎯 成功验证标准

✅ **部署成功**:
- [ ] 所有Pod处于Running状态
- [ ] CRD资源可以正常创建
- [ ] Istio Ambient模式正常运行

✅ **功能验证**:
- [ ] 路由测试返回不同响应
- [ ] GPS位置影响路由决策
- [ ] 可以查看UAV监控数据

🚁 **恭喜！您的智能UAV集群已经准备就绪！**