package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// RoutingDemo 路由演示程序
type RoutingDemo struct {
	engine        *RoutingEngine
	configGen     *RoutingConfigGenerator
	logger        *logrus.Logger
}

// NewRoutingDemo 创建路由演示
func NewRoutingDemo() *RoutingDemo {
	return &RoutingDemo{
		configGen: NewRoutingConfigGenerator(),
		logger:    Log().WithField("component", "routing-demo").Logger,
	}
}

// RunDemo 运行路由演示
func (rd *RoutingDemo) RunDemo(ctx context.Context) error {
	rd.logger.Info("Starting routing demo")

	// 1. 创建模拟数据提供者
	dataProvider := NewMockDataProvider()

	// 2. 创建路由引擎
	rd.engine = NewRoutingEngine(dataProvider, 30*time.Second)

	// 3. 启动路由引擎
	if err := rd.engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start routing engine: %w", err)
	}
	defer rd.engine.Stop()

	// 4. 运行演示场景
	rd.runScenarios(ctx)

	rd.logger.Info("Routing demo completed")
	return nil
}

// runScenarios 运行演示场景
func (rd *RoutingDemo) runScenarios(ctx context.Context) {
	// 等待路由引擎初始化
	time.Sleep(2 * time.Second)

	// 场景1：展示基本路由功能
	rd.demonstrateBasicRouting()

	// 场景2：演示路由决策
	rd.demonstrateRoutingDecision()

	// 场景3：生成路由配置
	rd.demonstrateConfigGeneration()

	// 场景4：展示路由统计
	rd.demonstrateRoutingStats()
}

// demonstrateBasicRouting 演示基本路由功能
func (rd *RoutingDemo) demonstrateBasicRouting() {
	rd.logger.Info("=== Scenario 1: Basic Routing Demo ===")

	// 模拟的UAV节点ID
	sourceNode := "uav-node-1"
	targetNode := "uav-node-3"

	// 获取路由
	route, err := rd.engine.GetOptimalRoute(sourceNode, targetNode)
	if err != nil {
		rd.logger.WithError(err).Error("Failed to get route")
		return
	}

	rd.logger.WithFields(logrus.Fields{
		"source":     sourceNode,
		"target":     targetNode,
		"target_ip":  route.TargetIP,
		"distance":   fmt.Sprintf("%.0fm", route.Distance),
		"score":      fmt.Sprintf("%.2f", route.Score),
		"priority":   route.Priority,
		"battery":    fmt.Sprintf("%.1f%%", route.BatteryLevel),
		"estimated_rtt": fmt.Sprintf("%.1fms", route.EstimatedRTT),
	}).Info("Optimal route found")

	// 展示路由表
	table, err := rd.engine.GetRoutingTable(sourceNode)
	if err != nil {
		rd.logger.WithError(err).Error("Failed to get routing table")
		return
	}

	rd.logger.WithField("routes_count", len(table)).Info("Routing table for source node")
	for targetID, routeInfo := range table {
		rd.logger.WithFields(logrus.Fields{
			"target":    targetID,
			"distance":  fmt.Sprintf("%.0fm", routeInfo.Distance),
			"score":     fmt.Sprintf("%.2f", routeInfo.Score),
			"priority":  routeInfo.Priority,
		}).Info("Available route")
	}
}

// demonstrateRoutingDecision 演示路由决策
func (rd *RoutingDemo) demonstrateRoutingDecision() {
	rd.logger.Info("=== Scenario 2: Routing Decision Demo ===")

	testCases := []struct {
		source string
		target string
		desc   string
	}{
		{"uav-node-1", "uav-node-2", "Normal routing between active nodes"},
		{"uav-node-3", "uav-node-1", "Reverse routing test"},
		{"uav-node-2", "uav-node-4", "Long distance routing"},
	}

	for _, tc := range testCases {
		rd.logger.WithField("description", tc.desc).Info("Testing routing decision")

		decision, err := rd.engine.MakeRoutingDecision(tc.source, tc.target)
		if err != nil {
			rd.logger.WithError(err).Error("Failed to make routing decision")
			continue
		}

		rd.logger.WithFields(logrus.Fields{
			"source":       decision.SourceNode,
			"target":       decision.TargetNode,
			"selected_ip":  decision.SelectedRoute.TargetIP,
			"distance":     fmt.Sprintf("%.0fm", decision.SelectedRoute.Distance),
			"score":        fmt.Sprintf("%.2f", decision.SelectedRoute.Score),
			"alternatives": len(decision.AlternativeRoutes),
			"reason":       decision.DecisionReason,
		}).Info("Routing decision made")

		// 展示备选路由
		for i, alt := range decision.AlternativeRoutes {
			rd.logger.WithFields(logrus.Fields{
				"alternative": i + 1,
				"target_ip":   alt.TargetIP,
				"distance":    fmt.Sprintf("%.0fm", alt.Distance),
				"score":       fmt.Sprintf("%.2f", alt.Score),
				"priority":    alt.Priority,
			}).Info("Alternative route")
		}
	}
}

// demonstrateConfigGeneration 演示配置生成
func (rd *RoutingDemo) demonstrateConfigGeneration() {
	rd.logger.Info("=== Scenario 3: Configuration Generation Demo ===")

	// 获取当前路由矩阵
	allRoutes := rd.engine.GetAllRoutes()

	// 生成简单配置
	simpleConfig := rd.configGen.GenerateSimpleRoutingConfig(allRoutes)
	rd.logger.WithField("routes_count", len(simpleConfig.Routes)).Info("Generated simple routing config")

	// 打印几条路由示例
	for i, route := range simpleConfig.Routes {
		if i >= 5 { // 只显示前5条
			break
		}
		rd.logger.WithFields(logrus.Fields{
			"source": route.Source,
			"target": route.Target,
			"ip":     route.TargetIP,
			"distance": fmt.Sprintf("%.0fm", route.Distance),
		}).Info("Route example")
	}

	// 生成Kubernetes配置
	k8sConfig := rd.configGen.GenerateKubernetesConfig(allRoutes)
	rd.logger.WithField("data_keys", len(k8sConfig.Data)).Info("Generated Kubernetes ConfigMap")

	// 生成Istio配置（为将来准备）
	istioConfig := rd.configGen.GenerateIstioConfig(allRoutes)
	rd.logger.WithFields(logrus.Fields{
		"service_entries":  len(istioConfig.Spec.ServiceEntries),
		"virtual_services": len(istioConfig.Spec.VirtualServices),
	}).Info("Generated Istio configuration (for future integration)")
}

// demonstrateRoutingStats 演示路由统计
func (rd *RoutingDemo) demonstrateRoutingStats() {
	rd.logger.Info("=== Scenario 4: Routing Statistics Demo ===")

	stats := rd.engine.GetRoutingStats()

	rd.logger.WithFields(logrus.Fields{
		"total_nodes":      stats.TotalNodes,
		"total_routes":     stats.TotalRoutes,
		"average_distance": fmt.Sprintf("%.0fm", stats.AverageDistance),
		"average_score":    fmt.Sprintf("%.2f", stats.AverageScore),
	}).Info("Routing statistics")

	// 计算路由效率
	if stats.TotalRoutes > 0 {
		shortRoutes := 0
		longRoutes := 0

		allRoutes := rd.engine.GetAllRoutes()
		for _, table := range allRoutes {
			for _, route := range table {
				if route.Distance < 500 { // 小于500米为短距离
					shortRoutes++
				} else {
					longRoutes++
				}
			}
		}

		shortPercentage := float64(shortRoutes) / float64(stats.TotalRoutes) * 100
		longPercentage := float64(longRoutes) / float64(stats.TotalRoutes) * 100

		rd.logger.WithFields(logrus.Fields{
			"short_routes":      shortRoutes,
			"long_routes":       longRoutes,
			"short_percentage":  fmt.Sprintf("%.1f%%", shortPercentage),
			"long_percentage":   fmt.Sprintf("%.1f%%", longPercentage),
		}).Info("Route efficiency analysis")
	}
}

// PrintRoutingMatrix 打印路由矩阵（用于调试）
func (rd *RoutingDemo) PrintRoutingMatrix() {
	rd.logger.Info("=== Current Routing Matrix ===")

	allRoutes := rd.engine.GetAllRoutes()

	for sourceID, table := range allRoutes {
		rd.logger.WithField("source", sourceID).Info("Routing table:")
		for targetID, route := range table {
			rd.logger.WithFields(logrus.Fields{
				"target": targetID,
				"ip":     route.TargetIP,
				"distance": fmt.Sprintf("%.0fm", route.Distance),
				"score":    fmt.Sprintf("%.2f", route.Score),
				"priority": route.Priority,
			}).Info("Route")
		}
	}
}

// MockDataProvider 模拟数据提供者
type MockDataProvider struct {
	data []UAVData
}

// NewMockDataProvider 创建模拟数据提供者
func NewMockDataProvider() *MockDataProvider {
	// 模拟洛杉矶地区的UAV节点数据
	return &MockDataProvider{
		data: []UAVData{
			{
				ID:            "uav-node-1",
				Name:          "UAV Downtown LA",
				IPAddress:     "10.0.1.10",
				Battery:       85.5,
				Latency:       15.2,
				Utilization:   35.8,
				GPS:           [2]float64{34.052235, -118.243683}, // Downtown LA
				Radius:        480.9,
				Status:        "ready",
				LastHeartbeat: time.Now().Add(-time.Second * 10),
				Labels: map[string]string{
					"type": "uav",
					"zone": "downtown",
				},
			},
			{
				ID:            "uav-node-2",
				Name:          "UAV Santa Monica",
				IPAddress:     "10.0.2.11",
				Battery:       72.3,
				Latency:       22.5,
				Utilization:   42.1,
				GPS:           [2]float64{34.019456, -118.491191}, // Santa Monica
				Radius:        399.4,
				Status:        "ready",
				LastHeartbeat: time.Now().Add(-time.Second * 15),
				Labels: map[string]string{
					"type": "uav",
					"zone": "westside",
				},
			},
			{
				ID:            "uav-node-3",
				Name:          "UAV Pasadena",
				IPAddress:     "10.0.3.12",
				Battery:       91.2,
				Latency:       18.8,
				Utilization:   28.5,
				GPS:           [2]float64{34.147785, -118.144516}, // Pasadena
				Radius:        305.6,
				Status:        "ready",
				LastHeartbeat: time.Now().Add(-time.Second * 5),
				Labels: map[string]string{
					"type": "uav",
					"zone": "eastside",
				},
			},
			{
				ID:            "uav-node-4",
				Name:          "UAV LAX Airport",
				IPAddress:     "10.0.4.13",
				Battery:       68.9,
				Latency:       25.3,
				Utilization:   55.7,
				GPS:           [2]float64{33.942791, -118.408162}, // LAX
				Radius:        489.6,
				Status:        "ready",
				LastHeartbeat: time.Now().Add(-time.Second * 8),
				Labels: map[string]string{
					"type": "uav",
					"zone": "airport",
				},
			},
			{
				ID:            "uav-node-5",
				Name:          "UAV Hollywood",
				IPAddress:     "10.0.5.14",
				Battery:       55.4,
				Latency:       30.5,
				Utilization:   68.2,
				GPS:           [2]float64{34.092809, -118.328661}, // Hollywood
				Radius:        447.0,
				Status:        "ready",
				LastHeartbeat: time.Now().Add(-time.Second * 20),
				Labels: map[string]string{
					"type": "uav",
					"zone": "hollywood",
				},
			},
		},
	}
}

// Name 返回数据提供者名称
func (mdp *MockDataProvider) Name() string {
	return "mock-data-provider"
}

// FetchData 获取数据
func (mdp *MockDataProvider) FetchData(ctx context.Context, req *DataRequest) (*DataResponse, error) {
	return &DataResponse{
		Source:   req.Source,
		Count:    len(mdp.data),
		UAVData:  mdp.data,
		Metadata: map[string]interface{}{
			"namespace": req.Namespace,
			"source":    "mock",
		},
		Timestamp: time.Now(),
	}, nil
}

// WatchData 监听数据变化
func (mdp *MockDataProvider) WatchData(ctx context.Context, callback DataCallback) error {
	// 模拟数据变化
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 模拟数据更新（随机改变电池电量）
				for i := range mdp.data {
					// 简单的电池消耗模拟
					mdp.data[i].Battery = max(20, mdp.data[i].Battery-1)
					mdp.data[i].LastHeartbeat = time.Now()
				}

				// 调用回调
				response, err := mdp.FetchData(ctx, &DataRequest{
					Source:    "mock",
					Namespace: "default",
				})
				if err != nil {
					continue
				}

				if err := callback(response); err != nil {
					return
				}
			}
		}
	}()

	return nil
}

// max 返回最大值
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}