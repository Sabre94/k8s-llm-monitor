package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
)

// LiveMonitorHandler 实时监控处理器
type LiveMonitorHandler struct {
	startTime time.Time
}

func (h *LiveMonitorHandler) OnPodUpdate(pod *models.PodInfo) {
	elapsed := time.Since(h.startTime)
	fmt.Printf("📦 [%s] Pod变化: %s/%s\n",
		elapsed.Round(time.Second), pod.Namespace, pod.Name)
	fmt.Printf("   状态: %s → %s\n", pod.Status, pod.Status)
	fmt.Printf("   节点: %s\n", pod.NodeName)
	fmt.Printf("   容器数: %d\n", len(pod.Containers))

	// 显示容器状态
	for _, container := range pod.Containers {
		fmt.Printf("   容器: %s (%s)\n", container.Name, container.State)
	}
	fmt.Println("   ---")
}

func (h *LiveMonitorHandler) OnServiceUpdate(service *models.ServiceInfo) {
	elapsed := time.Since(h.startTime)
	fmt.Printf("🔗 [%s] Service变化: %s/%s\n",
		elapsed.Round(time.Second), service.Namespace, service.Name)
	fmt.Printf("   类型: %s\n", service.Type)
	fmt.Printf("   集群IP: %s\n", service.ClusterIP)
	fmt.Printf("   端口数: %d\n", len(service.Ports))
	fmt.Println("   ---")
}

func (h *LiveMonitorHandler) OnEvent(event *models.EventInfo) {
	elapsed := time.Since(h.startTime)
	fmt.Printf("📋 [%s] 集群事件: %s\n",
		elapsed.Round(time.Second), event.Type)
	fmt.Printf("   原因: %s\n", event.Reason)
	fmt.Printf("   消息: %s\n", event.Message)
	fmt.Printf("   来源: %s\n", event.Source)
	if event.Count > 1 {
		fmt.Printf("   次数: %d\n", event.Count)
	}
	fmt.Println("   ---")
}

func main() {
	fmt.Println("🔥 K8s 实时监控启动")
	fmt.Println("================================================")
	fmt.Println("💡 在另一个终端运行以下命令来测试:")
	fmt.Println("   kubectl run test-xxx --image=nginx:alpine")
	fmt.Println("   kubectl delete pod <pod-name>")
	fmt.Println("   kubectl expose deployment xxx --port=80")
	fmt.Println("================================================")

	// 加载配置
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("❌ 配置加载失败: %v", err)
	}

	// 创建K8s客户端
	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("❌ 客户端创建失败: %v", err)
	}

	// 测试连接
	if err := k8sClient.TestConnection(); err != nil {
		log.Fatalf("❌ 连接测试失败: %v", err)
	}

	fmt.Printf("✅ K8s连接成功\n")
	fmt.Printf("📊 当前集群状态:\n")

	// 显示当前状态
	clusterInfo, _ := k8sClient.GetClusterInfo()
	fmt.Printf("   版本: %s\n", clusterInfo["version"])
	fmt.Printf("   节点: %d个\n", clusterInfo["nodes"])
	fmt.Printf("   Pod: %d个\n", clusterInfo["pods"])
	fmt.Printf("   监控命名空间: %v\n", clusterInfo["namespaces"])
	fmt.Println()

	// 显示当前Pods
	fmt.Printf("📦 当前Pod列表:\n")
	for _, namespace := range k8sClient.Namespaces() {
		pods, _ := k8sClient.GetPods(namespace)
		fmt.Printf("   %s: %d个Pods\n", namespace, len(pods))
		for _, pod := range pods {
			fmt.Printf("     - %s (%s)\n", pod.Name, pod.Status)
		}
	}
	fmt.Println()

	// 创建处理器
	handler := &LiveMonitorHandler{
		startTime: time.Now(),
	}

	// 创建上下文，可以优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动监控
	fmt.Println("👀 开始实时监控... (按 Ctrl+C 退出)")
	fmt.Println("================================================")

	// 启动监控协程
	go func() {
		if err := k8sClient.WatchResources(ctx, handler); err != nil {
			fmt.Printf("❌ 监控出错: %v\n", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 定期显示统计信息
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 收到中断信号，正在关闭监控...")
			cancel()
			time.Sleep(2 * time.Second)
			fmt.Println("👋 监控已关闭")
			return

		case <-ticker.C:
			// 每30秒显示一次统计
			elapsed := time.Since(handler.startTime)
			fmt.Printf("⏱️  [%s] 监控运行中...\n", elapsed.Round(time.Second))

			// 显示当前状态
			for _, namespace := range k8sClient.Namespaces() {
				pods, _ := k8sClient.GetPods(namespace)
				fmt.Printf("   %s: %d个Pods\n", namespace, len(pods))
			}
			fmt.Println()
		}
	}
}
