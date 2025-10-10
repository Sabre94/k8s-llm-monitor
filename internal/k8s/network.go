package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkAnalyzer 网络分析器
type NetworkAnalyzer struct {
	client    *Client
	logger    *logrus.Logger
	rttTester *RTTTester
	enableRTT bool
}

// NewNetworkAnalyzer 创建网络分析器
func NewNetworkAnalyzer(client *Client) *NetworkAnalyzer {
	return &NetworkAnalyzer{
		client:    client,
		logger:    client.logger,
		rttTester: NewRTTTester(client),
		enableRTT: true, // 默认启用RTT测试
	}
}

// AnalyzePodCommunication 分析Pod间通信
func (na *NetworkAnalyzer) AnalyzePodCommunication(ctx context.Context, podA, podB string) (*models.CommunicationAnalysis, error) {
	// 解析Pod名称和namespace
	podANamespace, podAName := parsePodName(podA)
	podBNamespace, podBName := parsePodName(podB)

	// 获取Pod信息
	podAInfo, err := na.getPodInfo(ctx, podANamespace, podAName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod A info: %w", err)
	}

	podBInfo, err := na.getPodInfo(ctx, podBNamespace, podBName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod B info: %w", err)
	}

	// 分析通信情况
	analysis := &models.CommunicationAnalysis{
		PodA:       podA,
		PodB:       podB,
		Status:     "unknown",
		Issues:     []string{},
		Solutions:  []string{},
		Confidence: 0.0,
	}

	// 检查Pod状态
	na.checkPodStatus(podAInfo, analysis)
	na.checkPodStatus(podBInfo, analysis)

	// 检查网络策略
	na.checkNetworkPolicies(ctx, podAInfo, podBInfo, analysis)

	// 检查服务发现
	na.checkServiceConnectivity(ctx, podAInfo, podBInfo, analysis)

	// 检查DNS配置
	na.checkDNSConnectivity(ctx, podAInfo, podBInfo, analysis)

	// 执行RTT测试
	if na.enableRTT {
		na.checkRTTConnectivity(ctx, podA, podB, analysis)
	}

	// 确定最终状态
	na.determineFinalStatus(analysis)

	return analysis, nil
}

// parsePodName 解析Pod名称
func parsePodName(podRef string) (namespace, name string) {
	parts := strings.Split(podRef, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "default", parts[0]
}

// getPodInfo 获取Pod信息
func (na *NetworkAnalyzer) getPodInfo(ctx context.Context, namespace, name string) (*models.PodInfo, error) {
	pod, err := na.client.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return na.client.convertPodToModel(pod), nil
}

// checkPodStatus 检查Pod状态
func (na *NetworkAnalyzer) checkPodStatus(pod *models.PodInfo, analysis *models.CommunicationAnalysis) {
	if pod.Status != "Running" {
		analysis.Issues = append(analysis.Issues,
			fmt.Sprintf("Pod %s/%s is not running (status: %s)", pod.Namespace, pod.Name, pod.Status))
		analysis.Solutions = append(analysis.Solutions,
			fmt.Sprintf("Check Pod %s/%s logs and events for issues", pod.Namespace, pod.Name))
	}
}

// checkNetworkPolicies 检查网络策略
func (na *NetworkAnalyzer) checkNetworkPolicies(ctx context.Context, podA, podB *models.PodInfo, analysis *models.CommunicationAnalysis) {
	// 获取两个Pod所在namespace的网络策略
	policiesA, err := na.getNetworkPolicies(ctx, podA.Namespace)
	if err != nil {
		na.logger.Warnf("Failed to get network policies for namespace %s: %v", podA.Namespace, err)
		return
	}

	policiesB, err := na.getNetworkPolicies(ctx, podB.Namespace)
	if err != nil {
		na.logger.Warnf("Failed to get network policies for namespace %s: %v", podB.Namespace, err)
		return
	}

	// 检查网络策略是否阻止通信
	na.analyzeNetworkPolicies(podA, podB, append(policiesA, policiesB...), analysis)
}

// getNetworkPolicies 获取网络策略
func (na *NetworkAnalyzer) getNetworkPolicies(ctx context.Context, namespace string) ([]*models.NetworkPolicyInfo, error) {
	policies, err := na.client.clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var policyInfos []*models.NetworkPolicyInfo
	for _, policy := range policies.Items {
		policyInfo := na.convertNetworkPolicyToModel(&policy)
		policyInfos = append(policyInfos, policyInfo)
	}

	return policyInfos, nil
}

// convertNetworkPolicyToModel 转换网络策略模型
func (na *NetworkAnalyzer) convertNetworkPolicyToModel(policy *networkingv1.NetworkPolicy) *models.NetworkPolicyInfo {
	policyInfo := &models.NetworkPolicyInfo{
		Name:        policy.Name,
		Namespace:   policy.Namespace,
		PodSelector: policy.Spec.PodSelector.MatchLabels,
	}

	// 转换Ingress规则
	for _, ingress := range policy.Spec.Ingress {
		ingressRule := models.NetworkPolicyRule{}
		for _, port := range ingress.Ports {
			ingressRule.Ports = append(ingressRule.Ports, models.PortRule{
				Protocol: string(*port.Protocol),
				Port:     port.Port.IntVal,
			})
		}
		policyInfo.Ingress = append(policyInfo.Ingress, ingressRule)
	}

	// 转换Egress规则
	for _, egress := range policy.Spec.Egress {
		egressRule := models.NetworkPolicyRule{}
		for _, port := range egress.Ports {
			egressRule.Ports = append(egressRule.Ports, models.PortRule{
				Protocol: string(*port.Protocol),
				Port:     port.Port.IntVal,
			})
		}
		policyInfo.Egress = append(policyInfo.Egress, egressRule)
	}

	return policyInfo
}

// analyzeNetworkPolicies 分析网络策略
func (na *NetworkAnalyzer) analyzeNetworkPolicies(podA, podB *models.PodInfo, policies []*models.NetworkPolicyInfo, analysis *models.CommunicationAnalysis) {
	// 简化的网络策略检查
	// 实际实现需要更复杂的逻辑来检查策略是否阻止通信

	for _, policy := range policies {
		if na.doesPolicyAffectPod(policy, podA) || na.doesPolicyAffectPod(policy, podB) {
			analysis.Issues = append(analysis.Issues,
				fmt.Sprintf("Network policy %s/%s may affect communication", policy.Namespace, policy.Name))
			analysis.Solutions = append(analysis.Solutions,
				fmt.Sprintf("Review network policy %s/%s rules", policy.Namespace, policy.Name))
		}
	}
}

// doesPolicyAffectPod 检查策略是否影响Pod
func (na *NetworkAnalyzer) doesPolicyAffectPod(policy *models.NetworkPolicyInfo, pod *models.PodInfo) bool {
	// 简化的匹配逻辑
	// 实际实现需要更复杂的标签匹配
	for key, value := range policy.PodSelector {
		if podValue, exists := pod.Labels[key]; exists && podValue == value {
			return true
		}
	}
	return false
}

// checkServiceConnectivity 检查服务连通性
func (na *NetworkAnalyzer) checkServiceConnectivity(ctx context.Context, podA, podB *models.PodInfo, analysis *models.CommunicationAnalysis) {
	// 检查Pod B是否通过Service暴露
	services, err := na.client.GetServices(podB.Namespace)
	if err != nil {
		na.logger.Warnf("Failed to get services for namespace %s: %v", podB.Namespace, err)
		return
	}

	// 查找是否有Service指向Pod B
	var targetService *models.ServiceInfo
	for _, svc := range services {
		if na.doesServiceTargetPod(svc, podB) {
			targetService = svc
			break
		}
	}

	if targetService == nil {
		analysis.Issues = append(analysis.Issues,
			fmt.Sprintf("No service found targeting Pod %s/%s", podB.Namespace, podB.Name))
		analysis.Solutions = append(analysis.Solutions,
			fmt.Sprintf("Create a service to expose Pod %s/%s", podB.Namespace, podB.Name))
	}
}

// doesServiceTargetPod 检查Service是否指向Pod
func (na *NetworkAnalyzer) doesServiceTargetPod(svc *models.ServiceInfo, pod *models.PodInfo) bool {
	for key, value := range svc.Selector {
		if podValue, exists := pod.Labels[key]; exists && podValue == value {
			return true
		}
	}
	return false
}

// checkDNSConnectivity 检查DNS连通性
func (na *NetworkAnalyzer) checkDNSConnectivity(ctx context.Context, podA, podB *models.PodInfo, analysis *models.CommunicationAnalysis) {
	// 检查CoreDNS状态
	coreDNSPods, err := na.client.GetPods("kube-system")
	if err != nil {
		na.logger.Warnf("Failed to get CoreDNS pods: %v", err)
		return
	}

	var coreDNSRunning bool
	for _, pod := range coreDNSPods {
		if strings.Contains(pod.Name, "coredns") && pod.Status == "Running" {
			coreDNSRunning = true
			break
		}
	}

	if !coreDNSRunning {
		analysis.Issues = append(analysis.Issues, "CoreDNS is not running properly")
		analysis.Solutions = append(analysis.Solutions, "Check CoreDNS pods in kube-system namespace")
	}
}

// checkRTTConnectivity 检查RTT连通性
func (na *NetworkAnalyzer) checkRTTConnectivity(ctx context.Context, podA, podB string, analysis *models.CommunicationAnalysis) {
	// 执行RTT测试
	result, err := na.rttTester.TestPodConnectivity(ctx, podA, podB)
	if err != nil {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("RTT测试失败: %v", err))
		analysis.Solutions = append(analysis.Solutions, "检查Pod是否支持网络命令执行")
		return
	}

	// 分析测试结果
	if result.SuccessRate < 50 {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("网络连通性差，成功率仅为%.1f%%", result.SuccessRate))
		analysis.Solutions = append(analysis.Solutions, "检查网络策略和防火墙配置")
	} else if result.SuccessRate < 100 {
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("网络存在丢包，成功率为%.1f%%", result.SuccessRate))
		analysis.Solutions = append(analysis.Solutions, "检查网络质量和节点状态")
	}

	// 评估延迟
	switch result.Latency {
	case "excellent", "good":
		// 延迟良好，无需特别说明
	case "fair":
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("网络延迟一般，平均RTT为%.2fms", result.AverageRTT))
		analysis.Solutions = append(analysis.Solutions, "考虑优化网络配置或检查网络负载")
	case "poor", "very_poor":
		analysis.Issues = append(analysis.Issues, fmt.Sprintf("网络延迟较高，平均RTT为%.2fms", result.AverageRTT))
		analysis.Solutions = append(analysis.Solutions, "检查网络配置和节点间网络连接")
	}

	// 记录测试详情
	na.logger.Infof("RTT测试结果: %s -> %s, 成功率: %.1f%%, 平均延迟: %.2fms, 延迟评级: %s",
		podA, podB, result.SuccessRate, result.AverageRTT, result.Latency)
}

// determineFinalStatus 确定最终状态
func (na *NetworkAnalyzer) determineFinalStatus(analysis *models.CommunicationAnalysis) {
	if len(analysis.Issues) == 0 {
		analysis.Status = "connected"
		analysis.Confidence = 0.9
		analysis.Solutions = append(analysis.Solutions, "No obvious issues detected")
	} else {
		analysis.Status = "disconnected"
		analysis.Confidence = 0.7
	}
}
