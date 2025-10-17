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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client K8s客户端封装
type Client struct {
	clientset  *kubernetes.Clientset
	dynamic    dynamic.Interface
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

	// 创建dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// 解析要监控的namespace
	namespaces := parseNamespaces(cfg.WatchNamespaces)

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Client{
		clientset:  clientset,
		dynamic:    dynamicClient,
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

// RESTConfig 返回底层的 REST 配置
func (c *Client) RESTConfig() (*rest.Config, error) {
	if c.restConfig == nil {
		return nil, fmt.Errorf("rest config not initialized")
	}
	return c.restConfig, nil
}

// ListUAVMetricsCRD 获取UAV指标CRD数据
func (c *Client) ListUAVMetricsCRD(ctx context.Context, namespace string) ([]*models.CustomResourceInfo, error) {
	if c.dynamic == nil {
		return nil, fmt.Errorf("dynamic client not initialized")
	}

	gvr := schema.GroupVersionResource{
		Group:    "monitoring.io",
		Version:  "v1",
		Resource: "uavmetrics",
	}

	resource := c.dynamic.Resource(gvr)

	var (
		list *unstructured.UnstructuredList
		err  error
	)

	if namespace == "" || namespace == metav1.NamespaceAll {
		list, err = resource.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = resource.Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list UAV metrics CRDs: %w", err)
	}

	customResources := make([]*models.CustomResourceInfo, 0, len(list.Items))
	for i := range list.Items {
		customResources = append(customResources, convertUnstructuredToModel(&list.Items[i], "monitoring.io", "UAVMetric"))
	}

	return customResources, nil
}

func convertUnstructuredToModel(obj *unstructured.Unstructured, group, kind string) *models.CustomResourceInfo {
	spec := map[string]interface{}{}
	if rawSpec, ok := obj.Object["spec"].(map[string]interface{}); ok {
		spec = rawSpec
	}

	status := map[string]interface{}{}
	if rawStatus, ok := obj.Object["status"].(map[string]interface{}); ok {
		status = rawStatus
	}

	return &models.CustomResourceInfo{
		Kind:         kind,
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		Group:        group,
		Version:      obj.GetAPIVersion(),
		Spec:         spec,
		Status:       status,
		Generation:   obj.GetGeneration(),
		CreationTime: obj.GetCreationTimestamp().Time,
		UpdateTime:   getLastUpdateTime(obj),
	}
}

// UpsertUAVMetric 创建或更新UAVMetric自定义资源
func (c *Client) UpsertUAVMetric(ctx context.Context, namespace string, report *models.UAVReport) error {
	if c.dynamic == nil {
		return fmt.Errorf("dynamic client not initialized")
	}

	if report == nil {
		return fmt.Errorf("uav report is nil")
	}

	if namespace == "" {
		namespace = c.config.Namespace
		if namespace == "" {
			namespace = "default"
		}
	}

	resourceName := fmt.Sprintf("uavmetric-%s", sanitizeResourceName(report.NodeName))
	if report.NodeName == "" {
		return fmt.Errorf("uav report missing node name")
	}

	reportTime := report.Timestamp
	if reportTime.IsZero() {
		reportTime = time.Now().UTC()
	}

	status := report.Status
	if status == "" {
		status = "active"
	}

	gvr := schema.GroupVersionResource{
		Group:    "monitoring.io",
		Version:  "v1",
		Resource: "uavmetrics",
	}

	resource := c.dynamic.Resource(gvr).Namespace(namespace)

	spec := map[string]interface{}{
		"node_name": report.NodeName,
		"uav_id":    report.UAVID,
	}

	if report.State != nil {
		state := report.State
		spec["gps"] = map[string]interface{}{
			"latitude":          state.GPS.Latitude,
			"longitude":         state.GPS.Longitude,
			"altitude":          state.GPS.Altitude,
			"relative_altitude": state.GPS.RelativeAltitude,
			"satellite_count":   state.GPS.SatelliteCount,
			"fix_type":          state.GPS.FixType,
		}
		spec["battery"] = map[string]interface{}{
			"voltage":            state.Battery.Voltage,
			"remaining_percent":  state.Battery.RemainingPercent,
			"remaining_capacity": state.Battery.RemainingCapacity,
			"temperature":        state.Battery.Temperature,
		}
		spec["flight"] = map[string]interface{}{
			"mode":           state.Flight.Mode,
			"armed":          state.Flight.Armed,
			"ground_speed":   state.Flight.GroundSpeed,
			"vertical_speed": state.Flight.VerticalSpeed,
		}
		spec["health"] = map[string]interface{}{
			"system_status": state.Health.SystemStatus,
			"error_count":   state.Health.ErrorCount,
			"warning_count": state.Health.WarningCount,
		}
	}

	statusPayload := map[string]interface{}{
		"last_update":       reportTime.UTC().Format(time.RFC3339),
		"collection_status": status,
	}

	labels := map[string]interface{}{
		"app":                     "uav-agent",
		"monitoring.io/component": "uav-metrics",
		"monitoring.io/node":      sanitizeResourceName(report.NodeName),
	}
	if report.UAVID != "" {
		labels["monitoring.io/uav-id"] = sanitizeResourceName(report.UAVID)
	}

	if report.NodeIP != "" {
		labels["monitoring.io/node-ip"] = report.NodeIP
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.io/v1",
			"kind":       "UAVMetric",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": namespace,
				"labels":    labels,
			},
			"spec":   spec,
			"status": statusPayload,
		},
	}

	existing, err := resource.Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, createErr := resource.Create(ctx, obj, metav1.CreateOptions{}); createErr != nil {
				return fmt.Errorf("failed to create UAVMetric %s: %w", resourceName, createErr)
			}
			return nil
		}
		return fmt.Errorf("failed to get UAVMetric %s: %w", resourceName, err)
	}

	existing.Object["spec"] = spec
	existing.Object["status"] = statusPayload

	if meta, ok := existing.Object["metadata"].(map[string]interface{}); ok {
		if existingLabels, ok := meta["labels"].(map[string]interface{}); ok {
			for key, value := range labels {
				existingLabels[key] = value
			}
		} else {
			meta["labels"] = labels
		}
	}

	if _, err = resource.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update UAVMetric %s: %w", resourceName, err)
	}

	return nil
}

func sanitizeResourceName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	return name
}

func getLastUpdateTime(obj *unstructured.Unstructured) time.Time {
	managed := obj.GetManagedFields()
	if len(managed) > 0 {
		if managed[0].Time != nil {
			return managed[0].Time.Time
		}
	}
	return obj.GetCreationTimestamp().Time
}

// WatchCRDs 监控CRD和自定义资源
func (c *Client) WatchCRDs(ctx context.Context, handler EventHandler) error {
	crdWatcher, err := NewCRDWatcher(c, handler)
	if err != nil {
		return fmt.Errorf("failed to create CRD watcher: %w", err)
	}
	return crdWatcher.Start(ctx)
}
