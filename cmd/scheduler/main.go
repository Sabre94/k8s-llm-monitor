package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/internal/scheduler"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var configPath string
	var interval time.Duration
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "config file path")
	flag.DurationVar(&interval, "interval", 15*time.Second, "scheduling reconcile interval")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	k8sClient, err := k8s.NewClient(&cfg.K8s)
	if err != nil {
		log.Fatalf("Failed to create K8s client: %v", err)
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		restConfig, err = k8sClient.RESTConfig()
		if err != nil {
			log.Fatalf("Failed to get REST config: %v", err)
		}
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create kube clientset: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %v", err)
	}

	controller := scheduler.NewController(dynamicClient, kubeClient, k8sClient, scheduler.Config{
		Interval: interval,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := controller.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("Scheduler controller stopped with error: %v", err)
	}

	log.Println("Scheduler controller exited")
}
