#!/bin/bash

# UAV调度系统K3s部署脚本
# 使用方法: ./deploy-to-k3s.sh

set -e

echo "🚀 开始部署UAV调度系统到K3s集群"

# 检查K3s是否运行
if ! command -v k3s &> /dev/null; then
    echo "❌ K3s未安装，请先安装K3s"
    echo "安装命令: curl -sfL https://get.k3s.io | sh"
    exit 1
fi

if ! sudo k3s kubectl cluster-info &> /dev/null; then
    echo "❌ K3s集群未运行，请启动K3s"
    echo "启动命令: sudo systemctl start k3s"
    exit 1
fi

echo "✅ K3s集群运行正常"

# 设置kubectl别名
alias kubectl='sudo k3s kubectl'

# 创建项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "📦 项目根目录: $PROJECT_ROOT"

# 构建K3s优化镜像
echo "🏗️ 构建K3s优化镜像..."
cd cmd/scheduler

if ! docker build -f Dockerfile.k3s -t uav-scheduler:k3s .; then
    echo "❌ 镜像构建失败"
    exit 1
fi

echo "✅ 镜像构建成功"

# 导出镜像到K3s
echo "📤 导出镜像到K3s..."
docker save uav-scheduler:k3s | sudo k3s ctr images import -

# 验证镜像导入
if sudo k3s ctr images ls | grep -q "uav-scheduler:k3s"; then
    echo "✅ 镜像导入成功"
else
    echo "❌ 镜像导入失败"
    exit 1
fi

# 返回项目根目录
cd "$PROJECT_ROOT"

# 部署到K3s
echo "🚀 部署应用到K3s集群..."
if ! sudo k3s kubectl apply -f deploy/k3s-deployment.yaml; then
    echo "❌ 部署失败"
    exit 1
fi

echo "✅ 部署成功"

# 等待Pod启动
echo "⏳ 等待调度器Pod启动..."
sleep 30

# 检查部署状态
echo "📋 检查部署状态..."

# 检查命名空间
if sudo k3s kubectl get namespace uav-system &> /dev/null; then
    echo "✅ 命名空间 uav-system 创建成功"
else
    echo "❌ 命名空间创建失败"
    exit 1
fi

# 检查CRD
if sudo k3s kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com &> /dev/null; then
    echo "✅ UAVMetric CRD 创建成功"
else
    echo "❌ CRD创建失败"
    exit 1
fi

# 检查UAV节点数据
UAV_COUNT=$(sudo k3s kubectl get uavmetrics -n uav-system --no-headers 2>/dev/null | wc -l)
if [ "$UAV_COUNT" -gt 0 ]; then
    echo "✅ 创建了 $UAV_COUNT 个UAV节点数据"
else
    echo "⚠️  UAV节点数据可能还在创建中"
fi

# 检查调度器Pod
POD_STATUS=$(sudo k3s kubectl get pods -n uav-system -l app=uav-scheduler -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
if [ "$POD_STATUS" = "Running" ]; then
    echo "✅ 调度器Pod运行正常"

    # 等待demo执行
    echo "⏳ 等待demo模式执行（15秒）..."
    sleep 15

    # 显示调度器日志
    echo ""
    echo "📋 调度器最新日志："
    sudo k3s kubectl logs -n uav-system deployment/uav-scheduler --tail=20

else
    echo "⏳ 调度器Pod状态: $POD_STATUS"
    echo "查看详细状态："
    sudo k3s kubectl get pods -n uav-system -l app=uav-scheduler
    echo ""
    echo "查看Pod详情："
    sudo k3s kubectl describe pods -n uav-system -l app=uav-scheduler
fi

# 检查服务
if sudo k3s kubectl get svc uav-scheduler-service -n uav-system &> /dev/null; then
    echo "✅ 服务创建成功"

    # 获取NodePort信息
    NODE_PORT=$(sudo k3s kubectl get svc uav-scheduler-service -n uav-system -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}')
    STATUS_PORT=$(sudo k3s kubectl get svc uav-scheduler-service -n uav-system -o jsonpath='{.spec.ports[?(@.name=="status")].nodePort}')

    echo ""
    echo "🌐 服务访问信息："
    echo "HTTP服务端口: $NODE_PORT"
    echo "状态检查端口: $STATUS_PORT"

    # 获取节点IP
    NODE_IP=$(sudo k3s kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
    echo "节点IP: $NODE_IP"
    echo ""
    echo "🔗 访问地址："
    echo "HTTP: http://$NODE_IP:$NODE_PORT"
    echo "健康检查: http://$NODE_IP:$STATUS_PORT/health"

else
    echo "❌ 服务创建失败"
fi

# 显示测试命令
echo ""
echo "🎯 测试命令："
echo "# 查看UAV节点数据"
echo "sudo k3s kubectl get uavmetrics -n uav-system"
echo ""
echo "# 查看调度器日志"
echo "sudo k3s kubectl logs -n uav-system deployment/uav-scheduler -f"
echo ""
echo "# 查看调度器状态"
echo "sudo k3s kubectl get pods -n uav-system -l app=uav-scheduler"
echo ""
echo "# 创建测试任务"
echo "cat <<EOF | sudo k3s kubectl apply -f -"
echo "apiVersion: v1"
echo "kind: Pod"
echo "metadata:"
echo "  name: test-mission"
echo "  namespace: uav-system"
echo "  annotations:"
echo "    uav-scheduler/algorithm: \"nsga2\""
echo "    uav-scheduler/task-type: \"surveillance\""
echo "    uav-scheduler/target-coverage: \"0.8\""
echo "spec:"
echo "  restartPolicy: Never"
echo "  containers:"
echo "  - name: mission"
echo "    image: busybox"
echo "    command: [\"/bin/sh\", \"-c\", \"echo 'K3s UAV Mission Started' && sleep 3600\"]"
echo "EOF"

# 性能监控建议
echo ""
echo "📊 K3s性能监控："
echo "# 查看节点资源使用"
echo "sudo k3s kubectl top nodes"
echo ""
echo "# 查看Pod资源使用"
echo "sudo k3s kubectl top pods -n uav-system"
echo ""
echo "# 查看系统资源"
echo "free -h"
echo "df -h"

echo ""
echo "🎉 UAV调度系统已成功部署到K3s集群！"
echo "📚 详细文档: docs/UAV_SCHEDULER_README.md"
echo "🐛 故障排除: 检查Pod状态和日志"