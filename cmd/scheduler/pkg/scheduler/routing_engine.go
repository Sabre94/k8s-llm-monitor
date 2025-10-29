package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RoutingEngine 路由引擎
type RoutingEngine struct {
	calculator     *DistanceCalculator
	matrix         RoutingMatrix
	mutex          sync.RWMutex
	dataProvider   DataProvider
	updateInterval time.Duration
	logger         *logrus.Logger
	stopCh         chan struct{}
}

// NewRoutingEngine 创建路由引擎
func NewRoutingEngine(dataProvider DataProvider, updateInterval time.Duration) *RoutingEngine {
	return &RoutingEngine{
		calculator:     NewDistanceCalculator(),
		matrix:         make(RoutingMatrix),
		dataProvider:   dataProvider,
		updateInterval: updateInterval,
		logger:         Log().WithField("component", "routing-engine").Logger,
		stopCh:         make(chan struct{}),
	}
}

// Start 启动路由引擎
func (re *RoutingEngine) Start(ctx context.Context) error {
	re.logger.Info("Starting routing engine")

	// 初始构建路由矩阵
	if err := re.buildRoutingMatrix(ctx); err != nil {
		return fmt.Errorf("failed to build initial routing matrix: %w", err)
	}

	// 启动定期更新
	go re.updateLoop(ctx)

	re.logger.Info("Routing engine started successfully")
	return nil
}

// Stop 停止路由引擎
func (re *RoutingEngine) Stop() {
	re.logger.Info("Stopping routing engine")
	close(re.stopCh)
}

// GetOptimalRoute 获取最优路由
func (re *RoutingEngine) GetOptimalRoute(sourceID, targetID string) (*RouteInfo, error) {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	route, err := re.calculator.GetOptimalRoute(sourceID, targetID, re.matrix)
	if err != nil {
		return nil, err
	}

	return route, nil
}

// GetRoutingTable 获取节点的路由表
func (re *RoutingEngine) GetRoutingTable(nodeID string) (map[string]RouteInfo, error) {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	table, exists := re.matrix.GetSourceTable(nodeID)
	if !exists {
		return nil, fmt.Errorf("routing table for node %s not found", nodeID)
	}

	// 返回副本以避免并发问题
	result := make(map[string]RouteInfo)
	for k, v := range table {
		result[k] = v
	}

	return result, nil
}

// GetAllRoutes 获取所有路由信息
func (re *RoutingEngine) GetAllRoutes() RoutingMatrix {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	// 返回副本
	result := make(RoutingMatrix)
	for sourceID, table := range re.matrix {
		result[sourceID] = make(map[string]RouteInfo)
		for targetID, route := range table {
			result[sourceID][targetID] = route
		}
	}

	return result
}

// buildRoutingMatrix 构建路由矩阵
func (re *RoutingEngine) buildRoutingMatrix(ctx context.Context) error {
	re.logger.Info("Building routing matrix")

	// 获取UAV数据
	req := &DataRequest{
		Source:    "uav-metrics-crd",
		Namespace: "default",
		Fields:    []string{"id", "battery", "latency", "utilization", "gps", "radius", "status"},
	}

	resp, err := re.dataProvider.FetchData(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch UAV data: %w", err)
	}

	if len(resp.UAVData) == 0 {
		re.logger.Warn("No UAV data available for routing")
		return nil
	}

	// 构建新的路由矩阵
	newMatrix := re.calculator.BuildRoutingMatrix(resp.UAVData)

	// 更新路由矩阵
	re.mutex.Lock()
	re.matrix = newMatrix
	re.mutex.Unlock()

	re.logger.WithFields(logrus.Fields{
		"nodes":  len(resp.UAVData),
		"routes": re.calculator.countRoutes(newMatrix),
	}).Info("Routing matrix built successfully")

	return nil
}

// updateLoop 定期更新路由矩阵
func (re *RoutingEngine) updateLoop(ctx context.Context) {
	ticker := time.NewTicker(re.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			re.logger.Info("Routing engine update loop stopped")
			return
		case <-re.stopCh:
			re.logger.Info("Routing engine update loop stopped")
			return
		case <-ticker.C:
			if err := re.buildRoutingMatrix(ctx); err != nil {
				re.logger.WithError(err).Error("Failed to update routing matrix")
			}
		}
	}
}

// RoutingDecision 路由决策
type RoutingDecision struct {
	SourceNode      string      `json:"source_node"`
	TargetNode      string      `json:"target_node"`
	SelectedRoute   *RouteInfo  `json:"selected_route"`
	AlternativeRoutes []RouteInfo `json:"alternative_routes"`
	DecisionTime    time.Time   `json:"decision_time"`
	DecisionReason  string      `json:"decision_reason"`
}

// MakeRoutingDecision 做出路由决策
func (re *RoutingEngine) MakeRoutingDecision(sourceID, targetID string) (*RoutingDecision, error) {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	// 获取主路由
	route, err := re.calculator.GetOptimalRoute(sourceID, targetID, re.matrix)
	if err != nil {
		return nil, fmt.Errorf("failed to get optimal route: %w", err)
	}

	// 获取备选路由
	var alternatives []RouteInfo
	if table, exists := re.matrix[sourceID]; exists {
		for targetID, routeInfo := range table {
			if targetID != targetID && routeInfo.Priority <= 3 { // 前3个备选
				alternatives = append(alternatives, routeInfo)
			}
		}
	}

	decision := &RoutingDecision{
		SourceNode:        sourceID,
		TargetNode:        targetID,
		SelectedRoute:     route,
		AlternativeRoutes: alternatives,
		DecisionTime:      time.Now(),
		DecisionReason:    re.generateDecisionReason(route, alternatives),
	}

	return decision, nil
}

// generateDecisionReason 生成决策原因
func (re *RoutingEngine) generateDecisionReason(selected *RouteInfo, alternatives []RouteInfo) string {
	reason := fmt.Sprintf("Selected route based on optimal score (%.2f) with distance %.0fm and battery %.1f%%",
		selected.Score, selected.Distance, selected.BatteryLevel)

	if len(alternatives) > 0 {
		reason += fmt.Sprintf(". %d alternative routes available", len(alternatives))
	}

	return reason
}

// GetRoutingStats 获取路由统计信息
func (re *RoutingEngine) GetRoutingStats() RoutingStats {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	stats := RoutingStats{
		TotalNodes:      len(re.matrix),
		TotalRoutes:     re.calculator.countRoutes(re.matrix),
		LastUpdate:      time.Now(),
		AverageDistance: 0,
		AverageScore:    0,
	}

	// 计算平均值
	totalDistance := 0.0
	totalScore := 0.0
	routeCount := 0

	for _, table := range re.matrix {
		for _, route := range table {
			totalDistance += route.Distance
			totalScore += route.Score
			routeCount++
		}
	}

	if routeCount > 0 {
		stats.AverageDistance = totalDistance / float64(routeCount)
		stats.AverageScore = totalScore / float64(routeCount)
	}

	return stats
}

// RoutingStats 路由统计信息
type RoutingStats struct {
	TotalNodes      int       `json:"total_nodes"`
	TotalRoutes     int       `json:"total_routes"`
	AverageDistance float64   `json:"average_distance"`
	AverageScore    float64   `json:"average_score"`
	LastUpdate      time.Time `json:"last_update"`
}