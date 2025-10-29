package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// 简化的UAV数据结构
type UAVData struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	IPAddress   string     `json:"ip_address"`
	Battery     float64    `json:"battery"`
	Latency     float64    `json:"latency"`
	Utilization float64    `json:"utilization"`
	GPS         [2]float64 `json:"gps"`
	Radius      float64    `json:"radius"`
	Status      string     `json:"status"`
}

// 节点距离信息
type NodeDistance struct {
	NodeID    string     `json:"node_id"`
	NodeName  string     `json:"node_name"`
	IPAddress string     `json:"ip_address"`
	Distance  float64    `json:"distance"`
	Score     float64    `json:"score"`
	Battery   float64    `json:"battery"`
	Latency   float64    `json:"latency"`
	GPS       [2]float64 `json:"gps"`
}

// 路由信息
type RouteInfo struct {
	TargetNodeID  string    `json:"target_node_id"`
	TargetIP      string    `json:"target_ip"`
	Distance      float64   `json:"distance"`
	Score         float64   `json:"score"`
	Priority      int       `json:"priority"`
	EstimatedRTT  float64   `json:"estimated_rtt"`
	BatteryLevel  float64   `json:"battery_level"`
	LastUpdate    time.Time `json:"last_update"`
}

// 距离计算器
type DistanceCalculator struct{}

// 计算GPS距离（Haversine公式）
func (dc *DistanceCalculator) CalculateGPSDistance(gps1, gps2 [2]float64) float64 {
	const earthRadius = 6371000.0 // 地球半径（米）

	lat1 := gps1[0] * math.Pi / 180.0
	lon1 := gps1[1] * math.Pi / 180.0
	lat2 := gps2[0] * math.Pi / 180.0
	lon2 := gps2[1] * math.Pi / 180.0

	dlat := lat2 - lat1
	dlon := lon2 - lon1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlon/2)*math.Sin(dlon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}

// 计算综合评分
func (dc *DistanceCalculator) calculateScore(distance float64, node UAVData) float64 {
	distanceScore := distance / 1000.0 // 距离（公里）

	batteryScore := 0.0
	if node.Battery > 0 {
		if node.Battery < 20 {
			batteryScore = 100.0
		} else {
			batteryScore = (100 - node.Battery) / 100.0 * 10.0
		}
	}

	latencyScore := node.Latency / 100.0
	return distanceScore + batteryScore + latencyScore
}

// 找到最近的节点
func (dc *DistanceCalculator) FindNearestNodes(targetGPS [2]float64, nodes []UAVData, maxNodes int) []NodeDistance {
	var distances []NodeDistance

	for _, node := range nodes {
		if node.GPS[0] == 0 && node.GPS[1] == 0 {
			continue
		}
		if node.Status != "ready" {
			continue
		}

		distance := dc.CalculateGPSDistance(targetGPS, node.GPS)
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

	sort.Slice(distances, func(i, j int) bool {
		return distances[i].Score < distances[j].Score
	})

	if maxNodes > 0 && len(distances) > maxNodes {
		distances = distances[:maxNodes]
	}
	return distances
}

// 构建路由矩阵
func (dc *DistanceCalculator) BuildRoutingMatrix(nodes []UAVData) map[string]map[string]RouteInfo {
	matrix := make(map[string]map[string]RouteInfo)

	for _, source := range nodes {
		if source.Status != "ready" {
			continue
		}

		routingTable := make(map[string]RouteInfo)
		nearestNodes := dc.FindNearestNodes(source.GPS, nodes, 3)

		for i, target := range nearestNodes {
			if target.NodeID == source.ID {
				continue
			}

			routeInfo := RouteInfo{
				TargetNodeID:  target.NodeID,
				TargetIP:      target.IPAddress,
				Distance:      target.Distance,
				Score:         target.Score,
				Priority:      i + 1,
				EstimatedRTT:  target.Latency,
				BatteryLevel:  target.Battery,
				LastUpdate:    time.Now(),
			}
			routingTable[target.NodeID] = routeInfo
		}
		matrix[source.ID] = routingTable
	}
	return matrix
}

func main() {
	fmt.Println("🚁 UAV Routing Demo")
	fmt.Println("==================")
	fmt.Println("Demonstrating GPS-based intelligent routing for UAV clusters")
	fmt.Println()

	// 创建模拟数据（洛杉矶地区的UAV节点）
	nodes := []UAVData{
		{
			ID:          "uav-node-1",
			Name:        "UAV Downtown LA",
			IPAddress:   "10.0.1.10",
			Battery:     85.5,
			Latency:     15.2,
			Utilization: 35.8,
			GPS:         [2]float64{34.052235, -118.243683}, // Downtown LA
			Radius:      480.9,
			Status:      "ready",
		},
		{
			ID:          "uav-node-2",
			Name:        "UAV Santa Monica",
			IPAddress:   "10.0.2.11",
			Battery:     72.3,
			Latency:     22.5,
			Utilization: 42.1,
			GPS:         [2]float64{34.019456, -118.491191}, // Santa Monica
			Radius:      399.4,
			Status:      "ready",
		},
		{
			ID:          "uav-node-3",
			Name:        "UAV Pasadena",
			IPAddress:   "10.0.3.12",
			Battery:     91.2,
			Latency:     18.8,
			Utilization: 28.5,
			GPS:         [2]float64{34.147785, -118.144516}, // Pasadena
			Radius:      305.6,
			Status:      "ready",
		},
		{
			ID:          "uav-node-4",
			Name:        "UAV LAX Airport",
			IPAddress:   "10.0.4.13",
			Battery:     68.9,
			Latency:     25.3,
			Utilization: 55.7,
			GPS:         [2]float64{33.942791, -118.408162}, // LAX
			Radius:      489.6,
			Status:      "ready",
		},
		{
			ID:          "uav-node-5",
			Name:        "UAV Hollywood",
			IPAddress:   "10.0.5.14",
			Battery:     55.4,
			Latency:     30.5,
			Utilization: 68.2,
			GPS:         [2]float64{34.092809, -118.328661}, // Hollywood
			Radius:      447.0,
			Status:      "ready",
		},
	}

	fmt.Printf("📍 Initialized %d UAV nodes in Los Angeles area\n\n", len(nodes))

	// 显示节点信息
	fmt.Println("📋 Node Status:")
	fmt.Println("===============")
	for _, node := range nodes {
		fmt.Printf("%s (%s)\n", node.Name, node.ID)
		fmt.Printf("  📍 GPS: [%.6f, %.6f]\n", node.GPS[0], node.GPS[1])
		fmt.Printf("  🔋 Battery: %.1f%%\n", node.Battery)
		fmt.Printf("  📶 Latency: %.1fms\n", node.Latency)
		fmt.Printf("  💻 IP: %s\n", node.IPAddress)
		fmt.Printf("  ✅ Status: %s\n\n", node.Status)
	}

	// 创建距离计算器
	calculator := &DistanceCalculator{}

	// 构建路由矩阵
	fmt.Println("🗺️  Building Routing Matrix...")
	matrix := calculator.BuildRoutingMatrix(nodes)

	totalRoutes := 0
	for _, table := range matrix {
		totalRoutes += len(table)
	}
	fmt.Printf("✅ Built routing matrix with %d routes\n\n", totalRoutes)

	// 演示路由查询
	fmt.Println("🧭 Routing Demo Scenarios:")
	fmt.Println("=========================")

	testCases := []struct {
		source string
		target string
		desc   string
	}{
		{"uav-node-1", "uav-node-3", "Downtown LA → Pasadena"},
		{"uav-node-2", "uav-node-4", "Santa Monica → LAX Airport"},
		{"uav-node-5", "uav-node-2", "Hollywood → Santa Monica"},
	}

	for _, tc := range testCases {
		fmt.Printf("\n🎯 Scenario: %s\n", tc.desc)
		fmt.Printf("   Source: %s → Target: %s\n", tc.source, tc.target)

		if routingTable, exists := matrix[tc.source]; exists {
			if route, exists := routingTable[tc.target]; exists {
				fmt.Printf("   ✅ Route found!\n")
				fmt.Printf("   📡 Target IP: %s\n", route.TargetIP)
				fmt.Printf("   📏 Distance: %.0f meters\n", route.Distance)
				fmt.Printf("   ⭐ Score: %.2f\n", route.Score)
				fmt.Printf("   🥇 Priority: %d\n", route.Priority)
				fmt.Printf("   🔋 Target Battery: %.1f%%\n", route.BatteryLevel)
				fmt.Printf("   📶 Estimated RTT: %.1fms\n", route.EstimatedRTT)
			} else {
				fmt.Printf("   ❌ No direct route available\n")
			}
		} else {
			fmt.Printf("   ❌ Source node not found in routing table\n")
		}
	}

	// 显示完整的路由表
	fmt.Println("\n📊 Complete Routing Tables:")
	fmt.Println("===========================")

	for sourceID, routingTable := range matrix {
		fmt.Printf("\n🚁 Source Node: %s\n", sourceID)
		fmt.Printf("   Available Routes: %d\n", len(routingTable))

		for targetID, route := range routingTable {
			fmt.Printf("   → %s: %s (%.0fm, score: %.2f, priority: %d)\n",
				targetID, route.TargetIP, route.Distance, route.Score, route.Priority)
		}
	}

	// 路离分析
	fmt.Println("\n📈 Routing Analysis:")
	fmt.Println("===================")

	shortRoutes := 0
	longRoutes := 0
	totalDistance := 0.0
	routeCount := 0

	for _, table := range matrix {
		for _, route := range table {
			totalDistance += route.Distance
			routeCount++
			if route.Distance < 5000 { // 小于5公里
				shortRoutes++
			} else {
				longRoutes++
			}
		}
	}

	if routeCount > 0 {
		avgDistance := totalDistance / float64(routeCount)
		shortPercentage := float64(shortRoutes) / float64(routeCount) * 100

		fmt.Printf("📏 Total Routes: %d\n", routeCount)
		fmt.Printf("📏 Average Distance: %.1f km\n", avgDistance/1000)
		fmt.Printf("📏 Short Routes (<5km): %d (%.1f%%)\n", shortRoutes, shortPercentage)
		fmt.Printf("📏 Long Routes (>5km): %d (%.1f%%)\n", longRoutes, 100-shortPercentage)
	}

	// Istio集成准备
	fmt.Println("\n🔮 Istio Ambient Integration Ready:")
	fmt.Println("===================================")
	fmt.Println("✅ GPS-based distance calculation implemented")
	fmt.Println("✅ Multi-factor scoring algorithm (distance + battery + latency)")
	fmt.Println("✅ Dynamic routing matrix generation")
	fmt.Println("✅ Route priority and failover support")
	fmt.Println("✅ Ready for Istio Ambient mode integration")
	fmt.Println()
	fmt.Println("📋 Next Steps for Istio Integration:")
	fmt.Println("   1. Deploy Istio in Ambient mode")
	fmt.Println("   2. Configure ztunnel for L4 routing")
	fmt.Println("   3. Deploy waypoint proxies for L7 routing")
	fmt.Println("   4. Convert routing matrix to Istio VirtualServices")
	fmt.Println("   5. Implement real-time route updates")

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Println("This demonstrates the feasibility of GPS-based intelligent routing for UAV clusters.")
}