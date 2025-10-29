package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	scheduler "github.com/yourusername/k8s-llm-monitor/cmd/scheduler/pkg/scheduler"
)

func main() {
	var (
		duration = flag.Duration("duration", 5*time.Minute, "Demo duration")
		verbose  = flag.Bool("verbose", true, "Enable verbose logging")
		config   = flag.String("config", "", "Path to scheduler config (optional)")
	)
	flag.Parse()

	if *verbose {
		// 设置详细日志
		scheduler.Log().SetLevel(logrus.DebugLevel)
	}

	fmt.Println("🚁 UAV Routing Demo")
	fmt.Println("==================")
	fmt.Printf("Duration: %s\n", *duration)
	fmt.Printf("Verbose logging: %v\n", *verbose)
	fmt.Println()

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// 创建路由演示
	demo := scheduler.NewRoutingDemo()

	// 启动演示
	fmt.Println("🚀 Starting routing demo...")

	// 在goroutine中运行演示
	demoDone := make(chan error, 1)
	go func() {
		demoDone <- demo.RunDemo(ctx)
	}()

	// 处理优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-demoDone:
		if err != nil {
			log.Printf("❌ Demo failed: %v", err)
			os.Exit(1)
		}
		fmt.Println("✅ Demo completed successfully")
	case sig := <-sigCh:
		fmt.Printf("\n🛑 Received signal %v, shutting down...\n", sig)
		cancel()

		// 等待演示完成或超时
		select {
		case <-demoDone:
			fmt.Println("✅ Demo shut down gracefully")
		case <-time.After(10 * time.Second):
			fmt.Println("⚠️  Demo shutdown timeout")
		}
	case <-ctx.Done():
		fmt.Println("⏰ Demo duration completed")
	}

	// 打印最终统计
	fmt.Println("\n📊 Final Demo Summary")
	fmt.Println("====================")
}