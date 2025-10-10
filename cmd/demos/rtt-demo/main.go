package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
)

func main() {
	fmt.Println("🚀 RTT测试演示")
	fmt.Println("=====================================")

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
		fmt.Printf("   %d. %s (状态: %s, IP: %s)\n", i+1, pod.Name, pod.Status, pod.IP)
	}

	// 3. 选择两个Pod进行RTT测试
	if len(pods) < 2 {
		fmt.Println("❌ 需要至少2个Pod进行RTT测试")
		return
	}

	podA := fmt.Sprintf("%s/%s", pods[0].Namespace, pods[0].Name)
	podB := fmt.Sprintf("%s/%s", pods[1].Namespace, pods[1].Name)

	fmt.Printf("\n🔍 测试 %s 和 %s 之间的RTT...\n", podA, podB)

	// 4. 创建RTT测试器
	rttTester := k8s.NewRTTTester(k8sClient)

	// 5. 执行RTT测试
	result, err := rttTester.TestPodConnectivity(context.Background(), podA, podB)
	if err != nil {
		log.Fatalf("❌ RTT测试失败: %v", err)
	}

	// 6. 显示测试结果
	fmt.Println("\n📊 RTT测试结果:")
	fmt.Printf("   Pod A: %s\n", result.PodA)
	fmt.Printf("   Pod B: %s\n", result.PodB)
	fmt.Printf("   测试次数: %d\n", result.TestCount)
	fmt.Printf("   成功率: %.1f%%\n", result.SuccessRate)
	fmt.Printf("   平均RTT: %.2f ms\n", result.AverageRTT)
	fmt.Printf("   延迟评级: %s\n", result.Latency)

	fmt.Println("\n📋 详细测试结果:")
	for i, rttResult := range result.RTTResults {
		fmt.Printf("   %d. 方法: %s\n", i+1, rttResult.Method)
		fmt.Printf("      成功: %v\n", rttResult.Success)
		if rttResult.Success {
			fmt.Printf("      RTT: %.2f ms\n", rttResult.RTT)
			fmt.Printf("      丢包率: %.1f%%\n", rttResult.PacketLoss)
		} else {
			fmt.Printf("      错误: %s\n", rttResult.ErrorMessage)
		}
		fmt.Printf("      时间: %s\n", rttResult.Timestamp.Format("15:04:05"))
	}

	// 7. 使用网络分析器进行完整分析
	fmt.Println("\n🔧 使用网络分析器进行完整分析...")
	networkAnalyzer := k8s.NewNetworkAnalyzer(k8sClient)
	analysis, err := networkAnalyzer.AnalyzePodCommunication(context.Background(), podA, podB)
	if err != nil {
		log.Fatalf("❌ 通信分析失败: %v", err)
	}

	fmt.Println("\n📊 完整通信分析结果:")
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

	fmt.Println("\n✅ RTT测试演示完成！")
}
