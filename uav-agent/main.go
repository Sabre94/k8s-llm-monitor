package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/k8s-llm-monitor/pkg/models"
	"github.com/yourusername/k8s-llm-monitor/pkg/uav"
)

func main() {
	var port int
	var masterURL string
	var reportInterval time.Duration

	flag.IntVar(&port, "port", 9090, "HTTP server port")
	flag.StringVar(&masterURL, "master-url", "", "Master server base URL for UAV reports")
	flag.DurationVar(&reportInterval, "report-interval", 0, "Interval for uploading UAV telemetry")
	flag.Parse()

	if masterURL == "" {
		masterURL = os.Getenv("MASTER_URL")
	}

	masterURL = strings.TrimSpace(masterURL)
	if masterURL != "" && !strings.HasPrefix(masterURL, "http://") && !strings.HasPrefix(masterURL, "https://") {
		masterURL = "http://" + masterURL
	}

	if reportInterval <= 0 {
		if envInterval := strings.TrimSpace(os.Getenv("REPORT_INTERVAL")); envInterval != "" {
			if parsed, err := time.ParseDuration(envInterval); err == nil {
				reportInterval = parsed
			} else {
				log.Printf("Invalid REPORT_INTERVAL value %q: %v", envInterval, err)
			}
		}
	}

	if reportInterval <= 0 {
		reportInterval = 15 * time.Second
	}

	// 获取节点信息
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "unknown-node"
	}

	nodeIP := os.Getenv("NODE_IP")
	if nodeIP == "" {
		nodeIP = "unknown-ip"
	}

	// 生成UAV ID（基于节点名）
	uavID := fmt.Sprintf("UAV-%s", nodeName)

	log.Printf("Starting UAV Agent...")
	log.Printf("UAV ID: %s", uavID)
	log.Printf("Node: %s", nodeName)
	log.Printf("IP: %s", nodeIP)
	log.Printf("Port: %d", port)

	// 创建MAVLink模拟器
	simulator := uav.NewMAVLinkSimulator(uavID, nodeName)
	simulator.Start()
	log.Printf("MAVLink simulator started")

	// 设置HTTP路由
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"uav_id":    uavID,
			"node_name": nodeName,
			"node_ip":   nodeIP,
			"timestamp": time.Now(),
		})
	})

	// 获取完整状态
	mux.HandleFunc("/api/v1/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		state := simulator.GetState()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   state,
		})
	})

	// 获取GPS数据
	mux.HandleFunc("/api/v1/gps", func(w http.ResponseWriter, r *http.Request) {
		state := simulator.GetState()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   state.GPS,
		})
	})

	// 获取姿态数据
	mux.HandleFunc("/api/v1/attitude", func(w http.ResponseWriter, r *http.Request) {
		state := simulator.GetState()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   state.Attitude,
		})
	})

	// 获取电池数据
	mux.HandleFunc("/api/v1/battery", func(w http.ResponseWriter, r *http.Request) {
		state := simulator.GetState()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   state.Battery,
		})
	})

	// 获取飞行数据
	mux.HandleFunc("/api/v1/flight", func(w http.ResponseWriter, r *http.Request) {
		state := simulator.GetState()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   state.Flight,
		})
	})

	// 控制接口 - 解锁
	mux.HandleFunc("/api/v1/command/arm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := simulator.Arm()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Armed successfully",
		})
	})

	// 控制接口 - 上锁
	mux.HandleFunc("/api/v1/command/disarm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		simulator.Disarm()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Disarmed successfully",
		})
	})

	// 控制接口 - 起飞
	mux.HandleFunc("/api/v1/command/takeoff", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Altitude float64 `json:"altitude"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Altitude == 0 {
			req.Altitude = 50.0 // 默认高度50米
		}

		simulator.TakeOff(req.Altitude)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Taking off to %.1fm", req.Altitude),
		})
	})

	// 控制接口 - 降落
	mux.HandleFunc("/api/v1/command/land", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		simulator.Land()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Landing initiated",
		})
	})

	// 控制接口 - 返航
	mux.HandleFunc("/api/v1/command/rtl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		simulator.ReturnToLaunch()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Returning to launch",
		})
	})

	// 控制接口 - 设置飞行模式
	mux.HandleFunc("/api/v1/command/mode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Mode string `json:"mode"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		simulator.SetFlightMode(req.Mode)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Flight mode set to %s", req.Mode),
		})
	})

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("HTTP Server starting on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	reportCtx, reportCancel := context.WithCancel(context.Background())
	defer reportCancel()

	if masterURL != "" {
		log.Printf("Telemetry reporting enabled: %s (interval %s)", masterURL, reportInterval)
		go startUAVReportLoop(reportCtx, masterURL, reportInterval, nodeName, nodeIP, uavID, simulator)
	} else {
		log.Printf("Master URL not configured. Telemetry reporting disabled")
	}

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down UAV agent...")
	reportCancel()

	simulator.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("UAV agent exited")
}

func startUAVReportLoop(ctx context.Context, masterURL string, interval time.Duration, nodeName, nodeIP, uavID string, simulator *uav.MAVLinkSimulator) {
	if interval <= 0 {
		interval = 15 * time.Second
	}

	endpoint := strings.TrimRight(masterURL, "/") + "/api/v1/uav/report"
	heartbeatSeconds := int(interval.Seconds())
	if heartbeatSeconds <= 0 {
		heartbeatSeconds = 15
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	sendReport := func() {
		if err := ctx.Err(); err != nil {
			return
		}

		reportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		state := simulator.GetState()
		stateCopy := state

		report := models.UAVReport{
			NodeName:                 nodeName,
			NodeIP:                   nodeIP,
			UAVID:                    uavID,
			Source:                   "agent",
			Status:                   "active",
			Timestamp:                time.Now().UTC(),
			HeartbeatIntervalSeconds: heartbeatSeconds,
			State:                    &stateCopy,
			Metadata: map[string]string{
				"agent": "go-uav-agent",
			},
		}

		if report.NodeName == "" {
			report.NodeName = "unknown-node"
		}
		if report.UAVID == "" {
			report.UAVID = fmt.Sprintf("UAV-%s", report.NodeName)
		}

		payload, err := json.Marshal(report)
		if err != nil {
			log.Printf("Failed to marshal UAV report: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(reportCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			log.Printf("Failed to create UAV report request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Failed to send UAV report to %s: %v", endpoint, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			log.Printf("UAV report rejected (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
			return
		}

		log.Printf("UAV report delivered (status %s)", resp.Status)
	}

	sendReport()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("UAV report loop stopped")
			return
		case <-ticker.C:
			sendReport()
		}
	}
}
