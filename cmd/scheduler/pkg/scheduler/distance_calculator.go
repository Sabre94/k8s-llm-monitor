package scheduler

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

// DistanceCalculator 距离计算器
type DistanceCalculator struct {
	logger *logrus.Logger
}

// NewDistanceCalculator 创建距离计算器
func NewDistanceCalculator() *DistanceCalculator {
	return &DistanceCalculator{
		logger: Log().WithField("component", "distance-calculator").Logger,
	}
}

// CalculateGPSDistance 计算两个GPS坐标之间的距离（Haversine公式）
// 返回距离单位：米
func (dc *DistanceCalculator) CalculateGPSDistance(gps1, gps2 [2]float64) float64 {
	if len(gps1) != 2 || len(gps2) != 2 {
		return math.MaxFloat64
	}

	// 地球半径（米）
	const earthRadius = 6371000.0

	// 转换为弧度
	lat1 := gps1[0] * math.Pi / 180.0
	lon1 := gps1[1] * math.Pi / 180.0
	lat2 := gps2[0] * math.Pi / 180.0
	lon2 := gps2[1] * math.Pi / 180.0

	// Haversine公式
	dlat := lat2 - lat1
	dlon := lon2 - lon1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlon/2)*math.Sin(dlon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := earthRadius * c
	return distance
}

// FindNearestNodes 找到离目标节点最近的N个节点
func (dc *DistanceCalculator) FindNearestNodes(targetGPS [2]float64, nodes []UAVData, maxNodes int) []NodeDistance {
	if len(nodes) == 0 {
		return []NodeDistance{}
	}

	var distances []NodeDistance

	for _, node := range nodes {
		// 跳过无效的GPS数据
		if node.GPS[0] == 0 && node.GPS[1] == 0 {
			continue
		}

		// 跳过离线节点
		if node.Status != "ready" {
			continue
		}

		distance := dc.CalculateGPSDistance(targetGPS, node.GPS)

		// 计算综合评分（距离 + 其他因素）
		score := dc.calculateScore(distance, node)

		nd := NodeDistance{
			NodeID:    node.ID,
			NodeName:  node.Name,
			IPAddress: node.IPAddress,
			Distance:  distance,
			Score:     score,
			Battery:   node.Battery,
			Latency:   node.Latency,
			GPS:       node.GPS,
		}

		distances = append(distances, nd)
	}

	// 按评分排序（评分越低越好）
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].Score < distances[j].Score
	})

	// 返回最近的N个节点
	if maxNodes > 0 && len(distances) > maxNodes {
		distances = distances[:maxNodes]
	}

	return distances
}

// calculateScore 计算综合评分
// 评分越低表示越优
func (dc *DistanceCalculator) calculateScore(distance float64, node UAVData) float64 {
	// 基础评分：距离（公里）
	distanceScore := distance / 1000.0

	// 电池评分：电池越低评分越高（惩罚低电量）
	batteryScore := 0.0
	if node.Battery > 0 {
		if node.Battery < 20 {
			batteryScore = 100.0 // 低电量严重惩罚
		} else {
			batteryScore = (100 - node.Battery) / 100.0 * 10.0
		}
	}

	// 延迟评分：延迟越高评分越高
	latencyScore := node.Latency / 100.0 // 转换为秒再评分

	// 综合评分
	totalScore := distanceScore + batteryScore + latencyScore

	return totalScore
}

// BuildRoutingMatrix 构建路由矩阵
func (dc *DistanceCalculator) BuildRoutingMatrix(nodes []UAVData) RoutingMatrix {
	matrix := make(RoutingMatrix)

	for _, source := range nodes {
		if source.Status != "ready" {
			continue
		}

		routingTable := make(map[string]RouteInfo)
		nearestNodes := dc.FindNearestNodes(source.GPS, nodes, 3) // 找到最近3个节点

		for i, target := range nearestNodes {
			// 跳过自己
			if target.NodeID == source.ID {
				continue
			}

			routeInfo := RouteInfo{
				TargetNodeID: target.NodeID,
				TargetIP:     target.IPAddress,
				Distance:     target.Distance,
				Score:        target.Score,
				Priority:     i + 1, // 路由优先级
				EstimatedRTT: target.Latency,
				BatteryLevel: target.Battery,
				LastUpdate:   time.Now(),
			}

			routingTable[target.NodeID] = routeInfo
		}

		matrix[source.ID] = routingTable
	}

	return matrix
}

// GetOptimalRoute 获取到目标节点的最优路由
func (dc *DistanceCalculator) GetOptimalRoute(sourceID, targetID string, matrix RoutingMatrix) (*RouteInfo, error) {
	sourceTable, exists := matrix[sourceID]
	if !exists {
		return nil, fmt.Errorf("source node %s not found in routing matrix", sourceID)
	}

	route, exists := sourceTable[targetID]
	if !exists {
		return nil, fmt.Errorf("route from %s to %s not found", sourceID, targetID)
	}

	return &route, nil
}

// UpdateRoutingMatrix 更新路由矩阵
func (dc *DistanceCalculator) UpdateRoutingMatrix(ctx context.Context, matrix RoutingMatrix, nodes []UAVData) RoutingMatrix {
	dc.logger.Info("Updating routing matrix")

	// 重新构建路由矩阵
	newMatrix := dc.BuildRoutingMatrix(nodes)

	dc.logger.WithFields(logrus.Fields{
		"nodes":       len(nodes),
		"routes":      dc.countRoutes(newMatrix),
		"update_time": time.Now(),
	}).Info("Routing matrix updated")

	return newMatrix
}

// countRoutes 统计路由数量
func (dc *DistanceCalculator) countRoutes(matrix RoutingMatrix) int {
	count := 0
	for _, table := range matrix {
		count += len(table)
	}
	return count
}

// NodeDistance 节点距离信息
type NodeDistance struct {
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	IPAddress string    `json:"ip_address"`
	Distance  float64   `json:"distance"`  // 距离（米）
	Score     float64   `json:"score"`     // 综合评分
	Battery   float64   `json:"battery"`   // 电池电量
	Latency   float64   `json:"latency"`   // 延迟
	GPS       [2]float64 `json:"gps"`      // GPS坐标
}

// RouteInfo 路由信息
type RouteInfo struct {
	TargetNodeID  string    `json:"target_node_id"`
	TargetIP      string    `json:"target_ip"`
	Distance      float64   `json:"distance"`      // 距离（米）
	Score         float64   `json:"score"`         // 综合评分
	Priority      int       `json:"priority"`      // 路由优先级（1最高）
	EstimatedRTT  float64   `json:"estimated_rtt"` // 预估RTT
	BatteryLevel  float64   `json:"battery_level"` // 电池电量
	LastUpdate    time.Time `json:"last_update"`
}

// RoutingMatrix 路由矩阵
type RoutingMatrix map[string]map[string]RouteInfo // sourceID -> targetID -> RouteInfo

// ToJSON 转换为JSON字符串
func (rm RoutingMatrix) ToJSON() string {
	data, _ := json.MarshalIndent(rm, "", "  ")
	return string(data)
}

// GetSourceTable 获取源节点的路由表
func (rm RoutingMatrix) GetSourceTable(sourceID string) (map[string]RouteInfo, bool) {
	table, exists := rm[sourceID]
	return table, exists
}

// AddRoute 添加路由
func (rm RoutingMatrix) AddRoute(sourceID, targetID string, route RouteInfo) {
	if _, exists := rm[sourceID]; !exists {
		rm[sourceID] = make(map[string]RouteInfo)
	}
	rm[sourceID][targetID] = route
}

// RemoveRoute 移除路由
func (rm RoutingMatrix) RemoveRoute(sourceID, targetID string) {
	if table, exists := rm[sourceID]; exists {
		delete(table, targetID)
	}
}