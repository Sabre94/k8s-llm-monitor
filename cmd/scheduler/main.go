package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/yourusername/k8s-llm-monitor/cmd/scheduler/pkg/scheduler"
)

var (
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	dryRun     = flag.Bool("dry-run", false, "Dry run mode (don't actually bind pods)")
)

func main() {
	flag.Parse()

	// 设置日志级别
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Fatal("Invalid log level")
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	logrus.Info("Starting UAV Scheduler")

	// 加载配置
	config, err := loadConfig(*configFile)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	logrus.WithField("config", *configFile).Info("Configuration loaded")

	// 创建调度器
	sched := scheduler.NewUAVScheduler(config)

	// 注册数据提供者
	if err := registerDataProviders(sched, config); err != nil {
		logrus.WithError(err).Fatal("Failed to register data providers")
	}

	// 注册算法
	if err := registerAlgorithms(sched, config); err != nil {
		logrus.WithError(err).Fatal("Failed to register algorithms")
	}

	// 注册Pod绑定器
	if err := registerPodBinder(sched, config, *dryRun); err != nil {
		logrus.WithError(err).Fatal("Failed to register pod binder")
	}

	// 启动调度器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Failed to start scheduler")
	}

	logrus.Info("UAV Scheduler started successfully")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动示例调度（如果启用）
	demoEnabled := getDemoEnabled(config)
	logrus.WithField("demo_enabled", demoEnabled).Info("Demo mode check")
	if demoEnabled == "true" {
		logrus.Info("Starting demo mode")
		go runDemo(sched, config)
	} else {
		logrus.Info("Demo mode disabled")
	}

	// 等待信号
	<-sigChan
	logrus.Info("Shutdown signal received")

	// 停止调度器
	if err := sched.Stop(); err != nil {
		logrus.WithError(err).Error("Failed to stop scheduler gracefully")
	}

	logrus.Info("UAV Scheduler stopped")
}

// loadConfig 加载配置文件
func loadConfig(filename string) (*scheduler.Config, error) {
	// 加载默认配置
	config := scheduler.DefaultConfig()

	// 如果文件存在，加载配置
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		logrus.WithField("file", filename).Warn("Config file not found, using defaults")
	}

	return config, nil
}

// registerDataProviders 注册数据提供者
func registerDataProviders(sched scheduler.Scheduler, config *scheduler.Config) error {
	// 注册CRD数据提供者
	crdProvider, err := scheduler.NewCRDDataProvider("crd", &config.K8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create CRD data provider: %w", err)
	}

	if err := sched.RegisterDataProvider(crdProvider); err != nil {
		return fmt.Errorf("failed to register CRD data provider: %w", err)
	}

	logrus.Info("CRD data provider registered")

	return nil
}

// registerAlgorithms 注册算法
func registerAlgorithms(sched scheduler.Scheduler, config *scheduler.Config) error {
	// 注册NSGA-II算法
	nsga2Config := getAlgorithmConfig(config, "nsga2")

	nsga2Algorithm := scheduler.NewNSGA2Algorithm("nsga2", nsga2Config)

	if err := nsga2Algorithm.Validate(nsga2Config); err != nil {
		logrus.WithError(err).Warn("NSGA-II algorithm validation failed, but continuing")
	}

	if err := sched.RegisterAlgorithm(nsga2Algorithm); err != nil {
		return fmt.Errorf("failed to register NSGA-II algorithm: %w", err)
	}

	logrus.Info("NSGA-II algorithm registered")

	// 可以在这里注册更多算法
	// 例如：greedy算法、random算法等

	return nil
}

// registerPodBinder 注册Pod绑定器
func registerPodBinder(sched scheduler.Scheduler, config *scheduler.Config, dryRun bool) error {
	var binder scheduler.PodBinder
	var err error

	if dryRun {
		binder = scheduler.NewMockPodBinder()
		logrus.Info("Mock pod binder registered (dry run mode)")
	} else {
		binder, err = scheduler.NewK8sPodBinder(&config.K8sConfig)
		if err != nil {
			return fmt.Errorf("failed to create K8s pod binder: %w", err)
		}
		logrus.Info("K8s pod binder registered")
	}

	if err := sched.SetPodBinder(binder); err != nil {
		return fmt.Errorf("failed to set pod binder: %w", err)
	}

	return nil
}

// getAlgorithmConfig 获取算法配置
func getAlgorithmConfig(config *scheduler.Config, algorithmName string) map[string]interface{} {
	if algoConfig, ok := config.Algorithms[algorithmName]; ok {
		if configMap, ok := algoConfig.(map[string]interface{}); ok {
			return configMap
		}
	}

	// 返回默认配置
	return map[string]interface{}{
		"population_size": 50,
		"max_generations": 20,
		"crossover_prob":  0.9,
		"grid_density":    40,
		"python_path":     "python3",
		"script_path":     "RUN2.py",
	}
}

// runDemo 运行演示
func runDemo(sched scheduler.Scheduler, config *scheduler.Config) {
	logrus.Info("Starting demo mode")

	// 等待调度器完全启动
	time.Sleep(time.Second * 5)

	// 演示单个Pod调度
	demoSinglePodScheduling(sched)

	// 演示Pod组调度
	demoPodGroupScheduling(sched)
}

// demoSinglePodScheduling 演示单个Pod调度
func demoSinglePodScheduling(sched scheduler.Scheduler) {
	logrus.Info("Demo: Single pod scheduling")

	req := &scheduler.ScheduleRequest{
		PodName:      "demo-uav-mission-1",
		PodNamespace: "default",
		AlgorithmName: "nsga2",
		Requirements: &scheduler.OptimizeRequest{
			TaskType:       "surveillance",  // 任务类型
			Priority:       "high",
			Objectives:     []string{"battery", "latency", "utilization", "count"},
			TargetCoverage: 0.8,             // 目标覆盖率80%
			MaxUAVs:        5,               // 最多选择5个UAV节点
			Constraints: map[string]interface{}{
				"min_battery": 50.0,
				"max_latency": 200.0,
			},
		},
		Options: map[string]interface{}{
			"demo": true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	result, err := sched.SchedulePod(ctx, req)
	if err != nil {
		logrus.WithError(err).Error("Demo mission scheduling failed")
		return
	}

	logrus.WithFields(logrus.Fields{
		"mission_pod":        result.PodName,
		"assigned_node_set":  result.AssignedNodeSet,     // 关键：返回节点集合！
		"node_count":         len(result.AssignedNodeSet),
		"success":            result.Success,
		"execution_time":     result.ExecutionTime,
	}).Info("Demo mission scheduling completed")

	// 显示优化结果
	if result.OptimizationResult != nil {
		logrus.WithFields(logrus.Fields{
			"algorithm":        result.OptimizationResult.AlgorithmName,
			"coverage_ratio":   result.OptimizationResult.CoverageRatio,
			"score":            result.OptimizationResult.Score,
			"pareto_solutions": len(result.OptimizationResult.ParetoFront),
		}).Info("Optimization result details")
	}
}

// demoPodGroupScheduling 演示Pod组调度
func demoPodGroupScheduling(sched scheduler.Scheduler) {
	logrus.Info("Demo: Pod group scheduling")

	pods := []scheduler.PodRef{
		{Name: "demo-uav-pod-2", Namespace: "default", Priority: 1},
		{Name: "demo-uav-pod-3", Namespace: "default", Priority: 2},
		{Name: "demo-uav-pod-4", Namespace: "default", Priority: 3},
	}

	req := &scheduler.ScheduleGroupRequest{
		Pods:          pods,
		AlgorithmName: "nsga2",
		Requirements: &scheduler.OptimizeRequest{
			TaskType:       "emergency",
			Priority:       "critical",
			Objectives:     []string{"battery", "latency", "utilization", "count"},
			TargetCoverage: 0.9,
			MaxUAVs:        8,
			Constraints: map[string]interface{}{
				"min_battery": 60.0,
				"max_latency": 150.0,
			},
		},
		Options: map[string]interface{}{
			"demo": true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	result, err := sched.SchedulePodGroup(ctx, req)
	if err != nil {
		logrus.WithError(err).Error("Demo pod group scheduling failed")
		return
	}

	logrus.WithFields(logrus.Fields{
		"total_pods":      result.TotalPods,
		"successful_pods": result.SuccessfulPods,
		"success":         result.Success,
		"execution_time":  result.ExecutionTime,
	}).Info("Demo pod group scheduling completed")

	// 打印绑定详情
	for _, binding := range result.Bindings {
		logrus.WithFields(logrus.Fields{
			"pod_name":  binding.PodName,
			"node_name": binding.NodeName,
			"reason":    binding.Reason,
		}).Info("Pod binding details")
	}
}

// 辅助方法：从配置中获取演示模式启用状态
func getDemoEnabled(config *scheduler.Config) string {
	// 处理YAML解析导致的类型问题
	demoValue := config.Algorithms["demo"]
	if demoValue == nil {
		logrus.Debug("Demo key not found in algorithms config")
		return "false"
	}

	// 尝试类型转换
	var demoConfig map[string]interface{}

	// 直接转换
	if dc, ok := demoValue.(map[string]interface{}); ok {
		demoConfig = dc
	} else if dc, ok := demoValue.(map[interface{}]interface{}); ok {
		// 处理yaml.v2解析的结果
		demoConfig = make(map[string]interface{})
		for k, v := range dc {
			if keyStr, ok := k.(string); ok {
				demoConfig[keyStr] = v
			}
		}
	} else {
		logrus.WithField("demo_value_type", fmt.Sprintf("%T", demoValue)).Debug("Demo config has unexpected type")
		return "false"
	}

	logrus.WithField("demo_config", demoConfig).Debug("Demo config parsed")

	if enabled, ok := demoConfig["enabled"].(bool); ok && enabled {
		logrus.Info("Demo mode is enabled")
		return "true"
	} else {
		logrus.WithField("enabled", enabled).Debug("Demo enabled field value")
	}

	return "false"
}
