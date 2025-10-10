package k8s

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
)

// NewNetworkTestResult 创建网络测试结果
func NewNetworkTestResult(podA, podB string) *models.NetworkTestResult {
	return &models.NetworkTestResult{
		PodA:       podA,
		PodB:       podB,
		RTTResults: []models.RTTResult{},
		TestCount:  0,
	}
}

// RTTTester RTT测试器
type RTTTester struct {
	client *Client
	logger *logrus.Logger
}

// NewRTTTester 创建新的RTT测试器
func NewRTTTester(client *Client) *RTTTester {
	return &RTTTester{
		client: client,
		logger: client.logger,
	}
}

// TestPodConnectivity 测试Pod间连通性和RTT
func (rt *RTTTester) TestPodConnectivity(ctx context.Context, podA, podB string) (*models.NetworkTestResult, error) {
	// 解析Pod名称
	podANamespace, podAName := parsePodName(podA)
	podBNamespace, podBName := parsePodName(podB)

	// 获取Pod信息
	podAInfo, err := rt.getPodInfo(ctx, podANamespace, podAName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod A info: %w", err)
	}

	podBInfo, err := rt.getPodInfo(ctx, podBNamespace, podBName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod B info: %w", err)
	}

	// 初始化测试结果
	result := NewNetworkTestResult(podA, podB)

	// 执行多种测试
	rt.executePingTest(ctx, podAInfo, podBInfo, result)
	rt.executeHTTPTest(ctx, podAInfo, podBInfo, result)

	// 计算统计信息
	rt.calculateStats(result)

	return result, nil
}

// executePingTest 执行ping测试
func (rt *RTTTester) executePingTest(ctx context.Context, podA, podB *models.PodInfo, result *models.NetworkTestResult) {
	rt.logger.Infof("执行ping测试: %s -> %s", podA.Name, podB.Name)

	// 从Pod A ping Pod B的IP
	if podB.IP != "" {
		rttResult := rt.pingFromPod(ctx, podA, podB.IP)
		rttResult.Method = "ping"
		result.RTTResults = append(result.RTTResults, rttResult)
		result.TestCount++
	}

	// 反向测试：从Pod B ping Pod A的IP
	if podA.IP != "" {
		rttResult := rt.pingFromPod(ctx, podB, podA.IP)
		rttResult.Method = "ping_reverse"
		result.RTTResults = append(result.RTTResults, rttResult)
		result.TestCount++
	}
}

// executeHTTPTest 执行HTTP测试（如果Pod支持）
func (rt *RTTTester) executeHTTPTest(ctx context.Context, podA, podB *models.PodInfo, result *models.NetworkTestResult) {
	// 检查Pod B是否可能是HTTP服务（通过端口和标签）
	if rt.isHTTPService(podB) {
		rt.logger.Infof("执行HTTP测试: %s -> %s", podA.Name, podB.Name)

		// 尝试从Pod A访问Pod B的HTTP服务
		rttResult := rt.httpFromPod(ctx, podA, podB.IP, 80)
		rttResult.Method = "http"
		result.RTTResults = append(result.RTTResults, rttResult)
		result.TestCount++
	}
}

// pingFromPod 从指定Pod执行ping命令
func (rt *RTTTester) pingFromPod(ctx context.Context, pod *models.PodInfo, targetIP string) models.RTTResult {
	startTime := time.Now()

	// 构建ping命令
	cmd := fmt.Sprintf("ping -c 3 -W 5 %s", targetIP)

	// 在Pod中执行命令
	output, err := rt.executeCommandInPod(ctx, pod.Namespace, pod.Name, cmd)

	result := models.RTTResult{
		Timestamp: startTime,
		Success:   false,
		Method:    "ping",
	}

	if err != nil {
		result.ErrorMessage = fmt.Sprintf("执行ping命令失败: %v", err)
		rt.logger.Errorf("Ping from pod %s to %s failed: %v", pod.Name, targetIP, err)
		return result
	}

	// 解析ping输出
	rt.parsePingOutput(output, &result)

	rt.logger.Infof("Ping %s -> %s: RTT=%.2fms, 丢包率=%.1f%%",
		pod.Name, targetIP, result.RTT, result.PacketLoss)

	return result
}

// httpFromPod 从指定Pod执行HTTP请求
func (rt *RTTTester) httpFromPod(ctx context.Context, pod *models.PodInfo, targetIP string, port int) models.RTTResult {
	startTime := time.Now()

	// 构建curl命令
	cmd := fmt.Sprintf("curl -s -o /dev/null -w %%{time_total} -m 5 http://%s:%d", targetIP, port)

	// 在Pod中执行命令
	output, err := rt.executeCommandInPod(ctx, pod.Namespace, pod.Name, cmd)

	result := models.RTTResult{
		Timestamp: startTime,
		Success:   false,
		Method:    "http",
	}

	if err != nil {
		result.ErrorMessage = fmt.Sprintf("执行HTTP请求失败: %v", err)
		rt.logger.Errorf("HTTP from pod %s to %s:%d failed: %v", pod.Name, targetIP, port, err)
		return result
	}

	// 解析curl输出
	rt.parseHTTPOutput(output, &result)

	rt.logger.Infof("HTTP %s -> %s:%d: RTT=%.2fms",
		pod.Name, targetIP, port, result.RTT)

	return result
}

// executeCommandInPod 在Pod中执行命令
func (rt *RTTTester) executeCommandInPod(ctx context.Context, namespace, podName, command string) (string, error) {
	// 获取Pod信息以获取容器名称
	pod, err := rt.client.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod info: %w", err)
	}

	// 使用第一个容器的名称
	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found in pod %s", podName)
	}
	containerName := pod.Spec.Containers[0].Name

	// 构建执行请求
	req := rt.client.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		Param("container", containerName).
		Param("command", "sh").
		Param("command", "-c").
		Param("command", command).
		Param("stdout", "true").
		Param("stderr", "true")

	// 创建执行器
	config := rt.client.restConfig

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	// 执行命令并捕获输出
	var stdout, stderr strings.Builder
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return "", fmt.Errorf("command execution failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// parsePingOutput 解析ping命令输出
func (rt *RTTTester) parsePingOutput(output string, result *models.RTTResult) {
	scanner := bufio.NewScanner(strings.NewReader(output))

	var rttSum float64
	var rttCount int
	var packetLoss float64

	for scanner.Scan() {
		line := scanner.Text()

		// 解析RTT信息：例如 "64 bytes from 10.244.0.4: icmp_seq=1 ttl=64 time=0.123 ms"
		if strings.Contains(line, "time=") && strings.Contains(line, "ms") {
			rtt := rt.extractRTTFromPingLine(line)
			if rtt > 0 {
				rttSum += rtt
				rttCount++
			}
		}

		// 解析丢包信息：例如 "3 packets transmitted, 3 received, 0% packet loss"
		if strings.Contains(line, "packet loss") {
			packetLoss = rt.extractPacketLossFromPingLine(line)
		}
	}

	if rttCount > 0 {
		result.RTT = rttSum / float64(rttCount)
		result.Success = true
	}

	result.PacketLoss = packetLoss
}

// parseHTTPOutput 解析curl输出
func (rt *RTTTester) parseHTTPOutput(output string, result *models.RTTResult) {
	if output == "" {
		return
	}

	// curl的-w参数直接输出总时间（秒）
	if rtt, err := strconv.ParseFloat(strings.TrimSpace(output), 64); err == nil {
		result.RTT = rtt * 1000 // 转换为毫秒
		result.Success = true
		result.PacketLoss = 0
	}
}

// extractRTTFromPingLine 从ping输出行中提取RTT
func (rt *RTTTester) extractRTTFromPingLine(line string) float64 {
	parts := strings.Split(line, "time=")
	if len(parts) < 2 {
		return 0
	}

	timePart := strings.Split(parts[1], " ")[0]
	timePart = strings.TrimSuffix(timePart, "ms")

	rtt, err := strconv.ParseFloat(timePart, 64)
	if err != nil {
		return 0
	}

	return rtt
}

// extractPacketLossFromPingLine 从ping输出行中提取丢包率
func (rt *RTTTester) extractPacketLossFromPingLine(line string) float64 {
	parts := strings.Split(line, " ")
	for _, part := range parts {
		if strings.Contains(part, "%") {
			lossStr := strings.TrimSuffix(part, "%")
			loss, err := strconv.ParseFloat(lossStr, 64)
			if err == nil {
				return loss
			}
		}
	}
	return 0
}

// isHTTPService 判断Pod是否可能是HTTP服务
func (rt *RTTTester) isHTTPService(pod *models.PodInfo) bool {
	// 检查标签
	if app, ok := pod.Labels["app"]; ok {
		httpApps := []string{"nginx", "httpd", "apache", "web"}
		for _, httpApp := range httpApps {
			if strings.Contains(strings.ToLower(app), httpApp) {
				return true
			}
		}
	}

	// 检查容器镜像
	for _, container := range pod.Containers {
		image := strings.ToLower(container.Image)
		if strings.Contains(image, "nginx") || strings.Contains(image, "httpd") {
			return true
		}
	}

	return false
}

// calculateStats 计算测试统计信息
func (rt *RTTTester) calculateStats(result *models.NetworkTestResult) {
	if len(result.RTTResults) == 0 {
		result.AverageRTT = 0
		result.SuccessRate = 0
		result.Latency = "unknown"
		return
	}

	var totalRTT float64
	var successCount int

	for _, rttResult := range result.RTTResults {
		if rttResult.Success {
			totalRTT += rttResult.RTT
			successCount++
		}
	}

	if successCount > 0 {
		result.AverageRTT = totalRTT / float64(successCount)
		result.SuccessRate = float64(successCount) / float64(len(result.RTTResults)) * 100
	} else {
		result.AverageRTT = 0
		result.SuccessRate = 0
	}

	// 评估延迟等级
	result.Latency = rt.assessLatency(result.AverageRTT)
}

// assessLatency 评估延迟等级
func (rt *RTTTester) assessLatency(rtt float64) string {
	switch {
	case rtt == 0:
		return "unknown"
	case rtt < 1:
		return "excellent"
	case rtt < 5:
		return "good"
	case rtt < 50:
		return "fair"
	case rtt < 100:
		return "poor"
	default:
		return "very_poor"
	}
}

// getPodInfo 获取Pod信息（添加到client.go的公共方法）
func (rt *RTTTester) getPodInfo(ctx context.Context, namespace, name string) (*models.PodInfo, error) {
	pod, err := rt.client.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return rt.client.convertPodToModel(pod), nil
}
