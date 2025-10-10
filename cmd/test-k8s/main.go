package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
)

// TestEventHandler 测试用的事件处理器
type TestEventHandler struct {
	podCount     int
	serviceCount int
	eventCount   int
}

func (h *TestEventHandler) OnPodUpdate(pod *models.PodInfo) {
	h.podCount++
	fmt.Printf("📦 Pod Update: %s/%s (Status: %s)\n", pod.Namespace, pod.Name, pod.Status)
}

func (h *TestEventHandler) OnServiceUpdate(service *models.ServiceInfo) {
	h.serviceCount++
	fmt.Printf("🔗 Service Update: %s/%s (Type: %s)\n", service.Namespace, service.Name, service.Type)
}

func (h *TestEventHandler) OnEvent(event *models.EventInfo) {
	h.eventCount++
	fmt.Printf("📋 Event: %s - %s (%s)\n", event.Reason, event.Message, event.Type)
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "config file path")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("🚀 Testing K8s connection...")

	// 创建K8s客户端
	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("Failed to create K8s client: %v", err)
	}

	// 测试连接
	fmt.Println("🔌 Testing K8s connection...")
	if err := k8sClient.TestConnection(); err != nil {
		log.Fatalf("Failed to connect to K8s: %v", err)
	}

	// 获取集群信息
	fmt.Println("📊 Getting cluster info...")
	clusterInfo, err := k8sClient.GetClusterInfo()
	if err != nil {
		log.Fatalf("Failed to get cluster info: %v", err)
	}

	fmt.Printf("✅ Cluster Info:\n")
	fmt.Printf("   Version: %s\n", clusterInfo["version"])
	fmt.Printf("   Nodes: %d\n", clusterInfo["nodes"])
	fmt.Printf("   Pods: %d\n", clusterInfo["pods"])
	fmt.Printf("   Namespaces: %v\n", clusterInfo["namespaces"])

	// 获取Pod列表
	fmt.Println("\n📦 Getting pods...")
	for _, ns := range k8sClient.Namespaces() {
		pods, err := k8sClient.GetPods(ns)
		if err != nil {
			fmt.Printf("❌ Failed to get pods in %s: %v\n", ns, err)
			continue
		}
		fmt.Printf("   Namespace %s: %d pods\n", ns, len(pods))

		// 显示前几个Pod的信息
		for i := 0; i < min(3, len(pods)); i++ {
			pod := pods[i]
			fmt.Printf("     - %s (Status: %s, Node: %s)\n", pod.Name, pod.Status, pod.NodeName)
		}
	}

	// 获取服务列表
	fmt.Println("\n🔗 Getting services...")
	for _, ns := range k8sClient.Namespaces() {
		services, err := k8sClient.GetServices(ns)
		if err != nil {
			fmt.Printf("❌ Failed to get services in %s: %v\n", ns, err)
			continue
		}
		fmt.Printf("   Namespace %s: %d services\n", ns, len(services))

		// 显示服务信息
		for _, svc := range services {
			fmt.Printf("     - %s (Type: %s, ClusterIP: %s)\n", svc.Name, svc.Type, svc.ClusterIP)
		}
	}

	// 获取最近事件
	fmt.Println("\n📋 Getting recent events...")
	for _, ns := range k8sClient.Namespaces() {
		events, err := k8sClient.GetEvents(ns, 10)
		if err != nil {
			fmt.Printf("❌ Failed to get events in %s: %v\n", ns, err)
			continue
		}
		fmt.Printf("   Namespace %s: %d recent events\n", ns, len(events))

		// 显示重要事件
		for _, event := range events {
			if event.Type == "Warning" || event.Type == "Error" {
				fmt.Printf("     - %s: %s (%s)\n", event.Reason, event.Message, event.Type)
			}
		}
	}

	// 测试网络分析
	fmt.Println("\n🔍 Testing network analysis...")
	networkAnalyzer := k8s.NewNetworkAnalyzer(k8sClient)

	// 查找两个Pod进行测试
	var podA, podB string
	for _, ns := range k8sClient.Namespaces() {
		pods, _ := k8sClient.GetPods(ns)
		if len(pods) >= 2 {
			podA = fmt.Sprintf("%s/%s", pods[0].Namespace, pods[0].Name)
			podB = fmt.Sprintf("%s/%s", pods[1].Namespace, pods[1].Name)
			break
		}
	}

	if podA != "" && podB != "" {
		fmt.Printf("🔍 Analyzing communication between %s and %s...\n", podA, podB)
		analysis, err := networkAnalyzer.AnalyzePodCommunication(context.Background(), podA, podB)
		if err != nil {
			fmt.Printf("❌ Failed to analyze communication: %v\n", err)
		} else {
			fmt.Printf("✅ Communication Analysis:\n")
			fmt.Printf("   Status: %s\n", analysis.Status)
			fmt.Printf("   Confidence: %.2f\n", analysis.Confidence)
			fmt.Printf("   Issues: %d\n", len(analysis.Issues))
			for _, issue := range analysis.Issues {
				fmt.Printf("     - %s\n", issue)
			}
			fmt.Printf("   Solutions: %d\n", len(analysis.Solutions))
			for _, solution := range analysis.Solutions {
				fmt.Printf("     - %s\n", solution)
			}
		}
	}

	// 测试监控功能
	fmt.Println("\n👀 Testing monitoring (running for 10 seconds)...")
	handler := &TestEventHandler{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := k8sClient.WatchResources(ctx, handler); err != nil {
		fmt.Printf("❌ Failed to start monitoring: %v\n", err)
	} else {
		fmt.Printf("✅ Monitoring Results:\n")
		fmt.Printf("   Pod updates: %d\n", handler.podCount)
		fmt.Printf("   Service updates: %d\n", handler.serviceCount)
		fmt.Printf("   Events: %d\n", handler.eventCount)
	}

	fmt.Println("\n✅ K8s connection test completed successfully!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
