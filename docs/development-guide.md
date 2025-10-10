# 从零开始构建K8s监控系统 - 完整开发指南

## 🎯 开发思路：从需求到代码

### 第一步：明确需求
我们要做一个"基于LLM的K8s智能监控系统"，核心功能：
1. 连接K8s集群
2. 实时监控资源变化
3. 分析Pod间通信
4. 提供智能诊断

### 第二步：技术选型
- 语言：Go（K8s官方客户端库支持最好）
- 主要库：client-go（K8s客户端）、gin（Web框架）
- 架构：分层架构（API层、业务层、数据层）

## 📁 文件1：项目初始化 (go.mod)

**为什么第一个写这个？**
- 任何Go项目都需要模块管理
- 定义项目名称和依赖关系

**创建文件：**
```bash
mkdir k8s-llm-monitor
cd k8s-llm-monitor
go mod init github.com/yourusername/k8s-llm-monitor
```

**文件内容：**
```go
module github.com/yourusername/k8s-llm-monitor

go 1.21

require (
	k8s.io/client-go v0.34.1
	github.com/gin-gonic/gin v1.11.0
	github.com/sashabaranov/go-openai v1.41.2
	github.com/spf13/viper v1.21.0
	github.com/sirupsen/logrus v1.9.3
)
```

## 📁 文件2：目录结构创建

**为什么第二个做这个？**
- 好的项目结构让代码清晰易懂
- 遵循Go项目的标准布局

**创建目录：**
```bash
mkdir -p cmd/{server,agent} internal/{k8s,llm,processor,api,config,storage} pkg/{utils,models} configs deployments scripts web
```

**目录结构说明：**
```
k8s-llm-monitor/
├── cmd/                    # 应用入口点
│   ├── server/            # 主服务程序
│   └── agent/             # 代理程序
├── internal/              # 内部包（不对外暴露）
│   ├── k8s/               # K8s相关功能
│   ├── llm/               # LLM相关功能
│   ├── processor/         # 数据处理
│   ├── api/               # API接口
│   ├── config/            # 配置管理
│   └── storage/           # 数据存储
├── pkg/                   # 公共包（可以被外部引用）
│   ├── utils/             # 工具函数
│   └── models/            # 数据模型
├── configs/               # 配置文件
├── deployments/           # 部署文件
├── scripts/               # 构建脚本
└── web/                   # 前端资源
```

## 📁 文件3：数据模型设计 (pkg/models/models.go)

**为什么第三个写这个？**
- 数据模型是整个系统的基础
- 定义了系统中数据的"样子"
- 所有功能都围绕数据模型展开

**设计思路：**
1. 需要监控K8s的哪些资源？Pod、Service、Event等
2. 需要存储哪些信息？状态、配置、网络信息等
3. 需要输出什么格式？分析结果、诊断建议等

**文件内容：**
```go
package models

import (
	"time"
)

// PodInfo Pod信息 - 我们最关心的资源
type PodInfo struct {
	Name       string            `json:"name"`       // Pod名称
	Namespace  string            `json:"namespace"`  // 命名空间
	Status     string            `json:"status"`     // 运行状态
	NodeName   string            `json:"node_name"`  // 所在节点
	IP         string            `json:"ip"`         // Pod IP
	Labels     map[string]string `json:"labels"`     // 标签
	StartTime  time.Time         `json:"start_time"` // 启动时间
	Containers []ContainerInfo   `json:"containers"` // 容器信息
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Name  string            `json:"name"`  // 容器名称
	Image string            `json:"image"` // 镜像
	State string            `json:"state"` // 运行状态
	Ready bool              `json:"ready"` // 是否就绪
	Env   map[string]string `json:"env"`   // 环境变量
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name      string        `json:"name"`       // 服务名称
	Namespace string        `json:"namespace"`  // 命名空间
	Type      string        `json:"type"`       // 服务类型
	ClusterIP string        `json:"cluster_ip"` // 集群IP
	Ports     []ServicePort `json:"ports"`      // 端口信息
	Selector  map[string]string `json:"selector"` // 选择器
}

// EventInfo 事件信息
type EventInfo struct {
	Type      string    `json:"type"`       // 事件类型
	Reason    string    `json:"reason"`     // 事件原因
	Message   string    `json:"message"`    // 事件消息
	Source    string    `json:"source"`     // 事件来源
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	Count     int32     `json:"count"`      // 发生次数
}

// CommunicationAnalysis 通信分析结果 - 我们的核心功能
type CommunicationAnalysis struct {
	PodA       string   `json:"pod_a"`        // 第一个Pod
	PodB       string   `json:"pod_b"`        // 第二个Pod
	Status     string   `json:"status"`       // 通信状态
	Issues     []string `json:"issues"`       // 发现的问题
	Solutions  []string `json:"solutions"`    // 解决方案
	Confidence float64  `json:"confidence"`   // 置信度
}
```

## 📁 文件4：配置管理 (internal/config/config.go)

**为什么第四个写这个？**
- 任何应用都需要配置
- 配置让程序更灵活，可以适应不同环境

**设计思路：**
1. 需要配置哪些内容？K8s连接、LLM设置、存储等
2. 配置格式：YAML格式，易于阅读
3. 支持环境变量覆盖，便于部署

**文件内容：**
```go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	K8s        K8sConfig        `mapstructure:"k8s"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	Debug  bool   `mapstructure:"debug"`
}

// K8sConfig K8s配置
type K8sConfig struct {
	Kubeconfig      string `mapstructure:"kubeconfig"`
	Namespace       string `mapstructure:"namespace"`
	WatchNamespaces string `mapstructure:"watch_namespaces"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("k8s.namespace", "default")
	viper.SetDefault("k8s.watch_namespaces", "default")
	// ... 其他默认值
}
```

## 📁 文件5：配置文件 (configs/config.yaml)

**为什么紧接着创建配置文件？**
- 有了配置结构，就需要对应的配置文件
- 让程序可以立即运行测试

**文件内容：**
```yaml
# K8s LLM Monitor Configuration

server:
  host: "0.0.0.0"
  port: 8080
  debug: true

k8s:
  kubeconfig: "/Users/xiabin/.kube/config"  # 本地开发使用
  namespace: "default"
  watch_namespaces: "default,kube-system"

llm:
  provider: "openai"
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-4"
  max_tokens: 2000
  temperature: 0.1

storage:
  type: "memory"

monitoring:
  metrics_interval: 30
  event_retention: 168

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

## 📁 文件6：K8s客户端核心 (internal/k8s/client.go)

**为什么现在写这个？**
- 有了数据模型和配置，就可以开始实现核心功能了
- K8s客户端是连接集群的关键组件

**设计思路：**
1. 如何连接K8s？支持kubeconfig和in-cluster两种方式
2. 需要哪些基本功能？获取资源、测试连接等
3. 错误处理：网络问题、权限问题等

**文件内容：**
```go
package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client K8s客户端封装
type Client struct {
	clientset    *kubernetes.Clientset
	config       *config.K8sConfig
	logger       *logrus.Logger
	namespaces   []string
}

// NewClient 创建新的K8s客户端
func NewClient(cfg *config.K8sConfig) (*Client, error) {
	// 1. 创建K8s配置
	config, err := buildK8sConfig(cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	// 2. 创建clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// 3. 解析namespace
	namespaces := parseNamespaces(cfg.WatchNamespaces)

	return &Client{
		clientset:  clientset,
		config:     cfg,
		logger:     logrus.New(),
		namespaces: namespaces,
	}, nil
}

// buildK8sConfig 构建K8s配置
func buildK8sConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		// 开发环境：使用kubeconfig文件
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// 生产环境：使用in-cluster配置
		return rest.InClusterConfig()
	}
}

// parseNamespaces 解析namespace字符串
func parseNamespaces(namespacesStr string) []string {
	if namespacesStr == "" {
		return []string{"default"}
	}

	parts := strings.Split(namespacesStr, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return []string{"default"}
	}

	return result
}

// TestConnection 测试K8s连接
func (c *Client) TestConnection() error {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	c.logger.Infof("Connected to Kubernetes cluster: %s", version.String())
	return nil
}

// GetClusterInfo 获取集群基本信息
func (c *Client) GetClusterInfo() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取集群版本
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	// 获取节点信息
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// 统计Pod数量
	podCount := 0
	for _, ns := range c.namespaces {
		pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			c.logger.Warnf("Failed to list pods in namespace %s: %v", ns, err)
			continue
		}
		podCount += len(pods.Items)
	}

	return map[string]interface{}{
		"version":    version.String(),
		"nodes":      len(nodes.Items),
		"pods":       podCount,
		"namespaces": c.namespaces,
	}, nil
}

// GetPods 获取指定namespace的Pod列表
func (c *Client) GetPods(namespace string) ([]*models.PodInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var podInfos []*models.PodInfo
	for _, pod := range pods.Items {
		podInfo := c.convertPodToModel(&pod)
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// Namespaces 返回监控的namespace列表
func (c *Client) Namespaces() []string {
	return c.namespaces
}
```

## 📁 文件7：数据转换工具 (internal/k8s/converter.go)

**为什么需要这个文件？**
- K8s的API对象很复杂，包含很多我们不需要的字段
- 需要转换为我们的简单模型，便于处理

**文件内容：**
```go
package k8s

import (
	"time"

	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// convertPodToModel 将K8s Pod对象转换为我们的模型
func (c *Client) convertPodToModel(pod *corev1.Pod) *models.PodInfo {
	podInfo := &models.PodInfo{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
		NodeName:  pod.Spec.NodeName,
		IP:        pod.Status.PodIP,
		Labels:    pod.Labels,
		StartTime: getCreationTime(pod),
	}

	// 转换容器信息
	for _, container := range pod.Spec.Containers {
		containerStatus := getContainerStatus(pod.Status.ContainerStatuses, container.Name)

		containerInfo := models.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			State: getContainerState(containerStatus),
			Ready: containerStatus != nil && containerStatus.Ready,
			Env:   make(map[string]string),
		}

		// 提取环境变量（只提取非敏感的）
		for _, envVar := range container.Env {
			if envVar.Value != "" {
				containerInfo.Env[envVar.Name] = envVar.Value
			}
		}

		podInfo.Containers = append(podInfo.Containers, containerInfo)
	}

	return podInfo
}

// getContainerStatus 获取容器状态
func getContainerStatus(statuses []corev1.ContainerStatus, name string) *corev1.ContainerStatus {
	for _, status := range statuses {
		if status.Name == name {
			return &status
		}
	}
	return nil
}

// getContainerState 获取容器状态字符串
func getContainerState(status *corev1.ContainerStatus) string {
	if status == nil {
		return "Unknown"
	}

	if status.State.Running != nil {
		return "Running"
	}
	if status.State.Waiting != nil {
		return "Waiting"
	}
	if status.State.Terminated != nil {
		return "Terminated"
	}

	return "Unknown"
}

// getCreationTime 获取创建时间
func getCreationTime(obj metav1.Object) time.Time {
	if obj.GetCreationTimestamp().Time.IsZero() {
		return time.Time{}
	}
	return obj.GetCreationTimestamp().Time
}
```

## 📁 文件8：主程序入口 (cmd/server/main.go)

**为什么现在写main.go？**
- 有了核心功能，就可以编写主程序了
- main.go是程序的入口点，用来测试我们的功能

**文件内容：**
```go
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "config file path")
	flag.Parse()

	// 1. 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("🚀 Starting K8s LLM Monitor...")

	// 2. 创建K8s客户端
	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("Failed to create K8s client: %v", err)
	}

	// 3. 测试连接
	if err := k8sClient.TestConnection(); err != nil {
		log.Fatalf("Failed to connect to K8s: %v", err)
	}

	// 4. 获取集群信息
	clusterInfo, err := k8sClient.GetClusterInfo()
	if err != nil {
		log.Fatalf("Failed to get cluster info: %v", err)
	}

	// 5. 显示基本信息
	fmt.Printf("✅ Connected to cluster:\n")
	fmt.Printf("   Version: %s\n", clusterInfo["version"])
	fmt.Printf("   Nodes: %d\n", clusterInfo["nodes"])
	fmt.Printf("   Pods: %d\n", clusterInfo["pods"])
	fmt.Printf("   Namespaces: %v\n", clusterInfo["namespaces"])

	// 6. 显示Pod信息
	fmt.Println("\n📦 Pods:")
	for _, namespace := range k8sClient.Namespaces() {
		pods, err := k8sClient.GetPods(namespace)
		if err != nil {
			fmt.Printf("   ❌ Failed to get pods in %s: %v\n", namespace, err)
			continue
		}
		fmt.Printf("   %s: %d pods\n", namespace, len(pods))
		for _, pod := range pods {
			fmt.Printf("     - %s (%s)\n", pod.Name, pod.Status)
		}
	}

	fmt.Println("\n✅ Basic functionality test completed!")
}
```

## 📁 文件9：测试和验证

**为什么需要测试？**
- 验证我们的代码是否正常工作
- 及时发现和修复问题

**测试步骤：**
```bash
# 1. 安装依赖
go mod tidy

# 2. 运行程序
go run cmd/server/main.go

# 3. 预期输出
🚀 Starting K8s LLM Monitor...
✅ Connected to cluster:
   Version: v1.34.0
   Nodes: 1
   Pods: 10
   Namespaces: [default kube-system]

📦 Pods:
   default: 3 pods
     - nginx-7977cdf8f5-rncbs (Running)
     - test-pod (Running)
     - busybox-76df46d5c9-x4hb4 (Running)
   kube-system: 7 pods
     - coredns-66bc5c9577-rpv5k (Running)
     - etcd-minikube (Running)
     ...

✅ Basic functionality test completed!
```

## 📁 文件10：扩展功能 - 监控系统 (internal/k8s/watcher.go)

**为什么需要监控系统？**
- 静态获取信息还不够，需要实时监控变化
- Watch机制是K8s的核心特性之一

**设计思路：**
1. 如何监听变化？使用K8s的Watch API
2. 如何处理事件？定义事件处理器接口
3. 如何保证可靠性？自动重连机制

**文件内容：**
```go
package k8s

import (
	"context"
	"time"

	"github.com/yourusername/k8s-llm-monitor/pkg/models"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// EventHandler 事件处理器接口
type EventHandler interface {
	OnPodUpdate(pod *models.PodInfo)
	OnServiceUpdate(service *models.ServiceInfo)
	OnEvent(event *models.EventInfo)
}

// Watcher 资源监控器
type Watcher struct {
	client *Client
	handler EventHandler
	logger *logrus.Logger
	stopCh chan struct{}
}

// NewWatcher 创建新的监控器
func NewWatcher(client *Client, handler EventHandler) *Watcher {
	return &Watcher{
		client:  client,
		handler: handler,
		logger:  client.logger,
		stopCh:  make(chan struct{}),
	}
}

// Start 开始监控
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting K8s resource watcher")

	// 为每个namespace启动监控
	for _, namespace := range w.client.namespaces {
		go w.watchNamespace(ctx, namespace)
	}

	return nil
}

// watchNamespace 监控指定namespace
func (w *Watcher) watchNamespace(ctx context.Context, namespace string) {
	w.logger.Infof("Start watching namespace: %s", namespace)

	// 启动各种资源的监控
	go w.watchPods(ctx, namespace)
	go w.watchServices(ctx, namespace)
	go w.watchEvents(ctx, namespace)
}

// watchPods 监控Pod变化
func (w *Watcher) watchPods(ctx context.Context, namespace string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			w.doWatchPods(ctx, namespace)
			time.Sleep(5 * time.Second) // 重试间隔
		}
	}
}

// doWatchPods 执行Pod监控
func (w *Watcher) doWatchPods(ctx context.Context, namespace string) {
	watcher, err := w.client.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.logger.Errorf("Failed to watch pods in namespace %s: %v", namespace, err)
		return
	}
	defer watcher.Stop()

	w.logger.Infof("Watching pods in namespace: %s", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				w.logger.Warnf("Pod watcher channel closed for namespace: %s", namespace)
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					w.logger.Warnf("Received non-pod object in pod watcher")
					continue
				}

				podInfo := w.client.convertPodToModel(pod)
				w.handler.OnPodUpdate(podInfo)

				w.logger.Debugf("Pod %s/%s: %s", namespace, pod.Name, event.Type)
			}
		}
	}
}

// WatchResources 统一的监控接口
func (c *Client) WatchResources(ctx context.Context, handler EventHandler) error {
	watcher := NewWatcher(c, handler)
	return watcher.Start(ctx)
}
```

## 📁 开发顺序总结

### 阶段1：基础架构
1. **go.mod** - 项目初始化
2. **目录结构** - 代码组织
3. **pkg/models/models.go** - 数据模型设计
4. **internal/config/config.go** - 配置管理
5. **configs/config.yaml** - 配置文件

### 阶段2：核心功能
6. **internal/k8s/client.go** - K8s客户端核心
7. **internal/k8s/converter.go** - 数据转换
8. **cmd/server/main.go** - 主程序测试

### 阶段3：监控功能
9. **internal/k8s/watcher.go** - 实时监控
10. **测试程序** - 验证监控功能

### 阶段4：高级功能
11. **internal/k8s/network.go** - 网络分析
12. **internal/llm/** - LLM集成
13. **internal/api/** - API接口
14. **web/** - 前端界面

## 🎯 关键设计原则

### 1. **分层架构**
- API层：处理HTTP请求
- 业务层：实现核心逻辑
- 数据层：K8s客户端和数据存储

### 2. **接口设计**
- 定义清晰的接口
- 实现依赖注入
- 便于测试和扩展

### 3. **错误处理**
- 优雅的错误处理
- 详细的日志记录
- 自动重连机制

### 4. **配置管理**
- 支持多种环境
- 环境变量覆盖
- 默认值设置

通过这样的开发顺序，你可以逐步构建一个完整、可靠、可扩展的K8s监控系统！