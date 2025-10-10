package k8s

import (
	"time"

	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// convertPodToModel 将K8s Pod对象转换为模型
func (c *Client) convertPodToModel(pod *corev1.Pod) *models.PodInfo {
	podInfo := &models.PodInfo{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
		NodeName:  pod.Spec.NodeName,
		IP:        pod.Status.PodIP,
		Labels:    pod.Labels,
		StartTime: getCreationTime(pod),
	}

	// 转换容器信息
	for _, container := range pod.Spec.Containers {
		containerStatus := getContainerStatus(pod.Status.ContainerStatuses, container.Name)

		containerInfo := models.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			State: getContainerState(containerStatus),
			Ready: containerStatus != nil && containerStatus.Ready,
			Env:   make(map[string]string),
		}

		// 提取环境变量（只提取非敏感的）
		for _, envVar := range container.Env {
			if envVar.Value != "" {
				containerInfo.Env[envVar.Name] = envVar.Value
			}
		}

		podInfo.Containers = append(podInfo.Containers, containerInfo)
	}

	return podInfo
}

// convertServiceToModel 将K8s Service对象转换为模型
func (c *Client) convertServiceToModel(svc *corev1.Service) *models.ServiceInfo {
	serviceInfo := &models.ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      string(svc.Spec.Type),
		ClusterIP: svc.Spec.ClusterIP,
		Selector:  svc.Spec.Selector,
	}

	// 转换端口信息
	for _, port := range svc.Spec.Ports {
		servicePort := models.ServicePort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: string(port.Protocol),
		}
		serviceInfo.Ports = append(serviceInfo.Ports, servicePort)
	}

	return serviceInfo
}

// convertEventToModel 将K8s Event对象转换为模型
func (c *Client) convertEventToModel(event *corev1.Event) *models.EventInfo {
	return &models.EventInfo{
		Type:      event.Type,
		Reason:    event.Reason,
		Message:   event.Message,
		Source:    event.Source.Component,
		Timestamp: event.LastTimestamp.Time,
		Count:     event.Count,
	}
}

// getContainerStatus 获取容器状态
func getContainerStatus(statuses []corev1.ContainerStatus, name string) *corev1.ContainerStatus {
	for _, status := range statuses {
		if status.Name == name {
			return &status
		}
	}
	return nil
}

// getContainerState 获取容器状态字符串
func getContainerState(status *corev1.ContainerStatus) string {
	if status == nil {
		return "Unknown"
	}

	if status.State.Running != nil {
		return "Running"
	}
	if status.State.Waiting != nil {
		return "Waiting"
	}
	if status.State.Terminated != nil {
		return "Terminated"
	}

	return "Unknown"
}

// getCreationTime 获取创建时间
func getCreationTime(obj metav1.Object) time.Time {
	if obj.GetCreationTimestamp().Time.IsZero() {
		return time.Time{}
	}
	return obj.GetCreationTimestamp().Time
}
