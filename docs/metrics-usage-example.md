# 指标系统使用示例

## 快速开始

### 1. 启动指标采集

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/yourusername/k8s-llm-monitor/internal/config"
    "github.com/yourusername/k8s-llm-monitor/internal/metrics"
    "k8s.io/client-go/tools/clientcmd"
)

func main() {
    // 加载配置
    cfg, err := config.Load("./configs/config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 创建 K8s REST 配置
    restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.K8s.Kubeconfig)
    if err != nil {
        log.Fatalf("Failed to create K8s config: %v", err)
    }

    // 创建指标管理器
    managerConfig := metrics.ManagerConfig{
        Namespaces:      cfg.Metrics.Namespaces,
        CollectInterval: time.Duration(cfg.Metrics.CollectInterval) * time.Second,
        EnableNode:      cfg.Metrics.EnableNode,
        EnablePod:       cfg.Metrics.EnablePod,
        EnableNetwork:   cfg.Metrics.EnableNetwork,
        EnableCustom:    cfg.Metrics.EnableCustom,
    }

    manager, err := metrics.NewManager(restConfig, managerConfig)
    if err != nil {
        log.Fatalf("Failed to create metrics manager: %v", err)
    }

    // 启动定期采集
    ctx := context.Background()
    go func() {
        if err := manager.Start(ctx); err != nil {
            log.Printf("Metrics manager stopped: %v", err)
        }
    }()

    // 等待第一次采集完成
    time.Sleep(2 * time.Second)

    // 使用指标数据
    printClusterStatus(manager)
}

func printClusterStatus(manager *metrics.Manager) {
    // 获取集群整体指标
    cluster := manager.GetClusterMetrics()

    fmt.Printf("=== 集群状态 ===\n")
    fmt.Printf("健康状态: %s\n", cluster.HealthStatus)
    fmt.Printf("节点总数: %d (健康: %d)\n", cluster.TotalNodes, cluster.HealthyNodes)
    fmt.Printf("Pod总数: %d (运行中: %d)\n", cluster.TotalPods, cluster.RunningPods)
    fmt.Printf("CPU使用率: %.2f%%\n", cluster.CPUUsageRate)
    fmt.Printf("内存使用率: %.2f%%\n", cluster.MemoryUsageRate)

    if len(cluster.Issues) > 0 {
        fmt.Printf("\n⚠️  存在问题:\n")
        for _, issue := range cluster.Issues {
            fmt.Printf("  - %s\n", issue)
        }
    }
}
```

---

## 2. 查询特定节点指标

```go
func checkNodeHealth(manager *metrics.Manager, nodeName string) {
    nodeMetrics, err := manager.GetNodeMetrics(nodeName)
    if err != nil {
        log.Printf("Failed to get node metrics: %v", err)
        return
    }

    fmt.Printf("\n=== 节点 %s ===\n", nodeName)
    fmt.Printf("健康状态: %v\n", nodeMetrics.Healthy)
    fmt.Printf("CPU使用率: %.2f%% (%d/%d 毫核)\n",
        nodeMetrics.CPUUsageRate,
        nodeMetrics.CPUUsage,
        nodeMetrics.CPUCapacity)
    fmt.Printf("内存使用率: %.2f%% (%.2f GB / %.2f GB)\n",
        nodeMetrics.MemoryUsageRate,
        float64(nodeMetrics.MemoryUsage)/1024/1024/1024,
        float64(nodeMetrics.MemoryCapacity)/1024/1024/1024)

    // 检查资源压力
    if nodeMetrics.IsUnderPressure() {
        fmt.Printf("⚠️  节点处于资源压力状态!\n")
    }

    // 显示可用资源
    cpuCores, memGB, diskGB := nodeMetrics.GetAvailableResources()
    fmt.Printf("可用资源: CPU=%.2f核, 内存=%.2fGB, 磁盘=%.2fGB\n",
        cpuCores, memGB, diskGB)

    // 显示异常条件
    if len(nodeMetrics.Conditions) > 0 {
        fmt.Printf("异常条件:\n")
        for _, condition := range nodeMetrics.Conditions {
            fmt.Printf("  - %s\n", condition)
        }
    }
}
```

---

## 3. 查询 Pod 指标

```go
func checkPodResources(manager *metrics.Manager, namespace, podName string) {
    podMetrics, err := manager.GetPodMetrics(namespace, podName)
    if err != nil {
        log.Printf("Failed to get pod metrics: %v", err)
        return
    }

    fmt.Printf("\n=== Pod %s/%s ===\n", namespace, podName)
    fmt.Printf("状态: %s (就绪: %v)\n", podMetrics.Phase, podMetrics.Ready)
    fmt.Printf("所在节点: %s\n", podMetrics.NodeName)
    fmt.Printf("重启次数: %d\n", podMetrics.Restarts)

    fmt.Printf("\nCPU:\n")
    fmt.Printf("  使用量: %d 毫核\n", podMetrics.CPUUsage)
    fmt.Printf("  Request: %d 毫核\n", podMetrics.CPURequest)
    fmt.Printf("  Limit: %d 毫核\n", podMetrics.CPULimit)
    fmt.Printf("  使用率(相对Limit): %.2f%%\n", podMetrics.CPUUsageRate)

    fmt.Printf("\n内存:\n")
    fmt.Printf("  使用量: %.2f MB\n", float64(podMetrics.MemoryUsage)/1024/1024)
    fmt.Printf("  Request: %.2f MB\n", float64(podMetrics.MemoryRequest)/1024/1024)
    fmt.Printf("  Limit: %.2f MB\n", float64(podMetrics.MemoryLimit)/1024/1024)
    fmt.Printf("  使用率(相对Limit): %.2f%%\n", podMetrics.MemoryUsageRate)

    // 检查是否接近限制
    if podMetrics.IsOverLimit() {
        fmt.Printf("\n⚠️  Pod 资源使用接近或超过限制!\n")
    }

    // 显示Container级别指标
    if len(podMetrics.Containers) > 0 {
        fmt.Printf("\nContainer 指标:\n")
        for _, container := range podMetrics.Containers {
            fmt.Printf("  %s:\n", container.Name)
            fmt.Printf("    CPU: %d 毫核\n", container.CPUUsage)
            fmt.Printf("    Memory: %.2f MB\n", float64(container.MemoryUsage)/1024/1024)
        }
    }
}
```

---

## 4. 分析集群资源分布

```go
func analyzeClusterResources(manager *metrics.Manager) {
    snapshot := manager.GetLatestSnapshot()

    fmt.Printf("\n=== 集群资源分析 ===\n")
    fmt.Printf("采集时间: %s\n\n", snapshot.Timestamp.Format("2006-01-02 15:04:05"))

    // 分析节点
    fmt.Printf("节点资源使用情况:\n")
    var totalCPU, usedCPU, totalMem, usedMem int64
    var unhealthyNodes []string

    for nodeName, node := range snapshot.NodeMetrics {
        totalCPU += node.CPUCapacity
        usedCPU += node.CPUUsage
        totalMem += node.MemoryCapacity
        usedMem += node.MemoryUsage

        fmt.Printf("  %s: CPU=%.1f%%, MEM=%.1f%%",
            nodeName, node.CPUUsageRate, node.MemoryUsageRate)

        if !node.Healthy {
            fmt.Printf(" ⚠️ 不健康")
            unhealthyNodes = append(unhealthyNodes, nodeName)
        }
        fmt.Printf("\n")
    }

    fmt.Printf("\n整体资源使用:\n")
    fmt.Printf("  CPU: %.1f%% (%d/%d 毫核)\n",
        float64(usedCPU)/float64(totalCPU)*100, usedCPU, totalCPU)
    fmt.Printf("  内存: %.1f%% (%.2f/%.2f GB)\n",
        float64(usedMem)/float64(totalMem)*100,
        float64(usedMem)/1024/1024/1024,
        float64(totalMem)/1024/1024/1024)

    // 找出资源使用最高的Pod
    fmt.Printf("\nCPU使用最高的5个Pod:\n")
    topCPUPods := getTopResourcePods(snapshot.PodMetrics, "cpu", 5)
    for i, pod := range topCPUPods {
        fmt.Printf("  %d. %s/%s: %d 毫核\n",
            i+1, pod.Namespace, pod.PodName, pod.CPUUsage)
    }

    fmt.Printf("\n内存使用最高的5个Pod:\n")
    topMemPods := getTopResourcePods(snapshot.PodMetrics, "memory", 5)
    for i, pod := range topMemPods {
        fmt.Printf("  %d. %s/%s: %.2f MB\n",
            i+1, pod.Namespace, pod.PodName,
            float64(pod.MemoryUsage)/1024/1024)
    }
}

func getTopResourcePods(podMetrics map[string]*metrics.PodMetrics, resourceType string, limit int) []*metrics.PodMetrics {
    // 实现排序逻辑，返回top N的Pod
    // (此处简化，实际需要实现排序)
    result := []*metrics.PodMetrics{}
    // ... 排序逻辑
    return result
}
```

---

## 5. 为 LLM 准备上下文

```go
func prepareLLMContext(manager *metrics.Manager) string {
    snapshot := manager.GetLatestSnapshot()
    cluster := snapshot.ClusterMetrics

    // 构建 LLM 分析所需的上下文
    context := fmt.Sprintf(`
集群状态概览:
- 健康状态: %s
- 节点: %d个 (健康: %d个)
- Pod: %d个 (运行中: %d个)
- CPU使用率: %.1f%%
- 内存使用率: %.1f%%

节点详情:
`, cluster.HealthStatus, cluster.TotalNodes, cluster.HealthyNodes,
       cluster.TotalPods, cluster.RunningPods,
       cluster.CPUUsageRate, cluster.MemoryUsageRate)

    for nodeName, node := range snapshot.NodeMetrics {
        context += fmt.Sprintf("- %s: CPU=%.1f%%, MEM=%.1f%%",
            nodeName, node.CPUUsageRate, node.MemoryUsageRate)

        if node.IsUnderPressure() {
            context += " [资源压力]"
        }
        if !node.Healthy {
            context += " [不健康]"
        }
        context += "\n"
    }

    // 添加问题Pod信息
    context += "\n问题Pod:\n"
    for key, pod := range snapshot.PodMetrics {
        if pod.Phase != "Running" || !pod.Ready || pod.Restarts > 5 {
            context += fmt.Sprintf("- %s: 状态=%s, 就绪=%v, 重启=%d次\n",
                key, pod.Phase, pod.Ready, pod.Restarts)
        }
        if pod.IsOverLimit() {
            context += fmt.Sprintf("- %s: 资源使用接近限制\n", key)
        }
    }

    return context
}

// 示例：将上下文发送给LLM分析
func analyzewithLLM(manager *metrics.Manager, question string) {
    context := prepareLLMContext(manager)

    // 构建 LLM 提示
    prompt := fmt.Sprintf(`
基于以下Kubernetes集群指标数据:

%s

请回答用户问题: %s
`, context, question)

    // 调用 LLM API
    // response := callLLMAPI(prompt)
    // fmt.Println(response)
}
```

---

## 6. HTTP API 集成示例

```go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/yourusername/k8s-llm-monitor/internal/metrics"
)

var metricsManager *metrics.Manager

func setupMetricsRoutes(mux *http.ServeMux) {
    // 集群整体指标
    mux.HandleFunc("/api/v1/metrics/cluster", handleClusterMetrics)

    // 所有节点指标
    mux.HandleFunc("/api/v1/metrics/nodes", handleNodesMetrics)

    // 单个节点指标
    mux.HandleFunc("/api/v1/metrics/nodes/", handleNodeMetrics)

    // 所有Pod指标
    mux.HandleFunc("/api/v1/metrics/pods", handlePodsMetrics)

    // 完整快照
    mux.HandleFunc("/api/v1/metrics/snapshot", handleSnapshot)
}

func handleClusterMetrics(w http.ResponseWriter, r *http.Request) {
    cluster := metricsManager.GetClusterMetrics()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "data":   cluster,
    })
}

func handleNodesMetrics(w http.ResponseWriter, r *http.Request) {
    snapshot := metricsManager.GetLatestSnapshot()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "data":   snapshot.NodeMetrics,
        "count":  len(snapshot.NodeMetrics),
    })
}

func handleSnapshot(w http.ResponseWriter, r *http.Request) {
    snapshot := metricsManager.GetLatestSnapshot()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "data":   snapshot,
    })
}
```

---

## 7. 完整示例程序

查看 `cmd/demos/metrics-demo/main.go` 获取完整的示例代码。

运行示例:
```bash
go run cmd/demos/metrics-demo/main.go
```

---

## 注意事项

1. **Metrics Server 依赖**
   - 需要在集群中部署 Metrics Server
   - 如果 Metrics Server 不可用，仍会返回基础信息（容量、标签等），但使用量数据为0

2. **权限要求**
   - 需要有读取 nodes 和 pods 的权限
   - 需要有读取 metrics.k8s.io 的权限

3. **性能考虑**
   - 采集间隔建议不低于 30 秒
   - 大规模集群建议增加采集间隔
   - 使用缓存机制避免频繁查询

4. **错误处理**
   - 采集失败不会中断服务
   - 会记录错误日志并继续使用缓存数据
