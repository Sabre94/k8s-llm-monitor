package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/k8s-llm-monitor/pkg/models"
	"github.com/sirupsen/logrus"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// CRDWatcher CRD监控器
type CRDWatcher struct {
	client          *Client
	dynamicClient   dynamic.Interface
	crdClient       *apiextensionsv1client.Clientset
	logger          *logrus.Logger
	crdWatchers     map[schema.GroupVersionResource]watch.Interface
	customResources map[string][]*models.CustomResourceInfo
	eventHandler    EventHandler
}

// NewCRDWatcher 创建新的CRD监控器
func NewCRDWatcher(client *Client, handler EventHandler) (*CRDWatcher, error) {
	// 创建dynamic client
	dynamicClient, err := dynamic.NewForConfig(client.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// 创建CRD clientset
	crdClient, err := apiextensionsv1client.NewForConfig(client.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD clientset: %w", err)
	}

	return &CRDWatcher{
		client:          client,
		dynamicClient:   dynamicClient,
		crdClient:       crdClient,
		logger:          client.logger,
		crdWatchers:     make(map[schema.GroupVersionResource]watch.Interface),
		customResources: make(map[string][]*models.CustomResourceInfo),
		eventHandler:    handler,
	}, nil
}

// Start 开始监控CRD和自定义资源
func (cw *CRDWatcher) Start(ctx context.Context) error {
	cw.logger.Info("Starting CRD watcher")

	// 1. 监控CRD资源
	go cw.watchCRDs(ctx)

	// 2. 获取现有CRD并监控自定义资源
	if err := cw.discoverAndWatchCustomResources(ctx); err != nil {
		return fmt.Errorf("failed to discover custom resources: %w", err)
	}

	return nil
}

// watchCRDs 监控CRD资源
func (cw *CRDWatcher) watchCRDs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			cw.doWatchCRDs(ctx)
			time.Sleep(5 * time.Second)
		}
	}
}

// doWatchCRDs 执行CRD监控
func (cw *CRDWatcher) doWatchCRDs(ctx context.Context) {
	watcher, err := cw.crdClient.ApiextensionsV1().CustomResourceDefinitions().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		cw.logger.Errorf("Failed to watch CRDs: %v", err)
		return
	}
	defer watcher.Stop()

	cw.logger.Info("Watching CRDs")

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				cw.logger.Warn("CRD watcher channel closed")
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				crd, ok := event.Object.(*apiextensionsv1.CustomResourceDefinition)
				if !ok {
					cw.logger.Warn("Received non-CRD object in CRD watcher")
					continue
				}

				// 转换CRD信息
				crdInfo := cw.convertCRDToModel(crd)
				cw.logger.Infof("CRD %s %s", string(event.Type), crdInfo.Name)

				// 如果是新增的CRD，开始监控对应的自定义资源
				if event.Type == watch.Added {
					go cw.watchCustomResource(ctx, crdInfo)
				}

				// 发送CRD事件
				if cw.eventHandler != nil {
					cw.eventHandler.OnCRDEvent(&models.CRDEvent{
						Type:      string(event.Type),
						Kind:      "CustomResourceDefinition",
						Group:     "apiextensions.k8s.io",
						Version:   "v1",
						Name:      crd.Name,
						Namespace: "",
						Object:    map[string]interface{}{
							"crd": crdInfo,
						},
						Timestamp: time.Now(),
					})
				}

			case watch.Deleted:
				crd, ok := event.Object.(*apiextensionsv1.CustomResourceDefinition)
				if !ok {
					cw.logger.Warn("Received non-CRD object in CRD watcher")
					continue
				}

				cw.logger.Infof("CRD %s deleted: %s", string(event.Type), crd.Name)

				// 停止监控对应的自定义资源
				gvr := schema.GroupVersionResource{
					Group:    crd.Spec.Group,
					Resource: crd.Spec.Names.Plural,
				}
				if watcher, exists := cw.crdWatchers[gvr]; exists {
					watcher.Stop()
					delete(cw.crdWatchers, gvr)
				}

				// 发送CRD事件
				if cw.eventHandler != nil {
					cw.eventHandler.OnCRDEvent(&models.CRDEvent{
						Type:      string(event.Type),
						Kind:      "CustomResourceDefinition",
						Group:     "apiextensions.k8s.io",
						Version:   "v1",
						Name:      crd.Name,
						Namespace: "",
						Object:    map[string]interface{}{
							"crd": crd.Name,
						},
						Timestamp: time.Now(),
					})
				}
			}
		}
	}
}

// discoverAndWatchCustomResources 发现并监控现有CRD的自定义资源
func (cw *CRDWatcher) discoverAndWatchCustomResources(ctx context.Context) error {
	// 获取所有CRD
	crdList, err := cw.crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	cw.logger.Infof("Discovered %d CRDs", len(crdList.Items))

	// 为每个已建立的CRD启动监控
	for _, crd := range crdList.Items {
		if len(crd.Status.Conditions) > 0 {
			for _, condition := range crd.Status.Conditions {
				if condition.Type == "Established" && condition.Status == "True" {
					crdInfo := cw.convertCRDToModel(&crd)
					go cw.watchCustomResource(ctx, crdInfo)
					break
				}
			}
		}
	}

	return nil
}

// watchCustomResource 监控自定义资源
func (cw *CRDWatcher) watchCustomResource(ctx context.Context, crd *models.CRDInfo) {
	gvr := schema.GroupVersionResource{
		Group:    crd.Group,
		Resource: crd.Plural,
	}

	// 如果已经在监控，先停止
	if watcher, exists := cw.crdWatchers[gvr]; exists {
		watcher.Stop()
	}

	cw.logger.Infof("Starting to watch custom resource: %s/%s", crd.Group, crd.Plural)

	// 根据CRD的范围决定监控范围
	for {
		select {
		case <-ctx.Done():
			return
		default:
			cw.doWatchCustomResource(ctx, crd, gvr)
			time.Sleep(5 * time.Second)
		}
	}
}

// doWatchCustomResource 执行自定义资源监控
func (cw *CRDWatcher) doWatchCustomResource(ctx context.Context, crd *models.CRDInfo, gvr schema.GroupVersionResource) {
	var watcher watch.Interface
	var err error

	if crd.Scope == "Cluster" {
		// 集群范围的自定义资源
		watcher, err = cw.dynamicClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
	} else {
		// 命名空间范围的自定义资源
		watcher, err = cw.dynamicClient.Resource(gvr).Namespace("").Watch(ctx, metav1.ListOptions{})
	}

	if err != nil {
		cw.logger.Errorf("Failed to watch custom resource %s/%s: %v", crd.Group, crd.Plural, err)
		return
	}

	cw.crdWatchers[gvr] = watcher
	defer func() {
		watcher.Stop()
		delete(cw.crdWatchers, gvr)
	}()

	cw.logger.Infof("Watching custom resource: %s/%s", crd.Group, crd.Plural)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				cw.logger.Warnf("Custom resource watcher channel closed for %s/%s", crd.Group, crd.Plural)
				return
			}

			// 处理unstructured对象
			unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				cw.logger.Warn("Received non-unstructured object in custom resource watcher")
				continue
			}

			// 转换为自定义资源信息
			customResource := cw.convertUnstructuredToCustomResource(unstructuredObj, crd)

			// 更新缓存
			cw.updateCustomResourceCache(crd, customResource, string(event.Type))

			cw.logger.Infof("Custom resource %s %s/%s", string(event.Type), crd.Kind, customResource.Name)

			// 发送事件
			if cw.eventHandler != nil {
				cw.eventHandler.OnCRDEvent(&models.CRDEvent{
					Type:      string(event.Type),
					Kind:      crd.Kind,
					Group:     crd.Group,
					Version:   customResource.Version,
					Name:      customResource.Name,
					Namespace: customResource.Namespace,
					Object:    unstructuredObj.Object,
					Timestamp: time.Now(),
				})
			}
		}
	}
}

// convertCRDToModel 转换CRD到模型
func (cw *CRDWatcher) convertCRDToModel(crd *apiextensionsv1.CustomResourceDefinition) *models.CRDInfo {
	versions := make([]string, len(crd.Spec.Versions))
	for i, version := range crd.Spec.Versions {
		versions[i] = version.Name
	}

	// Check if CRD is established
	isEstablished := false
	for _, condition := range crd.Status.Conditions {
		if condition.Type == "Established" && condition.Status == "True" {
			isEstablished = true
			break
		}
	}

	return &models.CRDInfo{
		Name:         crd.Name,
		Group:        crd.Spec.Group,
		Kind:         crd.Spec.Names.Kind,
		Scope:        string(crd.Spec.Scope),
		Versions:     versions,
		Plural:       crd.Spec.Names.Plural,
		Singular:     crd.Spec.Names.Singular,
		Established:  isEstablished,
		Stored:       len(crd.Status.StoredVersions) > 0,
		CreationTime: crd.CreationTimestamp.Time,
	}
}

// convertUnstructuredToCustomResource 转换unstructured对象到自定义资源信息
func (cw *CRDWatcher) convertUnstructuredToCustomResource(obj *unstructured.Unstructured, crd *models.CRDInfo) *models.CustomResourceInfo {
	return &models.CustomResourceInfo{
		Kind:         crd.Kind,
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		Group:        crd.Group,
		Version:      obj.GetAPIVersion(),
		Spec:         obj.Object["spec"].(map[string]interface{}),
		Status:       cw.getStatusFromObject(obj.Object),
		Generation:   obj.GetGeneration(),
		CreationTime: obj.GetCreationTimestamp().Time,
		UpdateTime:   getLastUpdateTime(obj),
	}
}

// getStatusFromObject 从对象中提取状态
func (cw *CRDWatcher) getStatusFromObject(obj map[string]interface{}) map[string]interface{} {
	if status, ok := obj["status"].(map[string]interface{}); ok {
		return status
	}
	return make(map[string]interface{})
}


// updateCustomResourceCache 更新自定义资源缓存
func (cw *CRDWatcher) updateCustomResourceCache(crd *models.CRDInfo, resource *models.CustomResourceInfo, eventType string) {
	key := fmt.Sprintf("%s/%s/%s", crd.Group, crd.Kind, resource.Namespace)

	switch eventType {
	case "ADDED", "MODIFIED":
		// 添加或更新资源
		resources := cw.customResources[key]
		found := false
		for i, existing := range resources {
			if existing.Name == resource.Name {
				resources[i] = resource
				found = true
				break
			}
		}
		if !found {
			resources = append(resources, resource)
		}
		cw.customResources[key] = resources

	case "DELETED":
		// 删除资源
		resources := cw.customResources[key]
		for i, existing := range resources {
			if existing.Name == resource.Name {
				cw.customResources[key] = append(resources[:i], resources[i+1:]...)
				break
			}
		}
	}
}

// GetCRDs 获取所有CRD
func (cw *CRDWatcher) GetCRDs(ctx context.Context) ([]*models.CRDInfo, error) {
	crdList, err := cw.crdClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	var crdInfos []*models.CRDInfo
	for _, crd := range crdList.Items {
		crdInfo := cw.convertCRDToModel(&crd)
		crdInfos = append(crdInfos, crdInfo)
	}

	return crdInfos, nil
}

// GetCustomResources 获取指定类型的自定义资源
func (cw *CRDWatcher) GetCustomResources(group, kind, namespace string) ([]*models.CustomResourceInfo, error) {
	key := fmt.Sprintf("%s/%s/%s", group, kind, namespace)
	if resources, ok := cw.customResources[key]; ok {
		return resources, nil
	}
	return []*models.CustomResourceInfo{}, nil
}