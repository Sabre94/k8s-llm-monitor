# K3s环境优化指南

## 🚀 性能优化

### 1. 系统级优化
```bash
# 增加文件描述符限制
echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# 优化内核参数
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
echo "fs.file-max=2097152" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# 优化Docker（如果使用）
echo '{"log-driver":"json-file","log-opts":{"max-size":"10m","max-file":"3"}}' | sudo tee /etc/docker/daemon.json
sudo systemctl restart docker
```

### 2. K3s配置优化
```bash
# 编辑K3s服务配置
sudo nano /etc/systemd/system/k3s.service

# 在ExecStart后添加参数：
# --write-kubeconfig-mode 644
# --no-deploy traefik  # 如果不需要traefik
# --disable=metrics-server  # 如果不需要监控

# 重启K3s
sudo systemctl daemon-reload
sudo systemctl restart k3s
```

### 3. 资源限制调整
```yaml
# 在k3s-deployment.yaml中调整资源限制
resources:
  requests:
    memory: "64Mi"    # 根据实际内存调整
    cpu: "50m"        # 根据CPU性能调整
  limits:
    memory: "128Mi"
    cpu: "100m"
```

## 🐛 故障排除

### 常见问题及解决方案

#### 1. 镜像导入失败
```bash
# 检查Docker和K3s兼容性
docker version
sudo k3s version

# 手动导入镜像
docker save uav-scheduler:k3s -o uav-scheduler.tar
sudo k3s ctr images import uav-scheduler.tar

# 验证镜像
sudo k3s ctr images ls | grep uav-scheduler
```

#### 2. Pod启动失败
```bash
# 查看Pod状态
sudo k3s kubectl get pods -n uav-system -o wide

# 查看Pod详情
sudo k3s kubectl describe pod -n uav-system <pod-name>

# 查看Pod日志
sudo k3s kubectl logs -n uav-system <pod-name>

# 进入Pod调试
sudo k3s kubectl exec -it -n uav-system <pod-name> -- /bin/sh
```

#### 3. 调度器无响应
```bash
# 检查服务状态
sudo k3s kubectl get svc -n uav-system

# 检查端口映射
sudo netstat -tlnp | grep :8080

# 端口转发测试
sudo k3s kubectl port-forward -n uav-system svc/uav-scheduler-service 8080:8080 &
curl http://localhost:8080/health
```

#### 4. CRD问题
```bash
# 检查CRD状态
sudo k3s kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com -o yaml

# 重新创建CRD
sudo k3s kubectl delete crd uavmetrics.monitor.k8s-llm-monitor.com
sudo k3s kubectl apply -f deploy/k3s-deployment.yaml

# 验证UAVMetric资源
sudo k3s kubectl get uavmetrics -n uav-system
```

#### 5. 内存不足
```bash
# 检查系统内存
free -h

# 检查K3s组件内存使用
sudo k3s kubectl top pods -A

# 清理未使用的镜像
sudo k3s crictl rmi --prune

# 重启内存密集的Pod
sudo k3s kubectl delete pod -n uav-system -l app=uav-scheduler
```

## 📊 监控和日志

### 1. 基础监控
```bash
# 查看节点状态
sudo k3s kubectl get nodes -o wide

# 查看资源使用
sudo k3s kubectl top nodes
sudo k3s kubectl top pods -n uav-system

# 查看集群事件
sudo k3s kubectl get events -n uav-system --sort-by='.lastTimestamp'
```

### 2. 日志管理
```bash
# 查看K3s服务日志
sudo journalctl -u k3s -f

# 查看调度器日志
sudo k3s kubectl logs -n uav-system deployment/uav-scheduler -f

# 查看所有Pod日志
sudo k3s kubectl logs -n uav-system -l app=uav-scheduler --tail=100
```

### 3. 性能测试
```bash
# 创建测试脚本
cat > test-performance.sh << 'EOF'
#!/bin/bash
echo "🚀 开始性能测试"

# 测试调度器响应
for i in {1..10}; do
  echo "测试 $i/10"
  time curl -s http://localhost:8081/health > /dev/null
  sleep 1
done

echo "✅ 性能测试完成"
EOF

chmod +x test-performance.sh
./test-performance.sh
```

## 🔄 更新和维护

### 1. 更新调度器
```bash
# 重新构建镜像
cd cmd/scheduler
docker build -f Dockerfile.k3s -t uav-scheduler:k3s-new .

# 导出新镜像
docker save uav-scheduler:k3s-new | sudo k3s ctr images import -

# 更新部署
sudo k3s kubectl set image deployment/uav-scheduler uav-scheduler=uav-scheduler:k3s-new -n uav-system

# 检查更新状态
sudo k3s kubectl rollout status deployment/uav-scheduler -n uav-system
```

### 2. 备份和恢复
```bash
# 备份配置
sudo k3s kubectl get all -n uav-system -o yaml > uav-system-backup.yaml

# 备份CRD数据
sudo k3s kubectl get uavmetrics -n uav-system -o yaml > uav-metrics-backup.yaml

# 恢复配置
sudo k3s kubectl apply -f uav-system-backup.yaml
```

## 🎯 生产环境建议

### 1. 安全配置
```bash
# 启用RBAC
sudo k3s kubectl create clusterrolebinding uav-scheduler-cluster-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=uav-system:uav-scheduler

# 网络策略（如果支持）
sudo k3s kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: uav-scheduler-netpol
  namespace: uav-system
spec:
  podSelector:
    matchLabels:
      app: uav-scheduler
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: uav-system
EOF
```

### 2. 高可用配置
```yaml
# 多副本配置
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-scheduler-ha
spec:
  replicas: 2  # 增加副本数
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
```

### 3. 监控告警
```bash
# 安装metrics-server（如果未安装）
sudo k3s kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# 创建监控脚本
cat > monitor.sh << 'EOF'
#!/bin/bash
while true; do
  echo "=== $(date) ==="
  echo "内存使用:"
  free -h
  echo "Pod状态:"
  sudo k3s kubectl get pods -n uav-system
  echo "调度器健康检查:"
  curl -s http://localhost:8081/health || echo "❌ 健康检查失败"
  echo ""
  sleep 60
done
EOF

chmod +x monitor.sh
nohup ./monitor.sh > monitor.log 2>&1 &
```

通过这些优化和故障排除指南，您的UAV调度系统可以在K3s环境中稳定高效地运行！