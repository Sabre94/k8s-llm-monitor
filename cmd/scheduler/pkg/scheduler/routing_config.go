package scheduler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// RoutingConfigGenerator 路由配置生成器
type RoutingConfigGenerator struct {
	logger *logrus.Logger
}

// NewRoutingConfigGenerator 创建路由配置生成器
func NewRoutingConfigGenerator() *RoutingConfigGenerator {
	return &RoutingConfigGenerator{
		logger: Log().WithField("component", "routing-config-generator").Logger,
	}
}

// GenerateIstioConfig 生成Istio路由配置（为将来集成准备）
func (rcg *RoutingConfigGenerator) GenerateIstioConfig(matrix RoutingMatrix) *IstioRoutingConfig {
	config := &IstioRoutingConfig{
		APIVersion: "v1alpha1",
		Kind:       "RoutingConfiguration",
		Metadata: Metadata{
			Name:      "uav-routing-config",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "uav-routing",
				"app.kubernetes.io/component":  "routing-engine",
				"app.kubernetes.io/managed-by": "uav-scheduler",
			},
			Annotations: map[string]string{
				"generated-at": time.Now().Format(time.RFC3339),
				"generator":    "uav-scheduler-routing-engine",
			},
		},
		Spec: RoutingSpec{},
	}

	// 转换路由矩阵为Istio配置
	for sourceID, table := range matrix {
		serviceEntry := ServiceEntry{
			Name:      fmt.Sprintf("uav-%s", sourceID),
			Namespace: "default",
			Hosts:     []string{fmt.Sprintf("%s.uav.local", sourceID)},
			Addresses: []string{},
			Ports: []Port{{
				Number:   8080,
				Protocol: "HTTP",
				Name:     "http",
			}},
			Resolution: "DNS",
			Location:   "MESH_INTERNAL",
		}

		// 添加目标地址
		for _, route := range table {
			serviceEntry.Addresses = append(serviceEntry.Addresses, route.TargetIP)
		}

		config.Spec.ServiceEntries = append(config.Spec.ServiceEntries, serviceEntry)

		// 生成虚拟服务配置
		for targetID, route := range table {
			virtualService := VirtualService{
				Name:      fmt.Sprintf("route-%s-to-%s", sourceID, targetID),
				Namespace: "default",
				Hosts:     []string{fmt.Sprintf("%s.uav.local", sourceID)},
				Gateways:  []string{fmt.Sprintf("uav-gateway-%s", sourceID)},
				Http: []HTTPRoute{{
					Match: []HTTPMatchRequest{{
						Headers: map[string]MatchHeader{
							"x-target-node": {
								Exact: targetID,
							},
						},
					}},
					Route: []HTTPRouteDestination{{
						Destination: Destination{
							Host: fmt.Sprintf("%s.uav.local", targetID),
							Port: PortSelector{
								Number: 8080,
							},
						},
						Weight:    100,
						Headers:   &HeadersOperations{},
						Mirror:    nil,
						MirrorPercentage: nil,
					}},
					Fault:     nil,
					Timeout:   "10s",
					Retries:   &RetryPolicy{
						Attempts:      3,
						PerTryTimeout: "3s",
						RetryOn:       "5xx,connect-failure,refused-stream",
					},
				}},
			}

			config.Spec.VirtualServices = append(config.Spec.VirtualServices, virtualService)
		}
	}

	rcg.logger.WithFields(logrus.Fields{
		"service_entries":   len(config.Spec.ServiceEntries),
		"virtual_services":  len(config.Spec.VirtualServices),
		"source_nodes":      len(matrix),
	}).Info("Generated Istio routing configuration")

	return config
}

// GenerateSimpleRoutingConfig 生成简单的路由配置（用于demo）
func (rcg *RoutingConfigGenerator) GenerateSimpleRoutingConfig(matrix RoutingMatrix) *SimpleRoutingConfig {
	config := &SimpleRoutingConfig{
		Version:    "v1",
		GeneratedAt: time.Now(),
		Routes:     []Route{},
	}

	for sourceID, table := range matrix {
		for targetID, route := range table {
			simpleRoute := Route{
				Source:      sourceID,
				Target:      targetID,
				TargetIP:    route.TargetIP,
				SourcePort:  8080,
				TargetPort:  8080,
				Protocol:    "TCP",
				Priority:    route.Priority,
				Distance:    route.Distance,
				Score:       route.Score,
				Description: fmt.Sprintf("Route from %s to %s (distance: %.0fm, score: %.2f)",
					sourceID, targetID, route.Distance, route.Score),
			}
			config.Routes = append(config.Routes, simpleRoute)
		}
	}

	rcg.logger.WithFields(logrus.Fields{
		"total_routes": len(config.Routes),
		"source_nodes": len(matrix),
	}).Info("Generated simple routing configuration")

	return config
}

// GenerateKubernetesConfig 生成Kubernetes配置（用于demo展示）
func (rcg *RoutingConfigGenerator) GenerateKubernetesConfig(matrix RoutingMatrix) *K8sRoutingConfig {
	config := &K8sRoutingConfig{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: Metadata{
			Name:      "uav-routing-rules",
			Namespace: "default",
			Labels: map[string]string{
				"app": "uav-routing",
			},
		},
		Data: map[string]string{},
	}

	// 生成路由规则JSON
	routingRules := make(map[string]interface{})
	routingRules["version"] = "v1"
	routingRules["generated_at"] = time.Now().Format(time.RFC3339)
	routingRules["routes"] = config.Data

	for sourceID, table := range matrix {
		nodeRoutes := make([]map[string]interface{}, 0)
		for targetID, route := range table {
			nodeRoute := map[string]interface{}{
				"target":         targetID,
				"target_ip":      route.TargetIP,
				"target_port":    8080,
				"distance":       route.Distance,
				"score":          route.Score,
				"priority":       route.Priority,
				"battery_level":  route.BatteryLevel,
				"estimated_rtt":  route.EstimatedRTT,
				"last_update":    route.LastUpdate.Format(time.RFC3339),
			}
			nodeRoutes = append(nodeRoutes, nodeRoute)
		}
		routingRules[sourceID] = nodeRoutes
	}

	// 转换为JSON字符串
	rulesJSON, err := json.MarshalIndent(routingRules, "", "  ")
	if err != nil {
		rcg.logger.WithError(err).Error("Failed to marshal routing rules")
		return config
	}

	config.Data["routing-rules.json"] = string(rulesJSON)
	config.Data["routing-info.txt"] = rcg.generateRoutingInfoText(matrix)

	rcg.logger.Info("Generated Kubernetes ConfigMap for routing rules")

	return config
}

// generateRoutingInfoText 生成路由信息文本
func (rcg *RoutingConfigGenerator) generateRoutingInfoText(matrix RoutingMatrix) string {
	info := "UAV Routing Configuration\n"
	info += "========================\n\n"
	info += fmt.Sprintf("Generated at: %s\n", time.Now().Format(time.RFC3339))
	info += fmt.Sprintf("Total source nodes: %d\n\n", len(matrix))

	for sourceID, table := range matrix {
		info += fmt.Sprintf("Source Node: %s\n", sourceID)
		info += fmt.Sprintf("Available Routes: %d\n", len(table))

		for targetID, route := range table {
			info += fmt.Sprintf("  -> %s: %s (distance: %.0fm, score: %.2f, priority: %d)\n",
				targetID, route.TargetIP, route.Distance, route.Score, route.Priority)
		}
		info += "\n"
	}

	return info
}

// Istio配置结构体定义
type IstioRoutingConfig struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       RoutingSpec `json:"spec"`
}

type Metadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type RoutingSpec struct {
	ServiceEntries  []ServiceEntry   `json:"serviceEntries"`
	VirtualServices []VirtualService `json:"virtualServices"`
}

type ServiceEntry struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Hosts      []string `json:"hosts"`
	Addresses  []string `json:"addresses"`
	Ports      []Port  `json:"ports"`
	Resolution string  `json:"resolution"`
	Location   string  `json:"location"`
}

type Port struct {
	Number   int    `json:"number"`
	Protocol string `json:"protocol"`
	Name     string `json:"name"`
}

type VirtualService struct {
	Name      string     `json:"name"`
	Namespace string     `json:"namespace"`
	Hosts     []string   `json:"hosts"`
	Gateways  []string   `json:"gateways"`
	Http      []HTTPRoute `json:"http"`
}

type HTTPRoute struct {
	Match         []HTTPMatchRequest   `json:"match"`
	Route         []HTTPRouteDestination `json:"route"`
	Fault         *FaultInjection      `json:"fault,omitempty"`
	Timeout       string               `json:"timeout"`
	Retries       *RetryPolicy         `json:"retries,omitempty"`
}

type HTTPMatchRequest struct {
	Headers map[string]MatchHeader `json:"headers,omitempty"`
}

type MatchHeader struct {
	Exact string `json:"exact"`
}

type HTTPRouteDestination struct {
	Destination        Destination         `json:"destination"`
	Weight             int                `json:"weight"`
	Headers            *HeadersOperations `json:"headers,omitempty"`
	Mirror             *Destination       `json:"mirror,omitempty"`
	MirrorPercentage   *Percent           `json:"mirrorPercentage,omitempty"`
}

type Destination struct {
	Host string       `json:"host"`
	Port PortSelector `json:"port"`
}

type PortSelector struct {
	Number int `json:"number"`
}

type HeadersOperations struct {
	Request  map[string]string `json:"request,omitempty"`
	Response map[string]string `json:"response,omitempty"`
}

type RetryPolicy struct {
	Attempts      int    `json:"attempts"`
	PerTryTimeout string `json:"perTryTimeout"`
	RetryOn       string `json:"retryOn"`
}

type FaultInjection struct {
	Delay *DelayInjection `json:"delay,omitempty"`
	Abort *AbortInjection `json:"abort,omitempty"`
}

type DelayInjection struct {
	Percentage Percent `json:"percentage"`
	FixedDelay string  `json:"fixedDelay"`
}

type AbortInjection struct {
	Percentage Percent `json:"percentage"`
	HTTPStatus int     `json:"httpStatus"`
}

type Percent struct {
	Value float64 `json:"value"`
}

// 简单路由配置
type SimpleRoutingConfig struct {
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Routes      []Route   `json:"routes"`
}

type Route struct {
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	TargetIP    string    `json:"target_ip"`
	SourcePort  int       `json:"source_port"`
	TargetPort  int       `json:"target_port"`
	Protocol    string    `json:"protocol"`
	Priority    int       `json:"priority"`
	Distance    float64   `json:"distance"`
	Score       float64   `json:"score"`
	Description string    `json:"description"`
}

// Kubernetes配置
type K8sRoutingConfig struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   Metadata          `json:"metadata"`
	Data       map[string]string `json:"data"`
}