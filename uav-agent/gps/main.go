package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// GPSData GPS数据结构
type GPSData struct {
	Latitude       float64    `json:"latitude"`
	Longitude      float64    `json:"longitude"`
	Altitude       float64    `json:"altitude"`
	Battery        float64    `json:"battery"`
	NodeID         string     `json:"node_id"`
	NodeName       string     `json:"node_name"`
	NodeIP         string     `json:"node_ip"`
	LastUpdate     time.Time  `json:"last_update"`
	Status         string     `json:"status"`
}

// GPSService GPS服务
type GPSService struct {
	nodeID      string
	nodeName    string
	nodeIP      string
	gpsData     GPSData
}

// NewGPSService 创建GPS服务
func NewGPSService() *GPSService {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "unknown"
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "UAV Node"
	}

	nodeIP := os.Getenv("NODE_IP")
	if nodeIP == "" {
		nodeIP = "127.0.0.1"
	}

	// 模拟的GPS数据 (洛杉矶地区)
	gpsLocations := map[string][2]float64{
		"uav-node-1": {34.052235, -118.243683}, // Downtown LA
		"uav-node-2": {34.019456, -118.491191}, // Santa Monica
		"uav-node-3": {34.147785, -118.144516}, // Pasadena
	}

	location := gpsLocations[nodeID]
	if location[0] == 0 && location[1] == 0 {
		location = [2]float64{34.052235, -118.243683} // 默认Downtown LA
	}

	return &GPSService{
		nodeID:   nodeID,
		nodeName: nodeName,
		nodeIP:   nodeIP,
		gpsData: GPSData{
			Latitude:   location[0],
			Longitude:  location[1],
			Altitude:   100.0,
			Battery:    85.0,
			NodeID:     nodeID,
			NodeName:   nodeName,
			NodeIP:     nodeIP,
			LastUpdate: time.Now(),
			Status:     "ready",
		},
	}
}

// StartGPSSimulation 启动GPS模拟
func (gs *GPSService) StartGPSSimulation() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 模拟GPS位置微调 (UAV移动)
			gs.updateGPSData()
			log.Printf("📍 GPS数据更新: [%.6f, %.6f], 电池: %.1f%%",
				gs.gpsData.Latitude, gs.gpsData.Longitude, gs.gpsData.Battery)
		}
	}
}

// updateGPSData 更新GPS数据 (模拟UAV移动和电池消耗)
func (gs *GPSService) updateGPSData() {
	// 模拟轻微位置变化 (UAV移动)
	gs.gpsData.Latitude += (randFloat() - 0.5) * 0.0001 // 约10米变化
	gs.gpsData.Longitude += (randFloat() - 0.5) * 0.0001

	// 模拟电池消耗
	gs.gpsData.Battery -= randFloat() * 0.5
	if gs.gpsData.Battery < 20 {
		gs.gpsData.Battery = 100 // 重置电池
	}

	// 模拟高度变化
	gs.gpsData.Altitude = 100 + (randFloat()-0.5)*20

	gs.gpsData.LastUpdate = time.Now()

	// 更新状态
	if gs.gpsData.Battery < 30 {
		gs.gpsData.Status = "low-battery"
	} else {
		gs.gpsData.Status = "ready"
	}
}

// randFloat 生成随机浮点数
func randFloat() float64 {
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

// setupRoutes 设置HTTP路由
func (gs *GPSService) setupRoutes() {
	http.HandleFunc("/health", gs.healthHandler)
	http.HandleFunc("/gps", gs.gpsHandler)
	http.HandleFunc("/status", gs.statusHandler)
	http.HandleFunc("/peers", gs.peersHandler)
}

// healthHandler 健康检查
func (gs *GPSService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"node_id": gs.nodeID,
		"timestamp": time.Now(),
	})
}

// gpsHandler GPS数据接口
func (gs *GPSService) gpsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   gs.gpsData,
	})
}

// statusHandler 状态接口
func (gs *GPSService) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"node": map[string]interface{}{
			"id":     gs.nodeID,
			"name":   gs.nodeName,
			"ip":     gs.nodeIP,
			"gps":    [2]float64{gs.gpsData.Latitude, gs.gpsData.Longitude},
			"battery": gs.gpsData.Battery,
			"status": gs.gpsData.Status,
		},
		"uptime": time.Since(time.Now().Add(-24 * time.Hour)).String(),
	})
}

// peersHandler 获取其他节点信息 (模拟)
func (gs *GPSService) peersHandler(w http.ResponseWriter, r *http.Request) {
	// 这里应该从其他节点获取信息，现在先返回模拟数据
	peers := []map[string]interface{}{
		{
			"id":   "uav-node-1",
			"name": "UAV Downtown LA",
			"ip":   "10.42.2.15",
			"gps":  [2]float64{34.052235, -118.243683},
			"battery": 85.5,
		},
		{
			"id":   "uav-node-2",
			"name": "UAV Santa Monica",
			"ip":   "10.42.1.25",
			"gps":  [2]float64{34.019456, -118.491191},
			"battery": 72.3,
		},
		{
			"id":   "uav-node-3",
			"name": "UAV Pasadena",
			"ip":   "10.42.1.26",
			"gps":  [2]float64{34.147785, -118.144516},
			"battery": 91.2,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"peers":  peers,
	})
}

func main() {
	log.Println("🚁 启动UAV GPS服务...")

	// 创建GPS服务
	gpsService := NewGPSService()

	// 设置路由
	gpsService.setupRoutes()

	// 启动GPS模拟
	go gpsService.StartGPSSimulation()

	// 启动HTTP服务器
	port := "9090"
	if envPort := os.Getenv("GPS_PORT"); envPort != "" {
		port = envPort
	}

	log.Printf("🌐 GPS服务启动在端口 %s", port)
	log.Printf("📍 节点信息: ID=%s, Name=%s, IP=%s",
		gpsService.nodeID, gpsService.nodeName, gpsService.nodeIP)
	log.Printf("📍 GPS位置: [%.6f, %.6f]",
		gpsService.gpsData.Latitude, gpsService.gpsData.Longitude)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}