# 代码流程图解

## 整体数据流

```
K8s集群 → 我们的监控程序 → 分析结果
    ↓              ↓            ↓
┌─────────┐    ┌──────────┐   ┌─────────┐
│ 资源变化 │───▶│ 数据收集 │──▶│ 智能分析 │
└─────────┘    └──────────┘   └─────────┘
    │              │              │
    ▼              ▼              ▼
Pod创建/删除    转换为模型    LLM分析
Service更新    存储在内存    生成建议
Events事件      触发处理     自然语言输出
```

## 核心代码执行流程

### 1. 程序启动（`main.go`）
```go
func main() {
    // 1. 加载配置
    cfg, err := config.Load(configPath)

    // 2. 创建K8s客户端
    k8sClient, err := k8s.NewClient(&cfg.K8s)

    // 3. 测试连接
    err = k8sClient.TestConnection()

    // 4. 开始监控
    k8sClient.WatchResources(ctx, handler)
}
```

### 2. K8s客户端初始化（`client.go`）
```go
func NewClient(cfg *config.K8sConfig) (*Client, error) {
    // 1. 创建K8s配置
    config, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)

    // 2. 创建clientset
    clientset, err := kubernetes.NewForConfig(config)

    // 3. 返回客户端
    return &Client{
        clientset:  clientset,
        config:     cfg,
        namespaces: namespaces,
    }, nil
}
```

### 3. 监控启动（`watcher.go`）
```go
func (w *Watcher) Start(ctx context.Context) error {
    // 为每个namespace启动监控
    for _, namespace := range w.client.namespaces {
        go w.watchNamespace(ctx, namespace)
    }
}
```

## 理解关键概念

### 1. 什么是"Watching"？
Watching就像"看电视直播"：
- 传统方式：每隔一段时间去问"有什么变化吗？"
- Watching方式：K8s主动告诉我们"有变化了！"

```go
// 这就是Watching的核心
watcher, err := w.client.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
for event := range watcher.ResultChan() {
    // 有变化了！处理这个事件
    switch event.Type {
    case watch.Added:   // 新建了Pod
    case watch.Modified: // Pod有变化
    case watch.Deleted:  // Pod被删除
    }
}
```

### 2. 什么是"EventHandler"？
EventHandler是一个"事件处理器"接口：
```go
type EventHandler interface {
    OnPodUpdate(pod *models.PodInfo)      // 当Pod变化时调用
    OnServiceUpdate(service *models.ServiceInfo) // 当Service变化时调用
    OnEvent(event *models.EventInfo)     // 当有事件时调用
}
```

### 3. 数据转换的意义
K8s的对象很复杂，我们的模型很简单：

```go
// K8s的Pod对象有几十个字段
type corev1.Pod struct {
    // 大量复杂的字段...
}

// 我们的Pod模型只关心关键信息
type PodInfo struct {
    Name      string            `json:"name"`
    Namespace string            `json:"namespace"`
    Status    string            `json:"status"`
    // 只包含我们需要的字段
}
```

## 学习路径建议

### 1. 先理解概念
1. 什么是Kubernetes？
2. 什么是Pod、Service、Namespace？
3. 什么是Watch机制？

### 2. 再看代码结构
1. 从`main.go`开始，看程序如何启动
2. 看`client.go`，理解如何连接K8s
3. 看`watcher.go`，理解如何监控变化

### 3. 最后深入细节
1. 看数据转换`converter.go`
2. 看网络分析`network.go`
3. 理解事件处理机制

### 4. 动手实践
1. 运行测试程序
2. 修改配置，观察变化
3. 添加自己的监控逻辑