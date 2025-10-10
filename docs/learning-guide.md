# 动手学习指南

## 学习路径：从简单到复杂

### 阶段1：理解K8s基础（1-2天）

#### 1.1 安装Minikube（本地K8s环境）
```bash
# 安装Minikube
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-darwin-amd64
sudo install minikube-darwin-amd64 /usr/local/bin/minikube

# 启动本地K8s集群
minikube start

# 验证安装
kubectl get nodes
kubectl get pods -A
```

#### 1.2 部署测试应用
```bash
# 部署一个简单的Web应用
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --port=80 --type=NodePort

# 查看部署结果
kubectl get pods
kubectl get services
```

#### 1.3 理解基本概念
```bash
# 查看Pod详细信息
kubectl describe pod <pod-name>

# 查看Pod日志
kubectl logs <pod-name>

# 查看集群事件
kubectl get events
```

### 阶段2：理解我们的代码（1天）

#### 2.1 运行测试程序
```bash
# 确保配置正确
cat configs/config.yaml

# 运行测试程序
make test-k8s
```

#### 2.2 修改代码观察变化

**实验1：修改监控的namespace**
```go
// configs/config.yaml
k8s:
  watch_namespaces: "default,kube-system"  # 添加kube-system
```

**实验2：添加自定义日志**
```go
// internal/k8s/client.go
func (c *Client) GetClusterInfo() (map[string]interface{}, error) {
    fmt.Println("DEBUG: 正在获取集群信息...")

    // ... 原有代码

    fmt.Printf("DEBUG: 发现%d个节点\n", len(nodes.Items))
    return info, nil
}
```

**实验3：添加新的监控指标**
```go
// internal/k8s/client.go 添加新函数
func (c *Client) GetDeploymentCount(namespace string) (int, error) {
    deployments, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return 0, err
    }
    return len(deployments.Items), nil
}
```

### 阶段3：深入理解监控原理（2-3天）

#### 3.1 理解Watch机制
```bash
# 在一个终端中运行
kubectl get pods -w

# 在另一个终端中创建Pod
kubectl run busybox --image=busybox -- sleep 3600

# 观察第一个终端的输出
```

#### 3.2 创建自己的监控器
```go
// 创建一个新的文件：internal/k8s/custom_watcher.go
package k8s

import (
    "context"
    "fmt"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/watch"
)

type CustomWatcher struct {
    client *Client
}

func (cw *CustomWatcher) WatchDeployments(ctx context.Context, namespace string) {
    watcher, err := cw.client.clientset.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{})
    if err != nil {
        fmt.Printf("Failed to watch deployments: %v\n", err)
        return
    }

    for event := range watcher.ResultChan() {
        switch event.Type {
        case watch.Added:
            fmt.Printf("🆕 新建Deployment\n")
        case watch.Modified:
            fmt.Printf("🔄 更新Deployment\n")
        case watch.Deleted:
            fmt.Printf("🗑️ 删除Deployment\n")
        }
    }
}
```

### 阶段4：网络分析实践（2天）

#### 4.1 创建网络策略实验
```yaml
# 创建网络策略
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: nginx
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: busybox
```

#### 4.2 测试网络分析功能
```bash
# 部署测试Pod
kubectl run nginx --image=nginx --labels="app=nginx"
kubectl run busybox --image=busybox --labels="app=busybox" -- sleep 3600

# 运行我们的测试程序
make test-k8s
```

### 阶段5：扩展代码功能（3-5天）

#### 5.1 添加新的分析功能
```go
// internal/k8s/analyzer.go 添加新功能
func (c *Client) AnalyzeResourceUsage(namespace string) (map[string]interface{}, error) {
    // 获取资源使用情况
    pods, err := c.GetPods(namespace)
    if err != nil {
        return nil, err
    }

    analysis := make(map[string]interface{})

    // 分析CPU和内存使用
    // TODO: 实现资源使用分析

    return analysis, nil
}
```

#### 5.2 创建Web界面
```go
// cmd/web/main.go
func main() {
    // 创建简单的Web服务器
    http.HandleFunc("/cluster-info", func(w http.ResponseWriter, r *http.Request) {
        // 调用我们的K8s客户端
        info := k8sClient.GetClusterInfo()
        json.NewEncoder(w).Encode(info)
    })

    http.ListenAndServe(":8080", nil)
}
```

## 学习检查点

### 1. 基础理解检查
- [ ] 能解释什么是Kubernetes
- [ ] 能解释Pod、Service、Namespace的概念
- [ ] 能使用kubectl的基本命令
- [ ] 能部署简单的应用

### 2. 代码理解检查
- [ ] 能解释main函数的执行流程
- [ ] 能解释K8s客户端的创建过程
- [ ] 能理解Watch机制的工作原理
- [ ] 能解释数据转换的目的

### 3. 实践能力检查
- [ ] 能运行测试程序
- [ ] 能修改配置并观察结果
- [ ] 能添加简单的监控功能
- [ ] 能理解网络分析的输出

### 4. 扩展能力检查
- [ ] 能添加新的API调用
- [ ] 能创建自定义的分析逻辑
- [ ] 能理解错误处理机制
- [ ] 能优化代码性能

## 常见问题解答

### Q1: 为什么需要这么多的代码结构？
A1: 好的代码结构让程序：
- 更容易维护（每个文件负责一件事）
- 更容易测试（可以单独测试每个模块）
- 更容易扩展（添加新功能不影响现有代码）

### Q2: 什么是interface？为什么需要它？
A2: Interface就像"合同"：
- 定义了"必须做什么"
- 不关心"怎么做"
- 让不同的组件可以协作

### Q3: Watch和Polling的区别？
A3:
- Polling（轮询）：不停的问"有变化吗？"
- Watch（监听）：K8s主动告诉你"有变化了！"
- Watch更高效，更实时

### Q4: 为什么要做数据转换？
A4:
- K8s的对象很复杂，包含很多我们不需要的信息
- 转换后数据更简洁，更容易处理
- 隔离了K8s API变化的影响

## 学习资源

### 推荐教程
1. [Kubernetes官方文档](https://kubernetes.io/docs/)
2. [Kubernetes By Example](https://kubernetesbyexample.com/)
3. [Go语言官方教程](https://tour.golang.org/)

### 推荐工具
1. [Minikube](https://minikube.sigs.k8s.io/) - 本地K8s环境
2. [Lens](https://k8slens.dev/) - K8s图形化管理工具
3. [kubectl](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands) - K8s命令行工具

通过这样的学习路径，你会逐步掌握K8s监控的核心概念和我们的代码实现！