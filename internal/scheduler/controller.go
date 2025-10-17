package scheduler

import (
	context "context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	uavMetricGVR = schema.GroupVersionResource{
		Group:    "monitoring.io",
		Version:  "v1",
		Resource: "uavmetrics",
	}

	schedulingRequestGVR = schema.GroupVersionResource{
		Group:    "scheduler.io",
		Version:  "v1",
		Resource: "schedulingrequests",
	}
)

// Controller 简单调度器控制器
type Controller struct {
	logger     *logrus.Logger
	dynamic    dynamic.Interface
	kubeClient *kubernetes.Clientset
	k8sClient  *k8s.Client
	interval   time.Duration
}

// Config 控制器配置
type Config struct {
	Interval time.Duration
}

// NewController 构造控制器
func NewController(dynamic dynamic.Interface, kubeClient *kubernetes.Clientset, k8sClient *k8s.Client, cfg Config) *Controller {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Second
	}

	return &Controller{
		logger:     logger,
		dynamic:    dynamic,
		kubeClient: kubeClient,
		k8sClient:  k8sClient,
		interval:   cfg.Interval,
	}
}

// Run 启动调度循环
func (c *Controller) Run(ctx context.Context) error {
	c.logger.Infof("Starting scheduler controller (interval: %s)", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		if err := c.reconcile(ctx); err != nil {
			c.logger.Errorf("Reconcile failed: %v", err)
		}

		select {
		case <-ctx.Done():
			c.logger.Info("Scheduler controller stopped")
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Controller) reconcile(ctx context.Context) error {
	requests, err := c.dynamic.Resource(schedulingRequestGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list scheduling requests failed: %w", err)
	}

	uavList, err := c.dynamic.Resource(uavMetricGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list UAV metrics failed: %w", err)
	}

	for _, item := range requests.Items {
		if err := c.processRequest(ctx, &item, uavList); err != nil {
			c.logger.Errorf("Process request %s/%s failed: %v", item.GetNamespace(), item.GetName(), err)
		}
	}

	return nil
}

func (c *Controller) processRequest(ctx context.Context, req *unstructured.Unstructured, uavList *unstructured.UnstructuredList) error {
	phase, found, err := unstructured.NestedString(req.Object, "status", "phase")
	if err != nil {
		return fmt.Errorf("read status.phase failed: %w", err)
	}
	if found && phase != "" && phase != "Pending" {
		return nil
	}

	spec, found, err := unstructured.NestedMap(req.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("request spec missing: %w", err)
	}

	var requestSpec models.SchedulingRequestSpec
	if workload, ok := spec["workload"].(map[string]interface{}); ok {
		requestSpec.Workload.Name, _ = workload["name"].(string)
		requestSpec.Workload.Namespace, _ = workload["namespace"].(string)
		requestSpec.Workload.Type, _ = workload["type"].(string)
	}

	if v, ok := spec["minBatteryPercent"].(float64); ok {
		requestSpec.MinBatteryPercent = v
	}

	if list, ok := spec["preferredNodes"].([]interface{}); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				requestSpec.PreferredNodes = append(requestSpec.PreferredNodes, s)
			}
		}
	}

	if requestSpec.Workload.Name == "" || requestSpec.Workload.Namespace == "" {
		return c.updateStatus(ctx, req, models.SchedulingRequestStatus{
			Phase:   "Failed",
			Message: "workload name/namespace 不能为空",
		})
	}

	candidates := c.buildCandidates(requestSpec, uavList)
	if len(candidates) == 0 {
		return c.updateStatus(ctx, req, models.SchedulingRequestStatus{
			Phase:   "Failed",
			Message: "无满足要求的 UAV 节点",
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	chosen := candidates[0]

	status := models.SchedulingRequestStatus{
		Phase:        "Assigned",
		AssignedNode: chosen.NodeName,
		AssignedUAV:  chosen.UAVID,
		Score:        chosen.Score,
		Message:      fmt.Sprintf("选中节点 %s (电量 %.1f%%)", chosen.NodeName, chosen.Battery),
	}

	return c.updateStatus(ctx, req, status)
}

func (c *Controller) buildCandidates(spec models.SchedulingRequestSpec, uavList *unstructured.UnstructuredList) []models.SchedulingCandidate {
	preferredSet := map[string]struct{}{}
	for _, node := range append([]string(nil), spec.PreferredNodes...) {
		preferredSet[strings.ToLower(node)] = struct{}{}
	}

	var candidates []models.SchedulingCandidate
	for _, item := range uavList.Items {
		uavSpec, _, _ := unstructured.NestedMap(item.Object, "spec")
		uavStatus, _, _ := unstructured.NestedMap(item.Object, "status")

		nodeName, _ := uavSpec["node_name"].(string)
		uavID, _ := uavSpec["uav_id"].(string)
		battery := readFloat(uavSpec, "battery", "remaining_percent")
		collectionStatus := strings.ToLower(readString(uavStatus, "collection_status"))

		if nodeName == "" {
			continue
		}

		if spec.MinBatteryPercent > 0 && battery < spec.MinBatteryPercent {
			continue
		}

		if collectionStatus != "" && collectionStatus != "active" {
			continue
		}

		heartbeatStr := readString(uavStatus, "last_update")
		heartbeat, _ := time.Parse(time.RFC3339, heartbeatStr)

		score := battery
		if _, ok := preferredSet[strings.ToLower(nodeName)]; ok {
			score += 10
		}

		candidate := models.SchedulingCandidate{
			NodeName:      nodeName,
			UAVID:         uavID,
			Battery:       battery,
			LastHeartbeat: heartbeat,
			Score:         score,
		}
		candidates = append(candidates, candidate)
	}

	return candidates
}

func (c *Controller) updateStatus(ctx context.Context, req *unstructured.Unstructured, status models.SchedulingRequestStatus) error {
	if status.LastUpdated == nil {
		now := time.Now().UTC()
		status.LastUpdated = &now
	}

	statusMap := map[string]interface{}{
		"phase":        status.Phase,
		"assignedNode": status.AssignedNode,
		"assignedUAV":  status.AssignedUAV,
		"score":        status.Score,
		"message":      status.Message,
		"lastUpdated":  status.LastUpdated.Format(time.RFC3339),
	}

	if status.Phase == "" {
		statusMap["phase"] = "Pending"
	}

	if err := unstructured.SetNestedMap(req.Object, statusMap, "status"); err != nil {
		return fmt.Errorf("set status failed: %w", err)
	}

	_, err := c.dynamic.Resource(schedulingRequestGVR).
		Namespace(req.GetNamespace()).
		UpdateStatus(ctx, req, metav1.UpdateOptions{})
	return err
}

func readFloat(m map[string]interface{}, fields ...string) float64 {
	current := interface{}(m)
	for _, field := range fields {
		if mp, ok := current.(map[string]interface{}); ok {
			current = mp[field]
		} else {
			return 0
		}
	}

	switch v := current.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func readString(m map[string]interface{}, fields ...string) string {
	current := interface{}(m)
	for _, field := range fields {
		if mp, ok := current.(map[string]interface{}); ok {
			current = mp[field]
		} else {
			return ""
		}
	}

	if s, ok := current.(string); ok {
		return s
	}
	return ""
}
