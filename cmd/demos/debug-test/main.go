package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
)

// DebugEventHandler 调试用的事件处理器
type DebugEventHandler struct {
	debug bool
}

func (h *DebugEventHandler) OnPodUpdate(pod *models.PodInfo) {
	if h.debug {
		fmt.Printf("🔍 [DEBUG] Pod更新事件:\n")
		fmt.Printf("   名称: %s/%s\n", pod.Namespace, pod.Name)
		fmt.Printf("   状态: %s\n", pod.Status)
		fmt.Printf("   节点: %s\n", pod.NodeName)
		fmt.Printf("   容器数: %d\n", len(pod.Containers))
		fmt.Printf("   时间: %s\n", time.Now().Format("15:04:05"))
		fmt.Println("   ---")
	}
}

func (h *DebugEventHandler) OnServiceUpdate(service *models.ServiceInfo) {
	if h.debug {
		fmt.Printf("🔍 [DEBUG] Service更新事件:\n")
		fmt.Printf("   名称: %s/%s\n", service.Namespace, service.Name)
		fmt.Printf("   类型: %s\n", service.Type)
		fmt.Printf("   集群IP: %s\n", service.ClusterIP)
		fmt.Printf("   时间: %s\n", time.Now().Format("15:04:05"))
		fmt.Println("   ---")
	}
}

func (h *DebugEventHandler) OnEvent(event *models.EventInfo) {
	if h.debug {
		fmt.Printf("🔍 [DEBUG] 集群事件:\n")
		fmt.Printf("   类型: %s\n", event.Type)
		fmt.Printf("   原因: %s\n", event.Reason)
		fmt.Printf("   消息: %s\n", event.Message)
		fmt.Printf("   时间: %s\n", time.Now().Format("15:04:05"))
		fmt.Println("   ---")
	}
}

func (h *DebugEventHandler) OnCRDEvent(event *models.CRDEvent) {
	if !h.debug || event == nil {
		return
	}
	fmt.Printf("🔍 [DEBUG] CRD事件:\n")
	fmt.Printf("   类型: %s\n", event.Type)
	fmt.Printf("   对象: %s/%s (%s)\n", event.Group, event.Name, event.Kind)
	fmt.Printf("   时间: %s\n", time.Now().Format("15:04:05"))
	fmt.Println("   ---")
}

func main() {
	fmt.Println("🧪 调试版本 - 让我们看看代码每一步做了什么")
	fmt.Println("==================================================")

	// 第1步：加载配置
	fmt.Println("📖 第1步：加载配置文件...")
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("❌ 配置加载失败: %v", err)
	}
	fmt.Printf("✅ 配置加载成功\n")
	fmt.Printf("   K8s配置文件: %s\n", cfg.K8s.Kubeconfig)
	fmt.Printf("   监控命名空间: %s\n", cfg.K8s.WatchNamespaces)
	fmt.Println()

	// 第2步：创建K8s客户端
	fmt.Println("🔌 第2步：创建K8s客户端...")
	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("❌ 客户端创建失败: %v", err)
	}
	fmt.Printf("✅ K8s客户端创建成功\n")
	fmt.Println()

	// 第3步：测试连接
	fmt.Println("🔍 第3步：测试K8s连接...")
	if err := k8sClient.TestConnection(); err != nil {
		log.Fatalf("❌ 连接测试失败: %v", err)
	}
	fmt.Printf("✅ K8s连接成功\n")
	fmt.Println()

	// 第4步：获取集群信息
	fmt.Println("📊 第4步：获取集群基本信息...")
	clusterInfo, err := k8sClient.GetClusterInfo()
	if err != nil {
		log.Fatalf("❌ 获取集群信息失败: %v", err)
	}
	fmt.Printf("✅ 集群信息获取成功:\n")
	fmt.Printf("   K8s版本: %s\n", clusterInfo["version"])
	fmt.Printf("   节点数量: %d\n", clusterInfo["nodes"])
	fmt.Printf("   Pod总数: %d\n", clusterInfo["pods"])
	fmt.Printf("   监控命名空间: %v\n", clusterInfo["namespaces"])
	fmt.Println()

	// 第5步：获取Pod列表（详细展示）
	fmt.Println("📦 第5步：获取Pod列表...")
	for _, namespace := range k8sClient.Namespaces() {
		fmt.Printf("   正在获取命名空间 '%s' 的Pods...\n", namespace)
		pods, err := k8sClient.GetPods(namespace)
		if err != nil {
			fmt.Printf("   ❌ 获取失败: %v\n", err)
			continue
		}
		fmt.Printf("   ✅ 找到 %d 个Pods:\n", len(pods))
		for _, pod := range pods {
			fmt.Printf("     - %s (状态: %s, 节点: %s)\n", pod.Name, pod.Status, pod.NodeName)
			for _, container := range pod.Containers {
				fmt.Printf("       容器: %s (镜像: %s)\n", container.Name, container.Image)
			}
		}
		fmt.Println()
	}

	// 第6步：获取Service列表
	fmt.Println("🔗 第6步：获取Service列表...")
	for _, namespace := range k8sClient.Namespaces() {
		fmt.Printf("   正在获取命名空间 '%s' 的Services...\n", namespace)
		services, err := k8sClient.GetServices(namespace)
		if err != nil {
			fmt.Printf("   ❌ 获取失败: %v\n", err)
			continue
		}
		fmt.Printf("   ✅ 找到 %d 个Services:\n", len(services))
		for _, svc := range services {
			fmt.Printf("     - %s (类型: %s, 集群IP: %s)\n", svc.Name, svc.Type, svc.ClusterIP)
		}
		fmt.Println()
	}

	// 第7步：网络分析演示
	fmt.Println("🔍 第7步：网络分析演示...")
	// 查找两个Pod进行分析
	var podA, podB string
	for _, namespace := range k8sClient.Namespaces() {
		pods, _ := k8sClient.GetPods(namespace)
		if len(pods) >= 2 {
			podA = fmt.Sprintf("%s/%s", pods[0].Namespace, pods[0].Name)
			podB = fmt.Sprintf("%s/%s", pods[1].Namespace, pods[1].Name)
			break
		}
	}

	if podA != "" && podB != "" {
		fmt.Printf("   分析 %s 和 %s 之间的通信...\n", podA, podB)
		networkAnalyzer := k8s.NewNetworkAnalyzer(k8sClient)
		analysis, err := networkAnalyzer.AnalyzePodCommunication(context.Background(), podA, podB)
		if err != nil {
			fmt.Printf("   ❌ 分析失败: %v\n", err)
		} else {
			fmt.Printf("   ✅ 分析完成:\n")
			fmt.Printf("     状态: %s\n", analysis.Status)
			fmt.Printf("     置信度: %.2f\n", analysis.Confidence)
			fmt.Printf("     发现问题: %d个\n", len(analysis.Issues))
			for _, issue := range analysis.Issues {
				fmt.Printf("       - %s\n", issue)
			}
			fmt.Printf("     建议方案: %d个\n", len(analysis.Solutions))
			for _, solution := range analysis.Solutions {
				fmt.Printf("       - %s\n", solution)
			}
		}
	} else {
		fmt.Printf("   ❌ 没有找到足够的Pod进行分析\n")
	}
	fmt.Println()

	// 第8步：开始实时监控（30秒）
	fmt.Println("👀 第8步：开始实时监控（30秒）...")
	handler := &DebugEventHandler{debug: true}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("   监控已启动，正在监听以下命名空间: %v\n", k8sClient.Namespaces())
	fmt.Printf("   现在在另一个终端运行以下命令来创建Pod，观察监控效果:\n")
	fmt.Printf("   kubectl run test-pod --image=nginx:alpine\n")
	fmt.Printf("   或者删除Pod: kubectl delete pod <pod-name>\n")
	fmt.Println("   等待30秒，观察变化...")
	fmt.Println()

	if err := k8sClient.WatchResources(ctx, handler); err != nil {
		fmt.Printf("   ❌ 监控启动失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 监控正常结束\n")
	}

	fmt.Println("\n🎉 调试完成！现在你应该理解了代码的执行流程。")
}
