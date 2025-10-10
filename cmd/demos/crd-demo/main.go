package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
)

// CRDDemoHandler CRD演示事件处理器
type CRDDemoHandler struct {
	k8sClient *k8s.Client
	logger    *log.Logger
}

func NewCRDDemoHandler(client *k8s.Client) *CRDDemoHandler {
	return &CRDDemoHandler{
		k8sClient: client,
		logger:    log.New(os.Stdout, "[CRD] ", log.LstdFlags),
	}
}

// 实现EventHandler接口
func (h *CRDDemoHandler) OnPodUpdate(pod *models.PodInfo) {
	// 不处理Pod事件
}

func (h *CRDDemoHandler) OnServiceUpdate(service *models.ServiceInfo) {
	// 不处理Service事件
}

func (h *CRDDemoHandler) OnEvent(event *models.EventInfo) {
	// 不处理普通事件
}

func (h *CRDDemoHandler) OnCRDEvent(event *models.CRDEvent) {
	h.logger.Printf("📡 CRD事件: %s %s/%s", event.Type, event.Kind, event.Name)

	switch event.Type {
	case "ADDED":
		h.logger.Printf("   ✅ 新增 %s 资源: %s", event.Kind, event.Name)
		if crd, ok := event.Object["crd"].(*models.CRDInfo); ok {
			h.logger.Printf("   📋 CRD详情: %s/%s (范围: %s)", crd.Group, crd.Kind, crd.Scope)
		}

	case "MODIFIED":
		h.logger.Printf("   🔄 修改 %s 资源: %s", event.Kind, event.Name)

	case "DELETED":
		h.logger.Printf("   ❌ 删除 %s 资源: %s", event.Kind, event.Name)
	}

	h.logger.Printf("   📍 命名空间: %s", event.Namespace)
	h.logger.Printf("   🕒 时间: %s", event.Timestamp.Format("15:04:05"))
	h.logger.Println("   " + strings.Repeat("-", 50))
}

func main() {
	fmt.Println("🔍 CRD监控演示")
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

	// 2. 测试连接
	if err := k8sClient.TestConnection(); err != nil {
		log.Fatalf("❌ 连接测试失败: %v", err)
	}

	// 3. 创建事件处理器
	handler := NewCRDDemoHandler(k8sClient)

	// 4. 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 5. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 6. 启动CRD监控
	fmt.Println("🚀 启动CRD监控...")
	go func() {
		if err := k8sClient.WatchCRDs(ctx, handler); err != nil {
			log.Printf("❌ CRD监控失败: %v", err)
		}
	}()

	// 7. 显示现有CRD
	fmt.Println("📋 获取现有CRD...")
	time.Sleep(2 * time.Second) // 等待监控启动

	if crdWatcher, err := k8s.NewCRDWatcher(k8sClient, handler); err == nil {
		crds, err := crdWatcher.GetCRDs(ctx)
		if err != nil {
			log.Printf("❌ 获取CRD列表失败: %v", err)
		} else {
			fmt.Printf("✅ 发现 %d 个CRD:\n", len(crds))
			for i, crd := range crds {
				fmt.Printf("   %d. %s (%s/%s) - 范围: %s, 状态: %s\n",
					i+1, crd.Name, crd.Group, crd.Kind, crd.Scope,
					map[bool]string{true: "已建立", false: "未建立"}[crd.Established])
			}
		}
	}

	// 8. 显示使用说明
	fmt.Println("\n💡 使用说明:")
	fmt.Println("   - 程序会自动监控所有CRD的创建、修改和删除")
	fmt.Println("   - 当CRD被创建时，会自动开始监控对应的自定义资源")
	fmt.Println("   - 所有自定义资源的变化都会被捕获并显示")
	fmt.Println("   - 按 Ctrl+C 退出程序")

	// 9. 等待退出信号
	select {
	case <-sigChan:
		fmt.Println("\n🛑 收到退出信号，正在停止...")
		cancel()
	case <-ctx.Done():
		fmt.Println("\n🛑 上下文已取消")
	}

	// 10. 清理
	time.Sleep(1 * time.Second)
	fmt.Println("✅ CRD监控演示完成！")
}