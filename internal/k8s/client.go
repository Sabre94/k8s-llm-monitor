package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client K8s客户端封装
type Client struct {
	clientset  *kubernetes.Clientset
	config     *config.K8sConfig
	restConfig *rest.Config
	logger     *logrus.Logger
	namespaces []string
}

// NewClient 创建新的K8s客户端
func NewClient(cfg *config.K8sConfig) (*Client, error) {
	var restConfig *rest.Config
	var err error

	// 如果有kubeconfig文件，使用文件配置
	if cfg.Kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		// 否则使用in-cluster配置
		restConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	// 创建clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// 解析要监控的namespace
	namespaces := parseNamespaces(cfg.WatchNamespaces)

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Client{
		clientset:  clientset,
		config:     cfg,
		restConfig: restConfig,
		logger:     logger,
		namespaces: namespaces,
	}, nil
}

// parseNamespaces 解析namespace字符串
func parseNamespaces(namespacesStr string) []string {
	if namespacesStr == "" {
		return []string{"default"}
	}

	// 按逗号分割并去除空格
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
	// 尝试获取集群版本
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

	// 获取Pod数量
	podCount := 0
	for _, ns := range c.namespaces {
		pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			c.logger.Warnf("Failed to list pods in namespace %s: %v", ns, err)
			continue
		}
		podCount += len(pods.Items)
	}

	info := map[string]interface{}{
		"version":    version.String(),
		"nodes":      len(nodes.Items),
		"pods":       podCount,
		"namespaces": c.namespaces,
	}

	return info, nil
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

// GetServices 获取指定namespace的Service列表
func (c *Client) GetServices(namespace string) ([]*models.ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var serviceInfos []*models.ServiceInfo
	for _, svc := range services.Items {
		serviceInfo := c.convertServiceToModel(&svc)
		serviceInfos = append(serviceInfos, serviceInfo)
	}

	return serviceInfos, nil
}

// GetEvents 获取指定namespace的事件
func (c *Client) GetEvents(namespace string, limit int64) ([]*models.EventInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var eventInfos []*models.EventInfo
	for _, event := range events.Items {
		eventInfo := c.convertEventToModel(&event)
		eventInfos = append(eventInfos, eventInfo)
	}

	return eventInfos, nil
}

// GetPodLogs 获取Pod日志
func (c *Client) GetPodLogs(namespace, podName string, lines int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &lines,
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	buf := make([]byte, 1024)
	var result string
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			result += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return result, nil
}

// Namespaces 返回监控的namespace列表
func (c *Client) Namespaces() []string {
	return c.namespaces
}

// WatchCRDs 监控CRD和自定义资源
func (c *Client) WatchCRDs(ctx context.Context, handler EventHandler) error {
	crdWatcher, err := NewCRDWatcher(c, handler)
	if err != nil {
		return fmt.Errorf("failed to create CRD watcher: %w", err)
	}
	return crdWatcher.Start(ctx)
}
