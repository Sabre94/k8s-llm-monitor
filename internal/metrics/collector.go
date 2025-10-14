package metrics

import (
	"context"
	"time"

	mt "github.com/yourusername/k8s-llm-monitor/pkg/metrics"
)

// Collector 指标采集器接口
type Collector interface {
	// Collect 采集指标
	Collect(ctx context.Context) error

	// GetLatestSnapshot 获取最新的指标快照
	GetLatestSnapshot() *mt.MetricsSnapshot

	// Start 启动定期采集
	Start(ctx context.Context, interval time.Duration) error

	// Stop 停止采集
	Stop() error
}

// NodeMetricsSource Node指标数据源接口
type NodeMetricsSource interface {
	// CollectNodeMetrics 采集所有节点指标
	CollectNodeMetrics(ctx context.Context) (map[string]*mt.NodeMetrics, error)

	// CollectSingleNodeMetrics 采集单个节点指标
	CollectSingleNodeMetrics(ctx context.Context, nodeName string) (*mt.NodeMetrics, error)
}

// PodMetricsSource Pod指标数据源接口
type PodMetricsSource interface {
	// CollectPodMetrics 采集所有Pod指标
	CollectPodMetrics(ctx context.Context) (map[string]*mt.PodMetrics, error)

	// CollectNamespacePodMetrics 采集指定namespace的Pod指标
	CollectNamespacePodMetrics(ctx context.Context, namespace string) (map[string]*mt.PodMetrics, error)
}

// NetworkMetricsSource 网络指标数据源接口
type NetworkMetricsSource interface {
	// CollectNetworkMetrics 采集网络指标
	CollectNetworkMetrics(ctx context.Context) ([]*mt.NetworkMetrics, error)

	// TestPodConnectivity 测试两个Pod之间的连通性
	TestPodConnectivity(ctx context.Context, sourcePod, targetPod string) (*mt.NetworkMetrics, error)
}

// CustomMetricsSource 自定义指标数据源接口（从CRD获取）
type CustomMetricsSource interface {
	// CollectCustomMetrics 采集自定义指标
	CollectCustomMetrics(ctx context.Context) (map[string]interface{}, error)
}

// UAVMetricsSource UAV指标数据源接口
type UAVMetricsSource interface {
	// CollectUAVMetrics 采集所有UAV指标
	CollectUAVMetrics(ctx context.Context) (map[string]interface{}, error)

	// CollectSingleUAVMetrics 采集单个UAV指标
	CollectSingleUAVMetrics(ctx context.Context, nodeName string) (interface{}, error)
}
