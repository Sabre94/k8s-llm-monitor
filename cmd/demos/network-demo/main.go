package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
)

func main() {
	fmt.Println("🔍 Pod通信检查演示")
	fmt.Println("==================================================")

	// 1. 初始化配置和客户端
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("❌ 配置加载失败: %v", err)
	}

	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("❌ 客户端创建失败: %v", err)
	}

	// 2. 获取当前运行的Pods
	fmt.Println("📦 获取当前Pod列表...")
	pods, err := k8sClient.GetPods("default")
	if err != nil {
		log.Fatalf("❌ 获取Pod失败: %v", err)
	}

	fmt.Printf("✅ 找到 %d 个Pods:\n", len(pods))
	for i, pod := range pods {
		fmt.Printf("   %d. %s (状态: %s)\n", i+1, pod.Name, pod.Status)
	}

	// 3. 选择两个Pod进行通信检查
	if len(pods) < 2 {
		fmt.Println("❌ 需要至少2个Pod进行通信检查")
		return
	}

	podA := fmt.Sprintf("%s/%s", pods[0].Namespace, pods[0].Name)
	podB := fmt.Sprintf("%s/%s", pods[1].Namespace, pods[1].Name)

	fmt.Printf("\n🔍 分析 %s 和 %s 之间的通信...\n", podA, podB)

	// 4. 创建网络分析器
	networkAnalyzer := k8s.NewNetworkAnalyzer(k8sClient)

	// 5. 执行通信分析
	analysis, err := networkAnalyzer.AnalyzePodCommunication(context.Background(), podA, podB)
	if err != nil {
		log.Fatalf("❌ 通信分析失败: %v", err)
	}

	// 6. 显示分析结果
	fmt.Println("\n📊 通信分析结果:")
	fmt.Printf("   Pod A: %s\n", analysis.PodA)
	fmt.Printf("   Pod B: %s\n", analysis.PodB)
	fmt.Printf("   状态: %s\n", analysis.Status)
	fmt.Printf("   置信度: %.2f\n", analysis.Confidence)
	fmt.Printf("   发现问题: %d个\n", len(analysis.Issues))

	if len(analysis.Issues) > 0 {
		fmt.Println("   问题描述:")
		for i, issue := range analysis.Issues {
			fmt.Printf("     %d. %s\n", i+1, issue)
		}
	} else {
		fmt.Println("   ✅ 未发现明显问题")
	}

	fmt.Printf("   建议方案: %d个\n", len(analysis.Solutions))
	for i, solution := range analysis.Solutions {
		fmt.Printf("     %d. %s\n", i+1, solution)
	}

	// 7. 详细展示检查过程
	fmt.Println("\n🔧 详细检查过程:")
	demonstrateCheckProcess(k8sClient, pods[0], pods[1])
}

// demonstrateCheckProcess 详细展示检查过程
func demonstrateCheckProcess(k8sClient *k8s.Client, podA, podB *models.PodInfo) {
	fmt.Println("   📋 第1步: 检查Pod基本状态")
	fmt.Printf("      Pod A (%s): %s\n", podA.Name, podA.Status)
	fmt.Printf("      Pod B (%s): %s\n", podB.Name, podB.Status)

	if podA.Status == "Running" && podB.Status == "Running" {
		fmt.Println("      ✅ 两个Pod都处于运行状态")
	} else {
		fmt.Println("      ❌ Pod状态异常，可能影响通信")
	}

	fmt.Println("\n   📋 第2步: 检查Pod所在节点")
	fmt.Printf("      Pod A 所在节点: %s\n", podA.NodeName)
	fmt.Printf("      Pod B 所在节点: %s\n", podB.NodeName)

	if podA.NodeName == podB.NodeName {
		fmt.Println("      ✅ 两个Pod在同一节点，通信效率较高")
	} else {
		fmt.Println("      ⚠️  两个Pod在不同节点，需要跨节点通信")
	}

	fmt.Println("\n   📋 第3步: 检查Pod IP地址")
	fmt.Printf("      Pod A IP: %s\n", podA.IP)
	fmt.Printf("      Pod B IP: %s\n", podB.IP)

	if podA.IP != "" && podB.IP != "" {
		fmt.Println("      ✅ 两个Pod都有IP地址")
	} else {
		fmt.Println("      ❌ Pod IP地址缺失，无法通信")
	}

	fmt.Println("\n   📋 第4步: 检查Pod标签")
	fmt.Printf("      Pod A 标签: %v\n", podA.Labels)
	fmt.Printf("      Pod B 标签: %v\n", podB.Labels)

	// 检查是否有匹配的Service
	fmt.Println("\n   📋 第5步: 检查Service覆盖")
	services, err := k8sClient.GetServices("default")
	if err != nil {
		fmt.Printf("      ❌ 获取Service失败: %v\n", err)
		return
	}

	podAServiceCount := 0
	podBServiceCount := 0

	for _, svc := range services {
		if doesServiceTargetPod(svc, podA) {
			podAServiceCount++
		}
		if doesServiceTargetPod(svc, podB) {
			podBServiceCount++
		}
	}

	fmt.Printf("      Pod A 被 %d 个Service覆盖\n", podAServiceCount)
	fmt.Printf("      Pod B 被 %d 个Service覆盖\n", podBServiceCount)

	if podBServiceCount > 0 {
		fmt.Println("      ✅ Pod B被Service覆盖，可以通过Service访问")
	} else {
		fmt.Println("      ⚠️  Pod B没有被Service覆盖，只能直接通过IP访问")
	}

	// 检查网络策略
	fmt.Println("\n   📋 第6步: 检查网络策略")
	fmt.Println("      ⚠️  网络策略检查功能尚未完全实现")
	fmt.Println("      📝 建议手动检查Network Policy配置")

	// 检查DNS
	fmt.Println("\n   📋 第7步: 检查DNS服务")
	coreDNSPods, err := k8sClient.GetPods("kube-system")
	if err != nil {
		fmt.Printf("      ❌ 获取CoreDNS失败: %v\n", err)
		return
	}

	coreDNSCount := 0
	for _, pod := range coreDNSPods {
		if contains(pod.Name, "coredns") && pod.Status == "Running" {
			coreDNSCount++
		}
	}

	fmt.Printf("      发现 %d 个运行的CoreDNS Pod\n", coreDNSCount)

	if coreDNSCount > 0 {
		fmt.Println("      ✅ DNS服务正常运行")
	} else {
		fmt.Println("      ❌ DNS服务异常，可能影响域名解析")
	}

	fmt.Println("\n   📋 第8步: 生成最终评估")
	fmt.Println("      🎯 基于以上检查，生成通信分析结果")
}

// 辅助函数
func doesServiceTargetPod(svc *models.ServiceInfo, pod *models.PodInfo) bool {
	for key, value := range svc.Selector {
		if podValue, exists := pod.Labels[key]; exists && podValue == value {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
