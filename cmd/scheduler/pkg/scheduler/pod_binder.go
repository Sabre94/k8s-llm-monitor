package scheduler

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
)

// K8sPodBinder Kubernetes Pod绑定器
type K8sPodBinder struct {
	config    *K8sConfig
	clientset *kubernetes.Clientset
	logger    *logrus.Logger
}

// NewK8sPodBinder 创建K8s Pod绑定器
func NewK8sPodBinder(config *K8sConfig) (*K8sPodBinder, error) {
	// 创建Kubernetes客户端配置
	var restConfig *rest.Config
	var err error

	if config.InCluster {
		restConfig, err = rest.InClusterConfig()
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	// 创建clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	binder := &K8sPodBinder{
		config:    config,
		clientset: clientset,
		logger:    Log().WithField("binder", "k8s").Logger,
	}

	return binder, nil
}

// BindPod 绑定Pod到节点
func (b *K8sPodBinder) BindPod(ctx context.Context, podName, nodeName string) error {
	binding := &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: b.config.Namespace,
		},
		Target: v1.ObjectReference{
			APIVersion: "v1",
			Kind:      "Node",
			Name:      nodeName,
		},
	}

	b.logger.WithFields(logrus.Fields{
		"pod_name":  podName,
		"node_name": nodeName,
	}).Info("Binding pod to node")

	// 创建绑定
	err := b.clientset.CoreV1().Pods(b.config.Namespace).Bind(ctx, binding, metav1.CreateOptions{})
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"pod_name":  podName,
			"node_name": nodeName,
			"error":     err.Error(),
		}).Error("Failed to bind pod to node")
		return fmt.Errorf("failed to bind pod %s to node %s: %w", podName, nodeName, err)
	}

	b.logger.WithFields(logrus.Fields{
		"pod_name":  podName,
		"node_name": nodeName,
	}).Info("Successfully bound pod to node")

	return nil
}

// BindPodGroup 绑定Pod组到节点集合
func (b *K8sPodBinder) BindPodGroup(ctx context.Context, bindings []PodBinding) error {
	b.logger.WithField("pod_count", len(bindings)).Info("Binding pod group")

	var errors []string
	successCount := 0

	for _, binding := range bindings {
		// 验证Pod是否存在
		pod, err := b.clientset.CoreV1().Pods(binding.PodNamespace).Get(ctx, binding.PodName, metav1.GetOptions{})
		if err != nil {
			errMsg := fmt.Sprintf("Pod %s/%s not found: %v", binding.PodNamespace, binding.PodName, err)
			errors = append(errors, errMsg)
			b.logger.WithFields(logrus.Fields{
				"pod_name":   binding.PodName,
				"namespace":  binding.PodNamespace,
				"error":      err.Error(),
			}).Error("Pod not found")
			continue
		}

		// 检查Pod状态
		if pod.Spec.NodeName != "" {
			b.logger.WithFields(logrus.Fields{
				"pod_name":   binding.PodName,
				"namespace":  binding.PodNamespace,
				"node_name":  pod.Spec.NodeName,
			}).Warn("Pod already assigned to node")
			successCount++
			continue
		}

		// 执行绑定
		if err := b.BindPod(ctx, binding.PodName, binding.NodeName); err != nil {
			errMsg := fmt.Sprintf("Failed to bind pod %s/%s to node %s: %v",
				binding.PodNamespace, binding.PodName, binding.NodeName, err)
			errors = append(errors, errMsg)
			continue
		}

		successCount++
	}

	b.logger.WithFields(logrus.Fields{
		"total_pods":    len(bindings),
		"successful":    successCount,
		"failed":        len(errors),
	}).Info("Pod group binding completed")

	if len(errors) > 0 {
		return fmt.Errorf("binding completed with %d errors: %s", len(errors), strings.Join(errors, "; "))
	}

	return nil
}

// BindNodeSet 绑定任务到节点集合（核心功能 - K8s实现）
func (b *K8sPodBinder) BindNodeSet(ctx context.Context, podName, podNamespace string, nodeSet []string, optResult *OptimizeResult) error {
	b.logger.WithFields(logrus.Fields{
		"pod_name":       podName,
		"namespace":      podNamespace,
		"node_set":       nodeSet,
		"node_count":     len(nodeSet),
		"coverage_ratio": optResult.CoverageRatio,
		"algorithm":      optResult.AlgorithmName,
	}).Info("Binding mission to node set")

	// 获取任务Pod
	pod, err := b.clientset.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get mission pod %s/%s: %w", podNamespace, podName, err)
	}

	// 更新Pod annotations（记录分配结果）
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations["uav-assigned-nodes"] = strings.Join(nodeSet, ",")
	pod.Annotations["uav-node-count"] = fmt.Sprintf("%d", len(nodeSet))
	pod.Annotations["uav-algorithm"] = optResult.AlgorithmName
	pod.Annotations["uav-coverage-ratio"] = fmt.Sprintf("%.3f", optResult.CoverageRatio)
	pod.Annotations["uav-binding-time"] = time.Now().Format(time.RFC3339)
	pod.Annotations["uav-optimization-score"] = fmt.Sprintf("%.2f", optResult.Score)

	// 添加优化结果详情
	if optResult.Objectives != nil {
		pod.Annotations["uav-avg-battery"] = fmt.Sprintf("%.1f", optResult.Objectives["avg_battery"])
		pod.Annotations["uav-avg-latency"] = fmt.Sprintf("%.1f", optResult.Objectives["avg_latency"])
		pod.Annotations["uav-avg-utilization"] = fmt.Sprintf("%.1f", optResult.Objectives["avg_utilization"])
	}

	// 更新Pod
	_, err = b.clientset.CoreV1().Pods(podNamespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update mission pod annotations: %w", err)
	}

	// 为每个节点创建对应的任务Pod（推荐）
	for i, nodeName := range nodeSet {
		taskPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-uav-task-%d", podName, i+1),
				Namespace: podNamespace,
				Labels: map[string]string{
					"mission-pod":    podName,
					"uav-node":       nodeName,
					"task-type":      "uav-task",
					"algorithm":      optResult.AlgorithmName,
				},
				Annotations: map[string]string{
					"parent-mission": podName,
					"assigned-node":  nodeName,
					"coverage-ratio": fmt.Sprintf("%.3f", optResult.CoverageRatio),
				},
			},
			Spec: v1.PodSpec{
				NodeName: nodeName,
				Containers: []v1.Container{
					{
						Name:  "uav-task",
						Image: "busybox:latest",
						Command: []string{"sleep", "3600"}, // 示例任务
						Env: []v1.EnvVar{
							{
								Name:  "MISSION_ID",
								Value: podName,
							},
							{
								Name:  "NODE_NAME",
								Value: nodeName,
							},
							{
								Name:  "COVERAGE_RATIO",
								Value: fmt.Sprintf("%.3f", optResult.CoverageRatio),
							},
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		}

		// 创建任务Pod
		_, err = b.clientset.CoreV1().Pods(podNamespace).Create(ctx, taskPod, metav1.CreateOptions{})
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"task_pod": taskPod.Name,
				"node_name": nodeName,
				"error": err.Error(),
			}).Error("Failed to create task pod")
			continue
		}

		b.logger.WithFields(logrus.Fields{
			"task_pod": taskPod.Name,
			"node_name": nodeName,
		}).Info("Created task pod on assigned node")
	}

	b.logger.WithFields(logrus.Fields{
		"mission_pod":   podName,
		"assigned_nodes": nodeSet,
	}).Info("Mission binding completed successfully")

	return nil
}

// MockPodBinder 模拟Pod绑定器（用于测试）
type MockPodBinder struct {
	bindings map[string]PodBinding
	logger   *logrus.Logger
}

// NewMockPodBinder 创建模拟Pod绑定器
func NewMockPodBinder() *MockPodBinder {
	return &MockPodBinder{
		bindings: make(map[string]PodBinding),
		logger:   Log().WithField("binder", "mock").Logger,
	}
}

// BindPod 绑定Pod到节点（模拟）
func (m *MockPodBinder) BindPod(ctx context.Context, podName, nodeName string) error {
	key := fmt.Sprintf("%s:%s", podName, nodeName)
	binding := PodBinding{
		PodName:  podName,
		NodeName: nodeName,
		Reason:   "mock_binding",
	}

	m.bindings[key] = binding

	m.logger.WithFields(logrus.Fields{
		"pod_name":  podName,
		"node_name": nodeName,
	}).Info("Mock binding pod to node")

	return nil
}

// BindPodGroup 绑定Pod组到节点集合（模拟）
func (m *MockPodBinder) BindPodGroup(ctx context.Context, bindings []PodBinding) error {
	m.logger.WithField("pod_count", len(bindings)).Info("Mock binding pod group")

	for _, binding := range bindings {
		key := fmt.Sprintf("%s:%s", binding.PodName, binding.NodeName)
		m.bindings[key] = binding

		m.logger.WithFields(logrus.Fields{
			"pod_name":   binding.PodName,
			"namespace":  binding.PodNamespace,
			"node_name":  binding.NodeName,
			"reason":     binding.Reason,
		}).Debug("Mock bound pod to node")
	}

	return nil
}

// BindNodeSet 绑定任务到节点集合（核心功能 - 模拟实现）
func (m *MockPodBinder) BindNodeSet(ctx context.Context, podName, podNamespace string, nodeSet []string, optResult *OptimizeResult) error {
	m.logger.WithFields(logrus.Fields{
		"pod_name":     podName,
		"namespace":    podNamespace,
		"node_set":     nodeSet,
		"node_count":   len(nodeSet),
		"coverage_ratio": optResult.CoverageRatio,
		"algorithm":     optResult.AlgorithmName,
	}).Info("Mock binding mission to node set")

	// 存储绑定信息
	for _, nodeName := range nodeSet {
		key := fmt.Sprintf("mission:%s:%s", podName, nodeName)
		binding := PodBinding{
			PodName:      podName,
			PodNamespace: podNamespace,
			NodeName:     nodeName,
			Reason:       fmt.Sprintf("NSGA-II optimization, coverage: %.2f", optResult.CoverageRatio),
		}
		m.bindings[key] = binding
	}

	return nil
}

// GetBindings 获取所有绑定（仅用于测试）
func (m *MockPodBinder) GetBindings() map[string]PodBinding {
	return m.bindings
}

// ClearBindings 清除所有绑定（仅用于测试）
func (m *MockPodBinder) ClearBindings() {
	m.bindings = make(map[string]PodBinding)
}

// NodeSelector 节点选择器
type NodeSelector struct {
	selectedNodes []string
	logger        *logrus.Logger
}

// NewNodeSelector 创建节点选择器
func NewNodeSelector(selectedNodes []string) *NodeSelector {
	return &NodeSelector{
		selectedNodes: selectedNodes,
		logger:        Log().WithField("component", "node-selector").Logger,
	}
}

// SelectBestNode 选择最优节点
func (ns *NodeSelector) SelectBestNode(podName string, requirements map[string]interface{}) (string, error) {
	if len(ns.selectedNodes) == 0 {
		return "", ErrNoAvailableNodes
	}

	// 简单策略：返回第一个可用节点
	// 实际可以根据Pod需求和节点状态进行更复杂的选择
	selectedNode := ns.selectedNodes[0]

	ns.logger.WithFields(logrus.Fields{
		"pod_name":     podName,
		"selected_node": selectedNode,
		"total_nodes":  len(ns.selectedNodes),
	}).Debug("Selected best node for pod")

	return selectedNode, nil
}

// SelectNodesForPodGroup 为Pod组选择节点
func (ns *NodeSelector) SelectNodesForPodGroup(pods []PodRef) ([]PodBinding, error) {
	if len(ns.selectedNodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	if len(pods) > len(ns.selectedNodes) {
		return nil, fmt.Errorf("not enough nodes available: requested %d, available %d",
			len(pods), len(ns.selectedNodes))
	}

	var bindings []PodBinding

	// 简单策略：按顺序分配节点
	for i, pod := range pods {
		if i >= len(ns.selectedNodes) {
			break
		}

		binding := PodBinding{
			PodName:      pod.Name,
			PodNamespace: pod.Namespace,
			NodeName:     ns.selectedNodes[i],
			Reason:       "uav_scheduling",
		}

		bindings = append(bindings, binding)

		ns.logger.WithFields(logrus.Fields{
			"pod_name":    pod.Name,
			"namespace":   pod.Namespace,
			"assigned_node": ns.selectedNodes[i],
			"priority":    pod.Priority,
		}).Debug("Assigned node to pod")
	}

	return bindings, nil
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	nodeLoads map[string]*NodeLoad
	logger    *logrus.Logger
}

// NodeLoad 节点负载
type NodeLoad struct {
	NodeName     string
	PodCount     int
	CPUUsage     float64
	MemoryUsage  float64
	LastUpdate   time.Time
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		nodeLoads: make(map[string]*NodeLoad),
		logger:    Log().WithField("component", "load-balancer").Logger,
	}
}

// UpdateNodeLoad 更新节点负载
func (lb *LoadBalancer) UpdateNodeLoad(nodeName string, podCount int, cpuUsage, memoryUsage float64) {
	load := &NodeLoad{
		NodeName:    nodeName,
		PodCount:    podCount,
		CPUUsage:    cpuUsage,
		MemoryUsage: memoryUsage,
		LastUpdate:  time.Now(),
	}

	lb.nodeLoads[nodeName] = load

	lb.logger.WithFields(logrus.Fields{
		"node_name":   nodeName,
		"pod_count":   podCount,
		"cpu_usage":   cpuUsage,
		"memory_usage": memoryUsage,
	}).Debug("Updated node load")
}

// SelectLeastLoadedNode 选择负载最小的节点
func (lb *LoadBalancer) SelectLeastLoadedNode(availableNodes []string) (string, error) {
	if len(availableNodes) == 0 {
		return "", ErrNoAvailableNodes
	}

	var bestNode string
	var minLoad float64 = math.Inf(1)

	for _, nodeName := range availableNodes {
		load, exists := lb.nodeLoads[nodeName]
		if !exists {
			// 如果没有负载信息，认为负载为0
			bestNode = nodeName
			break
		}

		// 计算综合负载（Pod数量 + 资源使用率）
		totalLoad := float64(load.PodCount) + load.CPUUsage + load.MemoryUsage

		if totalLoad < minLoad {
			minLoad = totalLoad
			bestNode = nodeName
		}
	}

	if bestNode == "" {
		return "", ErrNoAvailableNodes
	}

	lb.logger.WithFields(logrus.Fields{
		"selected_node": bestNode,
		"load":          minLoad,
		"available_nodes": len(availableNodes),
	}).Debug("Selected least loaded node")

	return bestNode, nil
}

// GetNodeLoad 获取节点负载
func (lb *LoadBalancer) GetNodeLoad(nodeName string) (*NodeLoad, bool) {
	load, exists := lb.nodeLoads[nodeName]
	return load, exists
}

// GetAllNodeLoads 获取所有节点负载
func (lb *LoadBalancer) GetAllNodeLoads() map[string]*NodeLoad {
	// 返回副本
	loads := make(map[string]*NodeLoad)
	for k, v := range lb.nodeLoads {
		loads[k] = v
	}
	return loads
}

// ClearExpiredLoads 清除过期的负载信息
func (lb *LoadBalancer) ClearExpiredLoads(maxAge time.Duration) {
	now := time.Now()
	for nodeName, load := range lb.nodeLoads {
		if now.Sub(load.LastUpdate) > maxAge {
			delete(lb.nodeLoads, nodeName)
			lb.logger.WithField("node_name", nodeName).Debug("Cleared expired load data")
		}
	}
}