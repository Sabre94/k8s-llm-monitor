#!/bin/bash

set -e

echo "=========================================="
echo "Building and Deploying UAV Agent"
echo "=========================================="

# 检查是否在项目根目录
if [ ! -f "go.mod" ]; then
    echo "Error: Must run from project root directory"
    exit 1
fi

# 构建Docker镜像（使用k3d的镜像仓库）
echo "Building Docker image..."
docker build -f build/Dockerfile.uav-agent -t uav-agent:latest .

# 导入镜像到k3d集群
echo "Importing image to k3d cluster..."
k3d image import uav-agent:latest -c k8s-llm-monitor

# 部署DaemonSet
echo "Deploying UAV Agent DaemonSet..."
kubectl apply -f deployments/uav-agent-daemonset.yaml

# 等待Pod启动
echo "Waiting for UAV agents to be ready..."
kubectl rollout status daemonset/uav-agent -n default --timeout=120s

# 显示部署状态
echo ""
echo "=========================================="
echo "Deployment Complete!"
echo "=========================================="
echo ""
kubectl get pods -l app=uav-agent -o wide

echo ""
echo "UAV Agent Endpoints:"
kubectl get pods -l app=uav-agent -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,IP:.status.podIP,HOST_IP:.status.hostIP --no-headers | while read name node pod_ip host_ip; do
    echo "  - $name (Node: $node)"
    echo "    http://$host_ip:9090/health"
    echo "    http://$host_ip:9090/api/v1/state"
done

echo ""
echo "Test commands:"
echo "  # Get health status"
echo "  curl http://localhost:9090/health"
echo ""
echo "  # Get UAV state"
echo "  curl http://localhost:9090/api/v1/state"
echo ""
echo "  # Arm the drone"
echo "  curl -X POST http://localhost:9090/api/v1/command/arm"
echo ""
echo "  # Take off to 50m"
echo "  curl -X POST http://localhost:9090/api/v1/command/takeoff -H 'Content-Type: application/json' -d '{\"altitude\":50}'"
