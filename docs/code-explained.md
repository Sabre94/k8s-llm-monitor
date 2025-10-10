# 代码逐行解释

## 1. 主程序入口 (cmd/test-k8s/main.go)

```go
func main() {
    // 第1步：解析命令行参数
    var configPath string
    flag.StringVar(&configPath, "config", "./configs/config.yaml", "config file path")
    flag.Parse()

    // 第2步：加载配置文件
    cfg, err := config.Load(configPath)
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 第3步：创建K8s客户端
    k8sClient, err := k8s.NewClient(&cfg.K8s)
    if err != nil {
        log.Fatalf("Failed to create K8s client: %v", err)
    }

    // 第4步：测试连接
    if err := k8sClient.TestConnection(); err != nil {
        log.Fatalf("Failed to connect to K8s: %v", err)
    }

    // 第5步：获取集群信息
    clusterInfo, err := k8sClient.GetClusterInfo()
    if err != nil {
        log.Fatalf("Failed to get cluster info: %v", err)
    }

    // 第6步：显示信息
    fmt.Printf("✅ Cluster Info:\n")
    fmt.Printf("   Version: %s\n", clusterInfo["version"])
    fmt.Printf("   Nodes: %d\n", clusterInfo["nodes"])
    fmt.Printf("   Pods: %d\n", clusterInfo["pods"])
}
```

**解释**：
1. 读取配置文件（告诉程序如何连接K8s）
2. 创建K8s客户端（建立与K8s的连接）
3. 测试连接是否成功
4. 获取并显示集群基本信息

## 2. K8s客户端详解 (internal/k8s/client.go)

### 最关键的部分：NewClient函数

```go
func NewClient(cfg *config.K8sConfig) (*Client, error) {
    var config *rest.Config
    var err error

    // 这里是连接K8s的核心逻辑
    if cfg.Kubeconfig != "" {
        // 开发环境：使用kubeconfig文件
        config, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
    } else {
        // 生产环境：使用集群内部配置
        config, err = rest.InClusterConfig()
    }

    if err != nil {
        return nil, fmt.Errorf("failed to create k8s config: %w", err)
    }

    // 创建K8s客户端
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create clientset: %w", err)
    }

    return &Client{
        clientset:  clientset,
        config:     cfg,
        namespaces: namespaces,
    }, nil
}
```

**通俗解释**：
- `kubeconfig`：像是K8s的"身份证文件"
- `InClusterConfig`：在K8s集群内部运行时，自动获取身份
- `clientset`：K8s API的"遥控器"，可以控制K8s

### 获取集群信息

```go
func (c *Client) GetClusterInfo() (map[string]interface{}, error) {
    // 获取K8s版本
    version, err := c.clientset.Discovery().ServerVersion()

    // 获取节点列表
    nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})

    // 统计Pod数量
    for _, ns := range c.namespaces {
        pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
        podCount += len(pods.Items)
    }

    return info, nil
}
```

**解释**：
1. 问K8s："你是什么版本的？"
2. 问K8s："有多少个节点？"
3. 问K8s："有多少个Pod？"

## 3. 监控系统详解 (internal/k8s/watcher.go)

### 核心概念：Watch机制

```go
func (w *Watcher) doWatchPods(ctx context.Context, namespace string) {
    // 创建一个"监控器"
    watcher, err := w.client.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})

    // 持续监听变化
    for {
        select {
        case event, ok := <-watcher.ResultChan():
            if !ok {
                // 连接断开，重新连接
                return
            }

            switch event.Type {
            case watch.Added:   // 新建了Pod
            case watch.Modified: // Pod有变化
            case watch.Deleted:  // Pod被删除
            }

            // 调用事件处理器
            pod, _ := event.Object.(*corev1.Pod)
            podInfo := w.client.convertPodToModel(pod)
            w.handler.OnPodUpdate(podInfo)
        }
    }
}
```

**生活比喻**：
- `Watch`：就像在门口装了个"门铃"
- `ResultChan`：就是门铃的"声音"
- `event.Type`：告诉你"有人来了"还是"有人走了"
- `handler.OnPodUpdate`：通知"管理员"来处理

## 4. 事件处理器 (cmd/test-k8s/main.go)

```go
type TestEventHandler struct {
    podCount     int
    serviceCount int
    eventCount   int
}

func (h *TestEventHandler) OnPodUpdate(pod *models.PodInfo) {
    h.podCount++
    fmt.Printf("📦 Pod Update: %s/%s (Status: %s)\n", pod.Namespace, pod.Name, pod.Status)
}
```

**解释**：
- 这是一个简单的"计数器"
- 每次有Pod变化时，计数器+1
- 并打印变化的信息

## 5. 网络分析详解 (internal/k8s/network.go)

```go
func (na *NetworkAnalyzer) AnalyzePodCommunication(ctx context.Context, podA, podB string) (*models.CommunicationAnalysis, error) {
    // 1. 获取两个Pod的信息
    podAInfo, err := na.getPodInfo(ctx, podANamespace, podAName)
    podBInfo, err := na.getPodInfo(ctx, podBNamespace, podBName)

    // 2. 检查Pod状态
    na.checkPodStatus(podAInfo, analysis)
    na.checkPodStatus(podBInfo, analysis)

    // 3. 检查网络策略
    na.checkNetworkPolicies(ctx, podAInfo, podBInfo, analysis)

    // 4. 检查服务发现
    na.checkServiceConnectivity(ctx, podAInfo, podBInfo, analysis)

    // 5. 检查DNS
    na.checkDNSConnectivity(ctx, podAInfo, podBInfo, analysis)

    return analysis, nil
}
```

**生活比喻**：
就像检查两个人是否能正常通信：
1. 确认两个人都在家（Pod状态）
2. 检查门锁是否开对（网络策略）
3. 检查地址是否正确（服务发现）
4. 检查电话是否畅通（DNS）

## 6. 数据转换 (internal/k8s/converter.go)

```go
func (c *Client) convertPodToModel(pod *corev1.Pod) *models.PodInfo {
    podInfo := &models.PodInfo{
        Name:      pod.Name,
        Namespace: pod.Namespace,
        Status:    string(pod.Status.Phase),
        NodeName:  pod.Spec.NodeName,
        IP:        pod.Status.PodIP,
    }

    // 处理容器信息
    for _, container := range pod.Spec.Containers {
        containerInfo := models.ContainerInfo{
            Name:  container.Name,
            Image: container.Image,
            State: getContainerState(containerStatus),
        }
        podInfo.Containers = append(podInfo.Containers, containerInfo)
    }

    return podInfo
}
```

**解释**：
- K8s的Pod对象很复杂，有几十个字段
- 我们只提取关心的信息：名称、状态、IP、容器等
- 这样简化后的数据更容易处理和分析

## 学习建议

### 1. 运行程序看输出
```bash
make test-k8s
```

### 2. 修改配置观察变化
```yaml
# configs/config.yaml
k8s:
  watch_namespaces: "default"  # 改成其他namespace试试
```

### 3. 添加自己的日志
```go
// 在代码中添加日志
fmt.Printf("DEBUG: 正在获取Pod列表...\n")
```

### 4. 理解数据结构
```go
// 打印Pod的完整信息
fmt.Printf("Pod详细信息: %+v\n", pod)
```

通过这样的方式，你会逐步理解代码的工作原理！