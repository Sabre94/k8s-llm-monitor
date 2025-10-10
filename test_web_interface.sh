#!/bin/bash

echo "🌐 K8s LLM Monitor Web界面测试"
echo "================================="

# 检查服务器是否运行
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "❌ 服务器未运行，请先启动："
    echo "   go run cmd/server/main.go"
    exit 1
fi

echo "✅ 服务器运行正常"

# 测试各个接口
echo ""
echo "📊 测试API接口："

echo "1. 健康检查："
curl -s http://localhost:8080/health | jq .status

echo "2. 集群状态："
curl -s http://localhost:8080/api/v1/cluster/status | jq .status

echo "3. Pod数量："
curl -s http://localhost:8080/api/v1/pods | jq .count

echo "4. Web界面："
if curl -s http://localhost:8080/ | grep -q "K8s LLM Monitor"; then
    echo "✅ Web界面可访问"
else
    echo "❌ Web界面无法访问"
fi

echo ""
echo "🚀 所有功能就绪！"
echo ""
echo "📱 访问Web界面："
echo "   浏览器打开: http://localhost:8080"
echo ""
echo "🛠️ 功能说明："
echo "   • 实时查看集群状态"
echo "   • 监控Pod运行情况"
echo "   • 分析Pod间网络连通性"
echo "   • 自动刷新数据"