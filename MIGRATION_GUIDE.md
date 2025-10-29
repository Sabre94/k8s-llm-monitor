# 🚀 UAV Monitor 集群迁移指南

## 📋 概述

本指南将帮助您将UAV Monitor项目完整移植到新的Kubernetes集群中。包括完整的部署流程、配置验证和故障排除。

## 🎯 前置要求

### 目标集群要求
- Kubernetes 1.21+ (推荐 1.25+)
- 节点数量: 至少2个节点用于UAV部署
- 内存: 每个节点至少 2GB 可用内存
- 网络: 集群内Pod间网络互通

### 本地环境要求
- kubectl 已配置并能访问目标集群
- Docker 已安装并运行
- Helm 3.0+ (可选，用于简化部署)

## 📦 完整部署流程

### Phase 1: 集群准备和验证

```bash
# 1. 验证集群状态
kubectl cluster-info
kubectl get nodes
kubectl get namespaces

# 2. 创建命名空间
kubectl create namespace uav-demo
kubectl create namespace uav-system

# 3. 验证权限
kubectl auth can-i create namespace
kubectl get pods --all-namespaces
```

### Phase 2: 部署API层 (CRD)

```bash
# 1. 部署UAV Metrics CRD
kubectl apply -f api/crd/uav-metrics.yaml

# 2. 部署UAV Routing CRD
kubectl apply -f api/crd/uav-routing.yaml

# 3. 验证CRD部署
kubectl get crd | grep uav
kubectl describe crd uavmetrics.monitoring.io
kubectl describe crd uavroutings.monitoring.io

# 4. 验证API访问
kubectl api-resources | grep uav
```

### Phase 3: 部署Istio Ambient模式

```bash
# 1. 下载Istio
curl -L https://istio.io/downloadIstio | sh -
export PATH="$PATH:$PWD/istio-1.27.3/bin"

# 2. 验证Istio版本
istioctl version

# 3. 安装Istio Ambient模式
istioctl install --set profile=ambient -y

# 4. 验证Istio部署
kubectl get pods -n istio-system
kubectl get svc -n istio-system

# 5. 为UAV命名空间启用Ambient模式
kubectl label namespace uav-demo istio.io/rev=default
kubectl label namespace uav-system istio.io/rev=default

# 6. 验证ztunnel部署
kubectl get pods -n istio-system | grep ztunnel
```

### Phase 4: 构建和部署UAV组件

```bash
# 1. 构建UAV代理镜像
cd uav-agent
docker build -t uav-agent:latest .

# 2. 构建GPS服务镜像
cd ../uav-agent/gps
docker build -t uav-gps-service:latest .

# 3. 构建调度器镜像
cd ../../scheduler
docker build -t uav-scheduler:latest -f Dockerfile.scheduler .

# 4. 构建API服务镜像
cd ../../api/rest
docker build -t uav-api:latest .

# 5. 推送镜像到仓库 (可选)
docker tag uav-agent:latest your-registry/uav-agent:latest
docker push your-registry/uav-agent:latest
# ... 对其他镜像执行相同操作
```

### Phase 5: 部署UAV组件

```bash
# 1. 部署UAV Agent DaemonSet
kubectl apply -f infrastructure/kubernetes/uav-agent-daemonset.yaml

# 2. 部署UAV GPS服务
kubectl apply -f infrastructure/kubernetes/uav-gps-service.yaml

# 3. 部署智能调度器
kubectl apply -f infrastructure/kubernetes/scheduler-deployment.yaml

# 4. 部署API服务
kubectl apply -f infrastructure/kubernetes/api-service.yaml

# 5. 部署监控组件
kubectl apply -f infrastructure/kubernetes/monitoring-deployment.yaml

# 6. 部署Web界面
kubectl apply -f infrastructure/kubernetes/frontend-deployment.yaml
```

### Phase 6: 配置Istio智能路由

```bash
# 1. 部署路由规则
kubectl apply -f infrastructure/istio/ambient/routing-rules.yaml

# 2. 部署服务发现配置
kubectl apply -f infrastructure/istio/ambient/service-discovery.yaml

# 3. 部署虚拟服务
kubectl apply -f infrastructure/istio/ambient/virtual-services.yaml

# 4. 验证路由配置
kubectl get virtualservices -n uav-demo
kubectl get destinationrules -n uav-demo
```

## 🧪 部署验证

### 1. 验证Pod状态

```bash
# 检查所有Pod状态
kubectl get pods -n uav-demo
kubectl get pods -n uav-system

# 检查Pod就绪状态
kubectl get pods -n uav-demo -o wide
kubectl get pods -n uav-system -o wide

# 检查Istio代理注入
kubectl describe pod -n uav-demo $(kubectl get pods -n uav-demo -l app=uav-agent -o jsonpath='{.items[0].metadata.name}') | grep istio
```

### 2. 验证服务连通性

```bash
# 创建测试客户端
kubectl run routing-test-client --image=curlimages/curl:latest -n uav-demo --restart=Never -- sleep=3600

# 测试GPS服务
kubectl exec -it routing-test-client -n uav-demo -- curl -s http://$(kubectl get pods -n uav-demo -l app=uav-agent -o jsonpath='{.items[0].status.podIP}'):9090/health

# 测试路由服务
kubectl exec -it routing-test-client -n uav-demo -- curl -s http://uav-smart-service.uav-demo.svc.cluster.local
```

### 3. 验证路由功能

```bash
# 测试基于位置的路由
kubectl exec -it routing-test-client -n uav-demo -- \
  curl -H "x-source-location: downtown-la" \
  uav-smart-service.uav-demo.svc.cluster.local

# 测试调度器API
kubectl port-forward svc/uav-scheduler-service 8080:8080 -n uav-system &
curl -X POST http://localhost:8080/api/v1/schedule \
  -H "Content-Type: application/json" \
  -d '{"pod_name": "test-uav", "algorithm": "nsga2"}'
```

## 🔧 配置优化

### 1. 节点标签和污点

```yaml
# 为UAV节点添加标签
kubectl label nodes <node-name> uav-node=true zone=production

# 设置污点避免调度到不合适的节点
kubectl taint nodes <node-name> uav-only=true:NoSchedule
```

### 2. 资源限制和请求

```yaml
# 修改UAV代理的资源限制
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "200m"
```

### 3. 网络策略

```yaml
# 允许UAV节点间通信
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: uav-internal-traffic
  namespace: uav-demo
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: uav-agent
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: uav-agent
```

## 📊 监控和日志

### 1. 设置监控

```bash
# 部署Prometheus
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace

# 配置服务发现
kubectl apply -f infrastructure/kubernetes/monitoring-rbac.yaml
```

### 2. 设置日志收集

```bash
# 安装Fluentd
helm repo add fluent https://fluent.github.io/helm-charts
helm install fluentd fluent/fluent-bit \
  --namespace logging --create-namespace

# 配置UAV日志收集
kubectl apply -f infrastructure/kubernetes/logging-config.yaml
```

### 3. 设置告警

```bash
# 配置告警规则
kubectl apply -f infrastructure/kubernetes/alerting-rules.yaml

# 设置告警通知
kubectl apply -f infrastructure/kubernetes/notification-config.yaml
```

## 🚨 故障排除

### 常见问题

#### 1. Istio Ambient部署问题

```bash
# 检查Istio版本兼容性
kubectl version --short
istioctl version

# 检查CNI插件
kubectl get pods -n istio-system | grep cni

# 重启istio
kubectl delete pods -n istio-system -l app=istiod
kubectl delete pods -n istio-system -l app=ztunnel
```

#### 2. Pod启动失败

```bash
# 检查Pod事件
kubectl describe pod <pod-name> -n uav-demo

# 检查资源使用
kubectl top nodes
kubectl top pods -n uav-demo

# 检查镜像拉取
kubectl get events -n uav-demo --field-selector=involvedObject.name=<pod-name>
```

#### 3. 路由不工作

```bash
# 检查VirtualService配置
kubectl get virtualservices -n uav-demo -o yaml

# 检查路由规则
istioctl proxy-config routes -n uav-demo <pod-name>

# 检查istiod日志
kubectl logs -n istio-system -l app=istiod
```

#### 4. GPS数据收集问题

```bash
# 检查GPS服务状态
kubectl get pods -n uav-demo -l app=uav-agent
kubectl logs <pod-name> -c gps-service -n uav-demo

# 测试GPS API
kubectl exec -it <pod-name> -n uav-demo -- curl -s http://localhost:9091/gps
```

## 🔄 集群间迁移

### 从A集群迁移到B集群

```bash
# 1. 在B集群创建命名空间
kubectl --context=<cluster-b> create namespace uav-demo
kubectl --context=<cluster-b> create namespace uav-system

# 2. 部署所有组件到B集群
kubectl --context=<cluster-b> apply -f api/crd/
kubectl --context=<cluster-b> apply -f infrastructure/kubernetes/
kubectl --context=<cluster-b> apply -f infrastructure/istio/

# 3. 导出A集群的配置数据
kubectl --context=<cluster-a> get uavmetrics -n uav-demo -o yaml > backup/uav-metrics.yaml
kubectl --context=<cluster-a> get uavroutings -n uav-demo -o yaml > backup/uav-routings.yaml

# 4. 导入配置到B集群
kubectl --context=<cluster-b> apply -f backup/

# 5. 验证B集群功能
kubectl --context=<cluster-b> get pods -n uav-demo
kubectl --context=<cluster-b> get virtualservices -n uav-demo
```

## 📱 备份和恢复

### 完整集群备份

```bash
# 1. 备份所有配置
kubectl get all -A -o yaml > backup/full-cluster-backup.yaml

# 2. 备份特定资源
kubectl get uavmetrics -A -o yaml > backup/uav-metrics.yaml
kubectl get uavroutings -A -o yaml > backup/uav-routings.yaml
kubectl get virtualservices -A -o yaml > backup/virtualservices.yaml

# 3. 备份工作负载
kubectl get pods -n uav-demo -o yaml > backup/uav-pods.yaml
kubectl get deployments -n uav-system -o yaml > backup/scheduler-deployments.yaml
```

### 恢复到新集群

```bash
# 1. 恢复CRD
kubectl apply -f backup/uav-metrics.yaml
kubectl apply -f backup/uav-routings.yaml

# 2. 恢复配置
kubectl apply -f backup/virtualservices.yaml

# 3. 恢复工作负载
kubectl apply -f backup/uav-pods.yaml
kubectl apply -f backup/scheduler-deployments.yaml
```

## 🎯 生产环境配置

### 安全配置

```yaml
# 启用RBAC
apiVersion: v1
kind: PodSecurityPolicy
metadata:
  name: uav-agent-psp
spec:
  privileged: false
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsUser: 1000
  runAsGroup: 3000
  fsGroup: 3000
```

### 高可用配置

```yaml
# 多副本调度器
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-scheduler
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
```

### 性能优化

```yaml
# 增加资源限制
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# 添加HPA
horizontalPodAutoscaler:
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

## 📚 部署清单

```bash
# 完整部署命令列表
kubectl apply -f api/crd/
kubectl apply -f infrastructure/kubernetes/rbac/
kubectl apply -f infrastructure/kubernetes/namespaces/
kubectl apply -f infrastructure/kubernetes/configmaps/
kubectl apply -f infrastructure/kubernetes/secrets/
kubectl apply -f infrastructure/kubernetes/uav-agent-daemonset.yaml
kubectl apply -f infrastructure/kubernetes/uav-gps-service.yaml
kubectl apply -f infrastructure/kubernetes/scheduler-deployment.yaml
kubectl apply -f infrastructure/kubernetes/api-service.yaml
kubectl apply -f infrastructure/kubernetes/monitoring-deployment.yaml
kubectl apply -f infrastructure/istio/ambient/
```

## 🎯 成功验证标准

✅ **部署成功指标**:
- 所有Pod处于Running状态
- 所有服务可访问
- CRD资源创建成功
- Istio Ambient模式正常运行

✅ **功能验证指标**:
- GPS数据正常收集
- 路由决策工作正常
- 调度器API响应正常
- Web界面可访问

✅ **性能验证指标**:
- Pod启动时间 < 2分钟
- API响应时间 < 500ms
- 路由决策延迟 < 100ms

现在你有了完整的迁移指南，可以轻松将UAV Monitor项目部署到任何Kubernetes集群中！🚁✨