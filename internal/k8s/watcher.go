package k8s

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/k8s-llm-monitor/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// EventHandler 事件处理器接口
type EventHandler interface {
	OnPodUpdate(pod *models.PodInfo)
	OnServiceUpdate(service *models.ServiceInfo)
	OnEvent(event *models.EventInfo)
	OnCRDEvent(event *models.CRDEvent)
}

// Watcher 资源监控器
type Watcher struct {
	client  *Client
	handler EventHandler
	logger  *logrus.Logger
	stopCh  chan struct{}
}

// NewWatcher 创建新的监控器
func NewWatcher(client *Client, handler EventHandler) *Watcher {
	return &Watcher{
		client:  client,
		handler: handler,
		logger:  client.logger,
		stopCh:  make(chan struct{}),
	}
}

// Start 开始监控
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting K8s resource watcher")

	// 为每个namespace启动监控
	for _, namespace := range w.client.namespaces {
		go w.watchNamespace(ctx, namespace)
	}

	return nil
}

// Stop 停止监控
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.logger.Info("K8s resource watcher stopped")
}

// watchNamespace 监控指定namespace
func (w *Watcher) watchNamespace(ctx context.Context, namespace string) {
	w.logger.Infof("Start watching namespace: %s", namespace)

	// 启动Pod监控
	go w.watchPods(ctx, namespace)

	// 启动Service监控
	go w.watchServices(ctx, namespace)

	// 启动事件监控
	go w.watchEvents(ctx, namespace)
}

// watchPods 监控Pod变化
func (w *Watcher) watchPods(ctx context.Context, namespace string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			w.doWatchPods(ctx, namespace)
			// 如果连接断开，等待一段时间后重试
			time.Sleep(5 * time.Second)
		}
	}
}

// doWatchPods 执行Pod监控
func (w *Watcher) doWatchPods(ctx context.Context, namespace string) {
	watcher, err := w.client.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.logger.Errorf("Failed to watch pods in namespace %s: %v", namespace, err)
		return
	}
	defer watcher.Stop()

	w.logger.Infof("Watching pods in namespace: %s", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				w.logger.Warnf("Pod watcher channel closed for namespace: %s", namespace)
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					w.logger.Warnf("Received non-pod object in pod watcher")
					continue
				}

				podInfo := w.client.convertPodToModel(pod)
				w.handler.OnPodUpdate(podInfo)

				w.logger.Debugf("Pod %s/%s: %s", namespace, pod.Name, event.Type)
			}
		}
	}
}

// watchServices 监控Service变化
func (w *Watcher) watchServices(ctx context.Context, namespace string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			w.doWatchServices(ctx, namespace)
			time.Sleep(5 * time.Second)
		}
	}
}

// doWatchServices 执行Service监控
func (w *Watcher) doWatchServices(ctx context.Context, namespace string) {
	watcher, err := w.client.clientset.CoreV1().Services(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.logger.Errorf("Failed to watch services in namespace %s: %v", namespace, err)
		return
	}
	defer watcher.Stop()

	w.logger.Infof("Watching services in namespace: %s", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				w.logger.Warnf("Service watcher channel closed for namespace: %s", namespace)
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				service, ok := event.Object.(*corev1.Service)
				if !ok {
					w.logger.Warnf("Received non-service object in service watcher")
					continue
				}

				serviceInfo := w.client.convertServiceToModel(service)
				w.handler.OnServiceUpdate(serviceInfo)

				w.logger.Debugf("Service %s/%s: %s", namespace, service.Name, event.Type)
			}
		}
	}
}

// watchEvents 监控事件
func (w *Watcher) watchEvents(ctx context.Context, namespace string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			w.doWatchEvents(ctx, namespace)
			time.Sleep(5 * time.Second)
		}
	}
}

// doWatchEvents 执行事件监控
func (w *Watcher) doWatchEvents(ctx context.Context, namespace string) {
	watcher, err := w.client.clientset.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.logger.Errorf("Failed to watch events in namespace %s: %v", namespace, err)
		return
	}
	defer watcher.Stop()

	w.logger.Infof("Watching events in namespace: %s", namespace)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				w.logger.Warnf("Event watcher channel closed for namespace: %s", namespace)
				return
			}

			switch event.Type {
			case watch.Added:
				k8sEvent, ok := event.Object.(*corev1.Event)
				if !ok {
					w.logger.Warnf("Received non-event object in event watcher")
					continue
				}

				eventInfo := w.client.convertEventToModel(k8sEvent)
				w.handler.OnEvent(eventInfo)

				w.logger.Debugf("Event %s in %s: %s - %s", k8sEvent.Reason, namespace, k8sEvent.InvolvedObject.Name, k8sEvent.Message)
			}
		}
	}
}

// WatchResources 统一的资源监控接口
func (c *Client) WatchResources(ctx context.Context, handler EventHandler) error {
	watcher := NewWatcher(c, handler)
	return watcher.Start(ctx)
}
