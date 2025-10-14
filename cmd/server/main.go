package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/k8s-llm-monitor/internal/config"
	"github.com/yourusername/k8s-llm-monitor/internal/k8s"
	"github.com/yourusername/k8s-llm-monitor/internal/metrics"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "config file path")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting K8s LLM Monitor...")
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("K8s Namespace: %s", cfg.K8s.Namespace)
	log.Printf("LLM Provider: %s", cfg.LLM.Provider)

	// 1. 初始化K8s客户端
	var k8sClient *k8s.Client
	var metricsManager *metrics.Manager

	if client, err := k8s.NewClient(&cfg.K8s); err != nil {
		log.Printf("Warning: Failed to create k8s client: %v", err)
		log.Printf("Running in development mode without K8s connection")
	} else {
		// 测试K8s连接
		if err := client.TestConnection(); err != nil {
			log.Printf("Warning: Failed to connect to k8s: %v", err)
			log.Printf("Running in development mode without K8s connection")
		} else {
			k8sClient = client
			log.Printf("Successfully connected to Kubernetes cluster")

			// 2. 初始化指标采集管理器
			if cfg.Metrics.Enabled {
				restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.K8s.Kubeconfig)
				if err != nil {
					log.Printf("Warning: Failed to create rest config: %v", err)
				} else {
					managerConfig := metrics.ManagerConfig{
						Namespaces:         cfg.Metrics.Namespaces,
						CollectInterval:    time.Duration(cfg.Metrics.CollectInterval) * time.Second,
						EnableNode:         cfg.Metrics.EnableNode,
						EnablePod:          cfg.Metrics.EnablePod,
						EnableNetwork:      cfg.Metrics.EnableNetwork,
						EnableCustom:       cfg.Metrics.EnableCustom,
						EnableUAV:          true, // 启用UAV指标采集
						NetworkMaxPairs:    5,    // 最多测试5对Pod
						NetworkTestTimeout: 10 * time.Second,
						K8sClient:          k8sClient, // 传递K8s client用于网络测试
					}

					manager, err := metrics.NewManager(restConfig, managerConfig)
					if err != nil {
						log.Printf("Warning: Failed to create metrics manager: %v", err)
					} else {
						metricsManager = manager
						log.Printf("Metrics manager created successfully")

						// 启动指标采集
						go func() {
							ctx := context.Background()
							if err := metricsManager.Start(ctx); err != nil {
								log.Printf("Metrics manager stopped: %v", err)
							}
						}()
						log.Printf("Metrics collection started (interval: %d seconds)", cfg.Metrics.CollectInterval)
					}
				}
			} else {
				log.Printf("Metrics collection is disabled in config")
			}
		}
	}

	// 3. 设置HTTP路由
	mux := http.NewServeMux()

	// 静态文件服务（Web界面）
	mux.Handle("/", http.FileServer(http.Dir("./web/")))

	// 健康检查接口
	mux.HandleFunc("/health", healthHandler)

	// 集群状态接口
	mux.HandleFunc("/api/v1/cluster/status", clusterStatusHandler(k8sClient))

	// Pod列表接口
	mux.HandleFunc("/api/v1/pods", podsHandler(k8sClient))

	// Pod通信分析接口
	mux.HandleFunc("/api/v1/analyze/pod-communication", podCommunicationHandler(k8sClient))

	// === 新增：指标相关接口 ===
	// 集群整体指标
	mux.HandleFunc("/api/v1/metrics/cluster", metricsClusterHandler(metricsManager))

	// 所有节点指标
	mux.HandleFunc("/api/v1/metrics/nodes", metricsNodesHandler(metricsManager))

	// 单个节点指标
	mux.HandleFunc("/api/v1/metrics/nodes/", metricsNodeHandler(metricsManager))

	// 所有Pod指标
	mux.HandleFunc("/api/v1/metrics/pods", metricsPodsHandler(metricsManager))

	// 完整快照
	mux.HandleFunc("/api/v1/metrics/snapshot", metricsSnapshotHandler(metricsManager))

	// 网络指标
	mux.HandleFunc("/api/v1/metrics/network", metricsNetworkHandler(metricsManager))

	// UAV指标
	mux.HandleFunc("/api/v1/metrics/uav", metricsUAVHandler(metricsManager))
	mux.HandleFunc("/api/v1/metrics/uav/", metricsUAVNodeHandler(metricsManager))

	// UAV数据上报接口
	mux.HandleFunc("/api/v1/uav/report", uavReportHandler(metricsManager, k8sClient))
	// UAV CRD数据
	mux.HandleFunc("/api/v1/crd/uav", uavCRDHandler(k8sClient))

	// 4. 创建HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// 5. 启动服务器 (在goroutine中)
	go func() {
		log.Printf("HTTP Server starting on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// 6. 优雅关闭处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// healthHandler 健康检查处理函数
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}
	json.NewEncoder(w).Encode(response)
}

// clusterStatusHandler 集群状态处理函数
func clusterStatusHandler(k8sClient *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// 检查K8s连接
		if k8sClient == nil {
			response := map[string]interface{}{
				"status":    "warning",
				"message":   "K8s client not available - running in development mode",
				"timestamp": time.Now().UTC(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// 获取集群信息
		clusterInfo, err := k8sClient.GetClusterInfo()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get cluster info: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"status":       "success",
			"cluster_info": clusterInfo,
			"timestamp":    time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// podCommunicationHandler Pod通信分析处理函数
func podCommunicationHandler(k8sClient *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// 检查K8s连接
		if k8sClient == nil {
			http.Error(w, "K8s client not available - running in development mode", http.StatusServiceUnavailable)
			return
		}

		// 解析请求参数
		var request struct {
			PodA string `json:"pod_a"`
			PodB string `json:"pod_b"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if request.PodA == "" || request.PodB == "" {
			http.Error(w, "pod_a and pod_b are required", http.StatusBadRequest)
			return
		}

		// 执行网络分析
		networkAnalyzer := k8s.NewNetworkAnalyzer(k8sClient)
		analysis, err := networkAnalyzer.AnalyzePodCommunication(r.Context(), request.PodA, request.PodB)
		if err != nil {
			http.Error(w, fmt.Sprintf("Analysis failed: %v", err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"status":    "success",
			"analysis":  analysis,
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// podsHandler Pod列表处理函数
func podsHandler(k8sClient *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// 检查K8s连接
		if k8sClient == nil {
			response := map[string]interface{}{
				"status":    "warning",
				"message":   "K8s client not available - running in development mode",
				"pods":      []interface{}{},
				"timestamp": time.Now().UTC(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// 获取所有监控命名空间的Pod
		allPods := []*models.PodInfo{}
		for _, namespace := range k8sClient.Namespaces() {
			pods, err := k8sClient.GetPods(namespace)
			if err != nil {
				log.Printf("Failed to get pods from namespace %s: %v", namespace, err)
				continue
			}
			allPods = append(allPods, pods...)
		}

		response := map[string]interface{}{
			"status":    "success",
			"pods":      allPods,
			"count":     len(allPods),
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// === 指标相关处理函数 ===

// metricsClusterHandler 集群整体指标处理函数
func metricsClusterHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		cluster := manager.GetClusterMetrics()

		response := map[string]interface{}{
			"status":    "success",
			"data":      cluster,
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsNodesHandler 所有节点指标处理函数
func metricsNodesHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		snapshot := manager.GetLatestSnapshot()

		response := map[string]interface{}{
			"status":    "success",
			"data":      snapshot.NodeMetrics,
			"count":     len(snapshot.NodeMetrics),
			"timestamp": snapshot.Timestamp,
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsNodeHandler 单个节点指标处理函数
func metricsNodeHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		// 从URL路径中提取节点名称
		nodeName := r.URL.Path[len("/api/v1/metrics/nodes/"):]
		if nodeName == "" {
			http.Error(w, "Node name is required", http.StatusBadRequest)
			return
		}

		nodeMetrics, err := manager.GetNodeMetrics(nodeName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Node not found: %v", err), http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"status":    "success",
			"data":      nodeMetrics,
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsPodsHandler 所有Pod指标处理函数
func metricsPodsHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		snapshot := manager.GetLatestSnapshot()

		response := map[string]interface{}{
			"status":    "success",
			"data":      snapshot.PodMetrics,
			"count":     len(snapshot.PodMetrics),
			"timestamp": snapshot.Timestamp,
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsSnapshotHandler 完整快照处理函数
func metricsSnapshotHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		snapshot := manager.GetLatestSnapshot()

		response := map[string]interface{}{
			"status": "success",
			"data":   snapshot,
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsNetworkHandler 网络指标处理函数
func metricsNetworkHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		networkMetrics := manager.GetNetworkMetrics()

		response := map[string]interface{}{
			"status":    "success",
			"data":      networkMetrics,
			"count":     len(networkMetrics),
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsUAVHandler 所有UAV指标处理函数
func metricsUAVHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		uavMetrics := manager.GetUAVMetrics()

		response := map[string]interface{}{
			"status":    "success",
			"data":      uavMetrics,
			"count":     len(uavMetrics),
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// metricsUAVNodeHandler 单个节点UAV指标处理函数
func metricsUAVNodeHandler(manager *metrics.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if manager == nil {
			http.Error(w, "Metrics manager not available", http.StatusServiceUnavailable)
			return
		}

		// 从URL路径中提取节点名称
		nodeName := r.URL.Path[len("/api/v1/metrics/uav/"):]
		if nodeName == "" {
			http.Error(w, "Node name is required", http.StatusBadRequest)
			return
		}

		uavMetric, exists := manager.GetSingleUAVMetrics(nodeName)
		if !exists {
			http.Error(w, fmt.Sprintf("UAV not found on node: %s", nodeName), http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"status":    "success",
			"data":      uavMetric,
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}

// uavReportHandler UAV状态上报处理函数
func uavReportHandler(manager *metrics.Manager, k8sClient *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var report models.UAVReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if report.NodeName == "" {
			http.Error(w, "node_name is required", http.StatusBadRequest)
			return
		}

		if report.UAVID == "" {
			report.UAVID = fmt.Sprintf("uav-%s", report.NodeName)
		}

		if report.Timestamp.IsZero() {
			report.Timestamp = time.Now().UTC()
		}

		if report.Source == "" {
			report.Source = "agent"
		}

		if report.Status == "" {
			report.Status = "active"
		}

		if manager != nil {
			manager.UpdateUAVReport(&report)
		} else {
			log.Printf("Metrics manager unavailable, skipping cache update for node %s", report.NodeName)
		}

		crdStatus := "unavailable"
		var crdError string
		if k8sClient != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			if err := k8sClient.UpsertUAVMetric(ctx, "", &report); err != nil {
				log.Printf("Failed to upsert UAVMetric for node %s: %v", report.NodeName, err)
				crdStatus = "error"
				crdError = err.Error()
			} else {
				crdStatus = "updated"
			}
		}

		response := map[string]interface{}{
			"status":     "success",
			"crd_status": crdStatus,
			"timestamp":  time.Now().UTC(),
			"node_name":  report.NodeName,
			"uav_id":     report.UAVID,
			"uav_status": report.Status,
		}

		if report.HeartbeatIntervalSeconds > 0 {
			response["heartbeat_interval_seconds"] = report.HeartbeatIntervalSeconds
		}

		if crdError != "" {
			response["message"] = crdError
		}

		json.NewEncoder(w).Encode(response)
	}
}

// uavCRDHandler UAV CRD数据处理函数
func uavCRDHandler(k8sClient *k8s.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if k8sClient == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "K8s client not available",
			})
			return
		}

		namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
		if strings.EqualFold(namespace, "all") {
			namespace = ""
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		crdData, err := k8sClient.ListUAVMetricsCRD(ctx, namespace)
		if err != nil {
			log.Printf("Failed to list UAV CRD data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		response := map[string]interface{}{
			"status":    "success",
			"count":     len(crdData),
			"data":      crdData,
			"timestamp": time.Now().UTC(),
		}

		json.NewEncoder(w).Encode(response)
	}
}
