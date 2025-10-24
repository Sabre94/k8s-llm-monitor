# UAV调度系统 - 快速启动指南

## 🚀 5分钟快速体验

### 前置要求
- ✅ Kubernetes集群 (minikube, k3s, 或其他)
- ✅ kubectl配置正确
- ✅ Docker (用于构建镜像)

### 一键部署

```bash
# 1. 克隆项目
git clone <repository-url>
cd k8s-llm-monitor

# 2. 快速部署（自动创建所有必要资源）
./deploy/quick-start.sh

# 3. 构建调度器镜像（在新终端）
cd cmd/scheduler
docker build -t uav-scheduler:latest .
```

### 验证部署

```bash
# 检查调度器状态
kubectl get pods -l app=uav-scheduler

# 查看UAV节点数据
kubectl get uavmetrics

# 查看调度日志
kubectl logs -f deployment/uav-scheduler
```

### 创建测试任务

```bash
kubectl apply -f deploy/test-mission.yaml
```

预期看到调度器自动选择最优UAV节点集合执行任务！

## 📦 详细部署选项

### 选项1: 本地测试模式

```bash
cd cmd/scheduler
go build -o uav-scheduler
./uav-scheduler -config=config.yaml -dry-run=true -log-level=info
```

### 选项2: 完整Kubernetes部署

```bash
# 1. 构建镜像
docker build -t your-registry/uav-scheduler:latest -f cmd/scheduler/Dockerfile .

# 2. 推送镜像
docker push your-registry/uav-scheduler:latest

# 3. 修改部署文件中的镜像地址
# 编辑 deploy/quick-start.sh 中的镜像名称

# 4. 运行部署脚本
./deploy/quick-start.sh production
```

## 🎯 调度流程验证

当你看到类似以下的日志时，说明调度器正在工作：

```
{"level":"info","msg":"Demo: Single pod scheduling"}
{"level":"info","msg":"NSGA-II optimization completed","selected_nodes":["uav-node-3","uav-node-1","uav-node-2"]}
{"level":"info","msg":"Mission scheduling completed successfully","node_count":3}
```

这表示：
- ✅ NSGA-II算法成功优化
- ✅ 自动选择了3个最优UAV节点
- ✅ 节点集合已绑定到任务

## 🐛 常见问题

**Q: 调度器Pod无法启动？**
```bash
# 检查Pod状态
kubectl describe pod -l app=uav-scheduler

# 确保镜像已构建
docker images | grep uav-scheduler
```

**Q: 没有看到demo执行？**
```bash
# 检查配置中的demo设置
kubectl get configmap uav-scheduler-config -o yaml

# 手动重启调度器
kubectl rollout restart deployment/uav-scheduler
```

**Q: UAVMetric CRD不存在？**
```bash
# 检查CRD状态
kubectl get crd uavmetrics.monitor.k8s-llm-monitor.com

# 重新创建CRD
kubectl apply -f docs/UAV_SCHEDULER_README.md
```

## 📚 更多信息

- 📖 **完整文档**: [docs/UAV_SCHEDULER_README.md](docs/UAV_SCHEDULER_README.md)
- 🔧 **配置详解**: [docs/UAV_SCHEDULER_README.md#配置详解](docs/UAV_SCHEDULER_README.md#配置详解)
- 🐛 **故障排除**: [docs/UAV_SCHEDULER_README.md#故障排除](docs/UAV_SCHEDULER_README.md#故障排除)

## 🎉 成功标志

当你看到以下情况时，说明部署成功：

1. ✅ 调度器Pod运行正常
2. ✅ 5个UAV节点数据创建成功
3. ✅ Demo模式自动执行
4. ✅ 日志显示"NSGA-II optimization completed"
5. ✅ 节点集合绑定成功

恭喜！你的UAV智能调度系统已经运行起来了！🚁✨