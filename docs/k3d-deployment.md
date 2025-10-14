# K3d 部署指南

本文档说明如何在k3d集群上部署K8s LLM Monitor。

## 前提条件

1. 安装k3d:
```bash
# macOS
brew install k3d

# 或使用官方脚本
wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
```

2. 创建k3d集群:
```bash
# 创建单节点集群
k3d cluster create mycluster

# 或创建多节点集群(用于测试跨节点网络)
k3d cluster create mycluster --servers 1 --agents 2

# 验证集群
kubectl get nodes
```

## 步骤1: 安装Metrics Server

k3d默认不包含metrics-server,需要手动安装:

```bash
# 安装metrics-server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# k3d需要禁用TLS验证
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

# 验证metrics-server运行
kubectl get pods -n kube-system | grep metrics-server

# 测试metrics API
kubectl top nodes
kubectl top pods -A
```

## 步骤2: 配置监控工具

修改配置文件适配k3d:

```yaml
# configs/config.yaml
server:
  host: "0.0.0.0"
  port: 8080
  debug: true

k8s:
  kubeconfig: "/Users/xiabin/.kube/config"  # k3d自动更新此文件
  namespace: "default"
  watch_namespaces: "default,kube-system"

metrics:
  enabled: true
  collect_interval: 30
  namespaces:
    - default
    - kube-system
  enable_node: true
  enable_pod: true
  enable_network: true  # k3d完全支持网络测试
  enable_custom: false
```

## 步骤3: 部署测试Pod

在k3d集群中部署一些测试Pod:

```bash
# 部署busybox(用于网络测试)
kubectl run busybox --image=busybox:latest --restart=Always -- sleep 3600

# 部署nginx(提供HTTP服务)
kubectl create deployment nginx --image=nginx:alpine
kubectl expose deployment nginx --port=80

# 验证Pod运行
kubectl get pods
```

## 步骤4: 运行监控工具

```bash
# 确保使用正确的kubeconfig
export KUBECONFIG=~/.kube/config

# 运行监控工具
go run cmd/server/main.go -config=./configs/config.yaml
```

## 步骤5: 验证功能

访问以下API验证各项功能:

```bash
# 1. 健康检查
curl http://localhost:8080/health

# 2. 集群状态
curl http://localhost:8080/api/v1/cluster/status

# 3. Pod列表
curl http://localhost:8080/api/v1/pods

# 4. 集群指标
curl http://localhost:8080/api/v1/metrics/cluster

# 5. 节点指标
curl http://localhost:8080/api/v1/metrics/nodes

# 6. 网络指标
curl http://localhost:8080/api/v1/metrics/network
```

## k3d vs Minikube 差异

| 特性 | k3d | Minikube |
|------|-----|----------|
| 基础技术 | k3s + Docker | 多种驱动(VM/Docker/etc) |
| 启动速度 | 更快 | 较慢 |
| 资源占用 | 更低 | 较高 |
| Metrics Server | 需手动安装 | 可通过addon安装 |
| 多节点 | 原生支持 | 需额外配置 |
| GPU支持 | 有限 | 更好 |

## 常见问题

### 1. Metrics API不可用
```bash
# 检查metrics-server状态
kubectl get pods -n kube-system | grep metrics-server

# 查看日志
kubectl logs -n kube-system deployment/metrics-server

# 确保添加了--kubelet-insecure-tls参数
kubectl describe deployment metrics-server -n kube-system | grep kubelet-insecure
```

### 2. 网络测试失败
```bash
# 确保busybox Pod正在运行
kubectl get pods busybox

# 测试Pod间网络
kubectl exec busybox -- ping -c 3 <other-pod-ip>
```

### 3. kubeconfig路径问题
```bash
# 检查k3d配置
k3d kubeconfig get mycluster

# 合并到默认kubeconfig
k3d kubeconfig merge mycluster --kubeconfig-merge-default
```

## 性能优化建议

1. **k3d集群配置**:
```bash
# 创建集群时分配更多资源
k3d cluster create mycluster --agents 2 --servers 1 \
  --k3s-arg "--disable=traefik@server:0" \  # 禁用不需要的组件
  --port 8080:80@loadbalancer
```

2. **指标采集间隔**:
```yaml
metrics:
  collect_interval: 60  # k3d资源有限,可适当增加间隔
```

3. **减少测试Pod对数**:
```go
// cmd/server/main.go
NetworkMaxPairs: 3,  // k3d环境减少为3对
```

## 结论

k8s-llm-monitor完全兼容k3d,是开发和测试的理想选择。主要区别在于:
- ✅ 需要手动安装metrics-server
- ✅ 配置metrics-server时需禁用TLS验证
- ✅ 建议适当调整采集频率和测试规模

k3d的轻量级特性使其非常适合本地开发和功能验证!
