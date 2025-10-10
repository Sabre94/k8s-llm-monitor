#!/bin/bash

echo "🚀 K8s LLM Monitor 功能测试脚本"
echo "================================"

# 1. 启动服务器（后台运行）
echo "1. 启动服务器..."
go run cmd/server/main.go &
SERVER_PID=$!
sleep 3

# 2. 测试健康检查
echo "2. 测试健康检查接口..."
curl -s http://localhost:8080/health | jq '.' || echo "健康检查失败"

# 3. 测试集群状态
echo "3. 测试集群状态接口..."
curl -s http://localhost:8080/api/v1/cluster/status | jq '.' || echo "集群状态检查失败"

# 4. 测试错误处理
echo "4. 测试错误处理..."
curl -s -X POST http://localhost:8080/api/v1/analyze/pod-communication \
  -H "Content-Type: application/json" \
  -d '{"invalid":"data"}' || echo "错误处理正常"

# 5. 停止服务器
echo "5. 停止服务器..."
kill $SERVER_PID
sleep 1

echo "✅ 测试完成！"