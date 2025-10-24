#!/bin/bash

# UAV调度系统快速部署脚本
# 使用方法: ./quick-start.sh [namespace]

set -e

NAMESPACE=${1:-default}
echo "🚀 开始部署UAV调度系统到命名空间: $NAMESPACE"

# 检查kubectl连接
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ 无法连接到Kubernetes集群，请检查kubectl配置"
    exit 1
fi

# 检查权限
if ! kubectl auth can-i create deployments -n $NAMESPACE; then
    echo "❌ 在命名空间 $NAMESPACE 中没有创建部署的权限"
    exit 1
fi

echo "✅ Kubernetes集群连接正常"

# 创建命名空间（如果不存在）
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# 部署CRD
echo "📦 创建UAVMetric CRD..."
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: uavmetrics.monitor.k8s-llm-monitor.com
spec:
  group: monitor.k8s-llm-monitor.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              nodeId:
                type: string
                description: "UAV节点唯一标识"
              battery:
                type: number
                minimum: 0
                maximum: 100
                description: "电池电量百分比"
              latency:
                type: number
                minimum: 0
                description: "网络延迟(ms)"
              utilization:
                type: number
                minimum: 0
                maximum: 100
                description: "CPU利用率百分比"
              gps:
                type: array
                items:
                  type: number
                minItems: 2
                maxItems: 2
                description: "GPS坐标 [纬度, 经度]"
              radius:
                type: number
                minimum: 0
                description: "UAV覆盖半径(米)"
              status:
                type: string
                enum: ["ready", "busy", "maintenance", "offline"]
                description: "节点状态"
  scope: Namespaced
  names:
    plural: uavmetrics
    singular: uavmetric
    kind: UAVMetric
EOF

# 创建RBAC权限
echo "🔐 创建RBAC权限..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: uav-scheduler
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: uav-scheduler
rules:
- apiGroups: [""]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["monitor.k8s-llm-monitor.com"]
  resources: ["uavmetrics"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: uav-scheduler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: uav-scheduler
subjects:
- kind: ServiceAccount
  name: uav-scheduler
  namespace: $NAMESPACE
EOF

# 创建配置
echo "⚙️ 创建��置ConfigMap..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: uav-scheduler-config
  namespace: $NAMESPACE
data:
  config.yaml: |
    # UAV Scheduler Configuration

    # 基础配置
    name: "uav-scheduler"
    listen_addr: ":8080"

    # Kubernetes配置
    k8s:
      namespace: "$NAMESPACE"
      in_cluster: true
      kubeconfig: ""

    # 算法配置
    algorithms:
      nsga2:
        population_size: 50
        max_generations: 20
        crossover_prob: 0.9
        grid_density: 40
        python_path: "python3"
        script_path: "RUN2.py"
        temp_dir: "/tmp"

      demo:
        enabled: true  # 快速体验启用demo

    # 数据提供者配置
    data_providers:
      crd:
        enabled: true
        watch_interval: "30s"
        cache_ttl: "2m"

    # 缓存配置
    cache:
      enabled: true
      ttl: "5m"
      max_size: 1000

    # 日志配置
    logging:
      level: "info"
      format: "json"
      output: "stdout"

    # 性能配置
    performance:
      max_concurrent_schedules: 10
      schedule_timeout: "30s"
      cleanup_interval: "1m"

    # 监控配置
    monitoring:
      metrics_enabled: true
      health_check_enabled: true
      status_port: 8081
EOF

# 创建示例UAV节点数据
echo "🚁 创建示例UAV节点数据..."
kubectl apply -f - <<EOF
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-1
  namespace: $NAMESPACE
  labels:
    type: uav
    zone: zone-a
    hardware: standard
spec:
  nodeId: "uav-node-1"
  battery: 85.5
  latency: 45.2
  utilization: 35.8
  gps: [34.043392, -118.266096]
  radius: 480.9
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-2
  namespace: $NAMESPACE
  labels:
    type: uav
    zone: zone-b
    hardware: high-performance
spec:
  nodeId: "uav-node-2"
  battery: 72.3
  latency: 78.5
  utilization: 42.1
  gps: [34.044353, -118.253013]
  radius: 399.4
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-3
  namespace: $NAMESPACE
  labels:
    type: uav
    zone: zone-a
    hardware: standard
spec:
  nodeId: "uav-node-3"
  battery: 91.2
  latency: 32.8
  utilization: 28.5
  gps: [34.035058, -118.247302]
  radius: 305.6
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-4
  namespace: $NAMESPACE
  labels:
    type: uav
    zone: zone-c
    hardware: high-performance
spec:
  nodeId: "uav-node-4"
  battery: 68.9
  latency: 95.3
  utilization: 55.7
  gps: [34.030712, -118.272819]
  radius: 489.6
  status: "ready"
---
apiVersion: monitor.k8s-llm-monitor.com/v1
kind: UAVMetric
metadata:
  name: uav-node-5
  namespace: $NAMESPACE
  labels:
    type: uav
    zone: zone-b
    hardware: standard
spec:
  nodeId: "uav-node-5"
  battery: 55.4
  latency: 120.5
  utilization: 68.2
  gps: [34.050073, -118.273802]
  radius: 447.0
  status: "ready"
EOF

echo "⚠️  注意：以下部署需要您先构建Docker镜像"
echo "请先在项目根目录运行："
echo "  docker build -t uav-scheduler:latest -f cmd/scheduler/Dockerfile ."
echo ""
echo "或者跳过调度器部署，只使用CRD和示例数据："
read -p "是否继续部署调度器Pod？(y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    # 创建调度器部署（需要镜像）
    echo "🚀 创建调度器部署..."
    kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-scheduler
  namespace: $NAMESPACE
  labels:
    app: uav-scheduler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: uav-scheduler
  template:
    metadata:
      labels:
        app: uav-scheduler
    spec:
      serviceAccountName: uav-scheduler
      containers:
      - name: uav-scheduler
        image: uav-scheduler:latest  # 请确保已构建此镜像
        imagePullPolicy: Never
        command: ["./uav-scheduler"]
        args: ["-config=/etc/config/config.yaml", "-log-level=info"]
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 8081
          name: status
        volumeMounts:
        - name: config
          mountPath: /etc/config
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: uav-scheduler-config
EOF

    # 创建服务
    echo "🌐 创建服务..."
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: uav-scheduler-service
  namespace: $NAMESPACE
  labels:
    app: uav-scheduler
spec:
  selector:
    app: uav-scheduler
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  - port: 8081
    targetPort: 8081
    name: status
  type: ClusterIP
EOF
fi

# 验证部署
echo ""
echo "✅ 部署完成！验证部署状态："

# 检查CRD
if kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com &> /dev/null; then
    echo "✅ UAVMetric CRD已创建"
else
    echo "❌ UAVMetric CRD创建失败"
fi

# 检查UAV节点
UAV_COUNT=$(kubectl get uavmetrics -n $NAMESPACE --no-headers | wc -l)
echo "✅ 已创建 $UAV_COUNT 个UAV节点数据"

# 检查调度器（如果部署了）
if kubectl get deployment uav-scheduler -n $NAMESPACE &> /dev/null; then
    echo "✅ 调度器部署已创建"
    READY=$(kubectl get deployment uav-scheduler -n $NAMESPACE -o jsonpath='{.status.readyReplicas}')
    if [[ "$READY" == "1" ]]; then
        echo "✅ 调度器Pod运行正常"

        # 等待demo执行
        echo "⏳ 等待demo模式执行（10秒）..."
        sleep 10

        # 显示日志
        echo ""
        echo "📋 最近调度器日志："
        kubectl logs -n $NAMESPACE deployment/uav-scheduler --tail=20
    else
        echo "⏳ 调度器Pod正在启动中..."
        echo "运行以下命令查看状态："
        echo "  kubectl get pods -n $NAMESPACE -l app=uav-scheduler"
        echo "  kubectl logs -n $NAMESPACE deployment/uav-scheduler"
    fi
else
    echo "⚠️  调度器未部署（需要构建镜像）"
fi

echo ""
echo "🎯 快速测试命令："
echo "# 查看UAV节点数据"
echo "kubectl get uavmetrics -n $NAMESPACE"
echo ""
echo "# 查看调度器状态"
echo "kubectl get pods -n $NAMESPACE -l app=uav-scheduler"
echo ""
echo "# 查看调度器日志"
echo "kubectl logs -n $NAMESPACE deployment/uav-scheduler"
echo ""
echo "# 创建测试任务"
echo "cat <<EOF | kubectl apply -f -"
echo "apiVersion: v1"
echo "kind: Pod"
echo "metadata:"
echo "  name: test-mission"
echo "  namespace: $NAMESPACE"
echo "  annotations:"
echo "    uav-scheduler/algorithm: \"nsga2\""
echo "    uav-scheduler/task-type: \"surveillance\""
echo "    uav-scheduler/target-coverage: \"0.8\""
echo "spec:"
echo "  restartPolicy: Never"
echo "  containers:"
echo "  - name: mission"
echo "    image: busybox"
echo "    command: [\"/bin/sh\", \"-c\", \"echo 'Mission started' && sleep 3600\"]"
echo "EOF"

echo ""
echo "🎉 部署完成！详细文档请查看 docs/UAV_SCHEDULER_README.md"