package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// UAVScheduler 无人机调度器实现
type UAVScheduler struct {
	// 配置
	config *Config

	// 组件
	algorithms    map[string]Algorithm
	dataProviders map[string]DataProvider
	podBinder     PodBinder
	loadBalancer  *LoadBalancer

	// 状态
	status        *SchedulerStatus
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// 缓存
	cache         *ScheduleCache

	// 日志
	logger        *logrus.Logger
}

// NewUAVScheduler 创建无人机调度器
func NewUAVScheduler(config *Config) *UAVScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &UAVScheduler{
		config:        config,
		algorithms:    make(map[string]Algorithm),
		dataProviders: make(map[string]DataProvider),
		loadBalancer:  NewLoadBalancer(),
		status: &SchedulerStatus{
			Status:       "initialized",
			StartTime:    time.Now(),
			Algorithms:   make(map[string]AlgorithmStatus),
			DataProviders: make(map[string]ProviderStatus),
		},
		cache: NewScheduleCache(config.Cache.TTL, config.Cache.MaxSize),
		ctx:    ctx,
		cancel: cancel,
		logger: Log().WithField("scheduler", config.Name).Logger,
	}

	return scheduler
}

// Start 启动调度器
func (s *UAVScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Starting UAV scheduler")

	// 更新状态
	s.status.Status = "starting"

	// 启动数据提供者监听
	for name, provider := range s.dataProviders {
		s.wg.Add(1)
		go func(providerName string, p DataProvider) {
			defer s.wg.Done()
			s.startDataProvider(providerName, p)
		}(name, provider)
	}

	// 启动缓存清理任务
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.startCacheCleanup()
	}()

	// 启动负载均衡器更新任务
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.startLoadBalancerUpdate()
	}()

	// 更新状态
	s.status.Status = "running"
	s.logger.Info("UAV scheduler started successfully")

	return nil
}

// Stop 停止调度器
func (s *UAVScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Stopping UAV scheduler")

	// 取消上下文
	s.cancel()

	// 等待所有goroutine完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All components stopped gracefully")
	case <-time.After(time.Second * 30):
		s.logger.Warn("Timeout waiting for components to stop")
	}

	// 更新状态
	s.status.Status = "stopped"
	s.logger.Info("UAV scheduler stopped")

	return nil
}

// SchedulePod 调度单个Pod
func (s *UAVScheduler) SchedulePod(ctx context.Context, req *ScheduleRequest) (*ScheduleResult, error) {
	startTime := time.Now()

	s.logger.WithFields(logrus.Fields{
		"pod_name":      req.PodName,
		"algorithm":     req.AlgorithmName,
		"namespace":     req.PodNamespace,
	}).Info("Starting pod scheduling")

	// 验证请求
	if err := s.validateScheduleRequest(req); err != nil {
		return s.createErrorScheduleResult(req, startTime, err.Error(), ""), nil
	}

	// 检查缓存
	cacheKey := s.generateCacheKey(req)
	if cached := s.cache.Get(cacheKey); cached != nil {
		s.logger.WithField("pod_name", req.PodName).Debug("Returning cached schedule result")
		return cached, nil
	}

	// 获取数据提供者
	provider, err := s.getDataProvider("crd")
	if err != nil {
		return s.createErrorScheduleResult(req, startTime, fmt.Sprintf("Failed to get data provider: %v", err), ""), nil
	}

	// 获取UAV数据
	dataReq := &DataRequest{
		Source:    "uav-metrics-crd",
		Namespace: s.config.K8sConfig.Namespace,
		Fields:    []string{"id", "battery", "latency", "utilization", "gps", "radius", "status"},
	}

	dataResp, err := provider.FetchData(ctx, dataReq)
	if err != nil {
		return s.createErrorScheduleResult(req, startTime, fmt.Sprintf("Failed to fetch UAV data: %v", err), ""), nil
	}

	// 更新优化请求数据
	req.Requirements.UAVData = dataResp.UAVData

	// 获取算法
	algorithm, err := s.getAlgorithm(req.AlgorithmName)
	if err != nil {
		return s.createErrorScheduleResult(req, startTime, fmt.Sprintf("Failed to get algorithm: %v", err), ""), nil
	}

	// 执行优化
	optResult, err := algorithm.Optimize(ctx, req.Requirements)
	if err != nil {
		s.updateAlgorithmStatus(req.AlgorithmName, false, err)
		return s.createErrorScheduleResult(req, startTime, fmt.Sprintf("Optimization failed: %v", err), ""), nil
	}

	// 直接绑定节点集合（不选择单个节点）
	if s.podBinder != nil {
		if err := s.podBinder.BindNodeSet(ctx, req.PodName, req.PodNamespace, optResult.SelectedNodes, optResult); err != nil {
			return s.createErrorScheduleResult(req, startTime, fmt.Sprintf("Node set binding failed: %v", err), ""), nil
		}
	}

	// 创建成功结果
	result := &ScheduleResult{
		Success:             true,
		PodName:             req.PodName,
		AssignedNodeSet:     optResult.SelectedNodes,  // 返回整个节点集合
		AlgorithmName:       req.AlgorithmName,
		ExecutionTime:       time.Since(startTime),
		Reason:              "optimization_successful",
		Message:             fmt.Sprintf("Mission %s assigned to node set %v", req.PodName, optResult.SelectedNodes),
		OptimizationResult:  optResult,
	}

	// 更新统计信息
	s.updateAlgorithmStatus(req.AlgorithmName, true, nil)
	s.updateSchedulerStats(true)

	// 缓存结果
	s.cache.Set(cacheKey, result)

	s.logger.WithFields(logrus.Fields{
		"pod_name":       req.PodName,
		"node_set":       result.AssignedNodeSet,
		"node_count":     len(result.AssignedNodeSet),
		"execution_time": result.ExecutionTime,
		"coverage_ratio": optResult.CoverageRatio,
	}).Info("Mission scheduling completed successfully")

	return result, nil
}

// SchedulePodGroup 调度Pod组
func (s *UAVScheduler) SchedulePodGroup(ctx context.Context, req *ScheduleGroupRequest) (*ScheduleGroupResult, error) {
	startTime := time.Now()

	s.logger.WithFields(logrus.Fields{
		"pod_count":     len(req.Pods),
		"algorithm":     req.AlgorithmName,
	}).Info("Starting pod group scheduling")

	// 验证请求
	if err := s.validateScheduleGroupRequest(req); err != nil {
		return s.createErrorScheduleGroupResult(req, startTime, []string{err.Error()}), nil
	}

	// 获取数据提供者
	provider, err := s.getDataProvider("crd")
	if err != nil {
		return s.createErrorScheduleGroupResult(req, startTime, []string{fmt.Sprintf("Failed to get data provider: %v", err)}), nil
	}

	// 获取UAV数据
	dataReq := &DataRequest{
		Source:    "uav-metrics-crd",
		Namespace: s.config.K8sConfig.Namespace,
		Fields:    []string{"id", "battery", "latency", "utilization", "gps", "radius", "status"},
	}

	dataResp, err := provider.FetchData(ctx, dataReq)
	if err != nil {
		return s.createErrorScheduleGroupResult(req, startTime, []string{fmt.Sprintf("Failed to fetch UAV data: %v", err)}), nil
	}

	// 更新优化请求数据
	req.Requirements.UAVData = dataResp.UAVData

	// 获取算法
	algorithm, err := s.getAlgorithm(req.AlgorithmName)
	if err != nil {
		return s.createErrorScheduleGroupResult(req, startTime, []string{fmt.Sprintf("Failed to get algorithm: %v", err)}), nil
	}

	// 执行优化
	optResult, err := algorithm.Optimize(ctx, req.Requirements)
	if err != nil {
		s.updateAlgorithmStatus(req.AlgorithmName, false, err)
		return s.createErrorScheduleGroupResult(req, startTime, []string{fmt.Sprintf("Optimization failed: %v", err)}), nil
	}

	// 为Pod组分配节点
	nodeSelector := NewNodeSelector(optResult.SelectedNodes)
	bindings, err := nodeSelector.SelectNodesForPodGroup(req.Pods)
	if err != nil {
		return s.createErrorScheduleGroupResult(req, startTime, []string{fmt.Sprintf("Node assignment failed: %v", err)}), nil
	}

	// 绑定Pod组
	var errors []string
	if s.podBinder != nil {
		if err := s.podBinder.BindPodGroup(ctx, bindings); err != nil {
			errors = append(errors, fmt.Sprintf("Pod binding failed: %v", err))
		}
	}

	// 创建结果
	result := &ScheduleGroupResult{
		Success:            len(errors) == 0,
		TotalPods:          len(req.Pods),
		SuccessfulPods:     len(bindings),
		Bindings:           bindings,
		AlgorithmName:      req.AlgorithmName,
		ExecutionTime:      time.Since(startTime),
		OptimizationResult: optResult,
		Errors:             errors,
	}

	// 更新统计信息
	s.updateAlgorithmStatus(req.AlgorithmName, len(errors) == 0, nil)
	if len(errors) == 0 {
		s.updateSchedulerStats(true)
	} else {
		s.updateSchedulerStats(false)
	}

	s.logger.WithFields(logrus.Fields{
		"total_pods":      result.TotalPods,
		"successful_pods": result.SuccessfulPods,
		"execution_time":  result.ExecutionTime,
		"coverage_ratio":  optResult.CoverageRatio,
		"errors_count":    len(errors),
	}).Info("Pod group scheduling completed")

	return result, nil
}

// RegisterAlgorithm 注册算法
func (s *UAVScheduler) RegisterAlgorithm(algorithm Algorithm) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := algorithm.Name()
	if _, exists := s.algorithms[name]; exists {
		return fmt.Errorf("algorithm %s already registered", name)
	}

	s.algorithms[name] = algorithm
	s.status.Algorithms[name] = AlgorithmStatus{
		Name:      name,
		Enabled:   true,
		TotalRuns: 0,
	}

	s.logger.WithField("algorithm", name).Info("Algorithm registered")

	return nil
}

// RegisterDataProvider 注册数据提供者
func (s *UAVScheduler) RegisterDataProvider(provider DataProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := provider.Name()
	if _, exists := s.dataProviders[name]; exists {
		return fmt.Errorf("data provider %s already registered", name)
	}

	s.dataProviders[name] = provider
	s.status.DataProviders[name] = ProviderStatus{
		Name:      name,
		Enabled:   true,
		Connected: false,
		DataCount: 0,
	}

	s.logger.WithField("provider", name).Info("Data provider registered")

	return nil
}

// SetPodBinder 设置Pod绑定器
func (s *UAVScheduler) SetPodBinder(binder PodBinder) error {
	s.podBinder = binder
	s.logger.Info("Pod binder set")
	return nil
}

// GetStatus 获取调度器状态
func (s *UAVScheduler) GetStatus() *SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 复制状态以避免并发访问问题
	status := *s.status
	status.Algorithms = make(map[string]AlgorithmStatus)
	status.DataProviders = make(map[string]ProviderStatus)

	for k, v := range s.status.Algorithms {
		status.Algorithms[k] = v
	}

	for k, v := range s.status.DataProviders {
		status.DataProviders[k] = v
	}

	// 更新缓存状态
	status.CacheSize = s.cache.Size()
	status.CacheHitRate = s.cache.GetHitRate()

	return &status
}

// 内部方法

// startDataProvider 启动数据提供者
func (s *UAVScheduler) startDataProvider(name string, provider DataProvider) {
	s.logger.WithField("provider", name).Info("Starting data provider")

	// 数据变化回调
	callback := func(data *DataResponse) error {
		s.logger.WithFields(logrus.Fields{
			"provider": name,
			"count":    data.Count,
		}).Debug("Received data update")

		// 更新数据提供者状态
		s.mu.Lock()
		if providerStatus, exists := s.status.DataProviders[name]; exists {
			providerStatus.Connected = true
			providerStatus.LastUpdate = time.Now()
			providerStatus.DataCount = data.Count
			s.status.DataProviders[name] = providerStatus
		}
		s.mu.Unlock()

		// 清理调度缓存，因为数据已更新
		s.cache.Clear()

		return nil
	}

	// 开始监听数据变化
	if err := provider.WatchData(s.ctx, callback); err != nil {
		s.logger.WithFields(logrus.Fields{
			"provider": name,
			"error":    err.Error(),
		}).Error("Data provider failed to watch data")
	}
}

// startCacheCleanup 启动缓存清理任务
func (s *UAVScheduler) startCacheCleanup() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cache.CleanExpired()
		}
	}
}

// startLoadBalancerUpdate 启动负载均衡器更新任务
func (s *UAVScheduler) startLoadBalancerUpdate() {
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateLoadBalancer()
		}
	}
}

// updateLoadBalancer 更新负载均衡器
func (s *UAVScheduler) updateLoadBalancer() {
	provider, err := s.getDataProvider("crd")
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, time.Second*30)
	defer cancel()

	// 获取集群状态
	dataReq := &DataRequest{
		Source:    "cluster-status",
		Namespace: s.config.K8sConfig.Namespace,
	}

	dataResp, err := provider.FetchData(ctx, dataReq)
	if err != nil || dataResp.ClusterState == nil {
		return
	}

	// 更新负载均衡器（这里简化处理）
	// 实际应该从节点状态计算负载
	for _, uav := range dataResp.UAVData {
		s.loadBalancer.UpdateNodeLoad(uav.ID, 1, uav.Utilization, uav.Utilization)
	}

	// 清理过期数据
	s.loadBalancer.ClearExpiredLoads(time.Minute * 10)
}

// validateScheduleRequest 验证调度请求
func (s *UAVScheduler) validateScheduleRequest(req *ScheduleRequest) error {
	if req.PodName == "" {
		return fmt.Errorf("pod name is required")
	}

	if req.AlgorithmName == "" {
		return fmt.Errorf("algorithm name is required")
	}

	if req.Requirements == nil {
		return fmt.Errorf("optimization requirements are required")
	}

	return nil
}

// validateScheduleGroupRequest 验证调度组请求
func (s *UAVScheduler) validateScheduleGroupRequest(req *ScheduleGroupRequest) error {
	if len(req.Pods) == 0 {
		return fmt.Errorf("at least one pod is required")
	}

	if req.AlgorithmName == "" {
		return fmt.Errorf("algorithm name is required")
	}

	if req.Requirements == nil {
		return fmt.Errorf("optimization requirements are required")
	}

	for _, pod := range req.Pods {
		if pod.Name == "" {
			return fmt.Errorf("pod name is required")
		}
	}

	return nil
}

// getAlgorithm 获取算法
func (s *UAVScheduler) getAlgorithm(name string) (Algorithm, error) {
	algorithm, exists := s.algorithms[name]
	if !exists {
		return nil, ErrAlgorithmNotFound
	}
	return algorithm, nil
}

// getDataProvider 获取数据提供者
func (s *UAVScheduler) getDataProvider(name string) (DataProvider, error) {
	provider, exists := s.dataProviders[name]
	if !exists {
		return nil, ErrDataProviderNotFound
	}
	return provider, nil
}

// updateAlgorithmStatus 更新算法状态
func (s *UAVScheduler) updateAlgorithmStatus(name string, success bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if status, exists := s.status.Algorithms[name]; exists {
		status.TotalRuns++
		status.LastRun = time.Now()

		if err != nil {
			status.LastError = err.Error()
		}

		// 更新成功率（简单计算）
		if success {
			if status.TotalRuns == 1 {
				status.SuccessRate = 1.0
			} else {
				status.SuccessRate = (status.SuccessRate*float64(status.TotalRuns-1) + 1.0) / float64(status.TotalRuns)
			}
		} else {
			if status.TotalRuns == 1 {
				status.SuccessRate = 0.0
			} else {
				status.SuccessRate = (status.SuccessRate * float64(status.TotalRuns-1)) / float64(status.TotalRuns)
			}
		}

		s.status.Algorithms[name] = status
	}
}

// updateSchedulerStats 更新调度器统计信息
func (s *UAVScheduler) updateSchedulerStats(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.TotalSchedules++

	if success {
		if s.status.TotalSchedules == 1 {
			s.status.SuccessRate = 1.0
		} else {
			s.status.SuccessRate = (s.status.SuccessRate*float64(s.status.TotalSchedules-1) + 1.0) / float64(s.status.TotalSchedules)
		}
	} else {
		if s.status.TotalSchedules == 1 {
			s.status.SuccessRate = 0.0
		} else {
			s.status.SuccessRate = (s.status.SuccessRate * float64(s.status.TotalSchedules-1)) / float64(s.status.TotalSchedules)
		}
	}
}

// generateCacheKey 生成缓存键
func (s *UAVScheduler) generateCacheKey(req *ScheduleRequest) string {
	return fmt.Sprintf("pod:%s:%s:%s", req.PodName, req.PodNamespace, req.AlgorithmName)
}

// createErrorScheduleResult 创建错误调度结果
func (s *UAVScheduler) createErrorScheduleResult(req *ScheduleRequest, startTime time.Time, errorMsg, assignedNode string) *ScheduleResult {
	result := &ScheduleResult{
		Success:      false,
		PodName:      req.PodName,
		AssignedNode: assignedNode,
		AlgorithmName: req.AlgorithmName,
		ExecutionTime: time.Since(startTime),
		Reason:       "scheduling_failed",
		Message:      errorMsg,
	}

	// 更新统计信息
	s.updateSchedulerStats(false)

	return result
}

// createErrorScheduleGroupResult 创建错误调度组结果
func (s *UAVScheduler) createErrorScheduleGroupResult(req *ScheduleGroupRequest, startTime time.Time, errors []string) *ScheduleGroupResult {
	result := &ScheduleGroupResult{
		Success:       false,
		TotalPods:     len(req.Pods),
		SuccessfulPods: 0,
		AlgorithmName:  req.AlgorithmName,
		ExecutionTime:  time.Since(startTime),
		Errors:        errors,
	}

	// 更新统计信息
	s.updateSchedulerStats(false)

	return result
}