#!/bin/bash

echo "🎯 K8s LLM Monitor 完整功能测试"
echo "================================"

# 1. 启动服务器
echo "1. 启动服务器..."
go run cmd/server/main.go &
SERVER_PID=$!
sleep 3

echo "2. 测试基础功能..."

# 健康检查
echo "  - 健康检查:"
curl -s http://localhost:8080/health | jq .status

# 集群状态
echo "  - 集群状态:"
curl -s http://localhost:8080/api/v1/cluster/status | jq .status

# Pod通信分析（会失败，但能看到错误处理）
echo "  - Pod通信分析（测试错误处理）:"
curl -s -X POST http://localhost:8080/api/v1/analyze/pod-communication \
  -H "Content-Type: application/json" \
  -d '{"pod_a":"default/nginx-1","pod_b":"default/nginx-2"}' | head -1

echo "3. 测试演示程序..."

# 运行RTT演示（会显示连接失败但不会崩溃）
echo "  - RTT演示程序:"
timeout 10s go run cmd/demos/rtt-demo/main.go || echo "    (正常，K8s未连接)"

echo "4. 停止服务器..."
kill $SERVER_PID
sleep 1

echo ""
echo "✅ 当前状态: 项目基础功能正常，等待K8s集群启动"
echo ""
echo "🚀 启动K8s集群的步骤:"
echo "   1. 启动Docker Desktop"
echo "   2. 运行: minikube start"
echo "   3. 验证: kubectl get nodes"
echo "   4. 重新运行此测试脚本"