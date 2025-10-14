# UAV 飞控模拟器使用指南

## 概述

本系统在每个 Kubernetes 节点上部署一个虚拟无人机飞控（UAV Agent），模拟真实无人机的飞行状态和 MAVLink 协议通信。

## 架构

```
┌────────────────────────────────────────────────────────────┐
│                    K8s Cluster                             │
│                                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │   Node 1    │  │   Node 2    │  │   Node 3    │      │
│  │             │  │             │  │             │      │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │      │
│  │ │ UAV Pod │ │  │ │ UAV Pod │ │  │ │ UAV Pod │ │      │
│  │ │ :9090   │ │  │ │ :9090   │ │  │ │ :9090   │ │      │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │      │
│  │  (UAV-1)    │  │  (UAV-2)    │  │  (UAV-3)    │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
│         ▲                ▲                ▲               │
│         └────────────────┴────────────────┘               │
│              K8s LLM Monitor (监控中心)                    │
└────────────────────────────────────────────────────────────┘
```

## 部署

### 1. 构建并部署

```bash
cd /Users/xiabin/Project/k8s-llm-monitor

# 一键构建和部署
./scripts/build-and-deploy-uav-agent.sh
```

### 2. 验证部署

```bash
# 查看UAV Agent Pod
kubectl get pods -l app=uav-agent -o wide

# 查看日志
kubectl logs -l app=uav-agent --tail=20

# 测试健康检查
curl http://localhost:9090/health
```

## 模拟的飞控数据

### 1. GPS 数据
```json
{
  "latitude": 39.9042,           // 纬度（度）
  "longitude": 116.4074,         // 经度（度）
  "altitude": 50.0,              // 海拔高度（米）
  "relative_altitude": 50.0,     // 相对起飞点高度（米）
  "hdop": 1.0,                   // 水平精度因子
  "satellite_count": 12,         // 卫星数量
  "fix_type": 3,                 // GPS定位类型（3=3D定位）
  "ground_speed": 5.0,           // 地面速度（m/s）
  "course_over_ground": 90.0     // 航向（度）
}
```

### 2. 姿态数据
```json
{
  "roll": 5.0,                   // 横滚角（度）
  "pitch": 3.0,                  // 俯仰角（度）
  "yaw": 90.0,                   // 偏航角/航向（度）
  "roll_rate": 0.5,              // 横滚角速度（度/秒）
  "pitch_rate": 0.3,             // 俯仰角速度（度/秒）
  "yaw_rate": 1.0                // 偏航角速度（度/秒）
}
```

### 3. 飞行状态
```json
{
  "mode": "AUTO",                // 飞行模式
  "armed": true,                 // 是否解锁
  "airspeed": 5.5,              // 空速（m/s）
  "ground_speed": 5.0,          // 地速（m/s）
  "vertical_speed": 0.5,        // 垂直速度（m/s）
  "throttle_percent": 50.0      // 油门百分比
}
```

### 4. 电池信息
```json
{
  "voltage": 22.2,              // 电压（V）
  "current": 10.5,              // 电流（A）
  "remaining_percent": 85.0,    // 剩余电量（%）
  "remaining_capacity": 4250.0, // 剩余容量（mAh）
  "total_capacity": 5000.0,     // 总容量（mAh）
  "temperature": 28.0,          // 温度（°C）
  "cell_count": 6,              // 电芯数量（6S）
  "time_remaining": 1800        // 预计剩余时间（秒）
}
```

## API 接口

### 查询接口

| 接口 | 方法 | 描述 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/v1/state` | GET | 获取完整状态 |
| `/api/v1/gps` | GET | 获取GPS数据 |
| `/api/v1/attitude` | GET | 获取姿态数据 |
| `/api/v1/flight` | GET | 获取飞行数据 |
| `/api/v1/battery` | GET | 获取电池数据 |

### 控制接口

| 接口 | 方法 | 描述 |
|------|------|------|
| `/api/v1/command/arm` | POST | 解锁（准备飞行） |
| `/api/v1/command/disarm` | POST | 上锁（停止飞行） |
| `/api/v1/command/takeoff` | POST | 起飞 |
| `/api/v1/command/land` | POST | 降落 |
| `/api/v1/command/rtl` | POST | 返航（Return To Launch） |
| `/api/v1/command/mode` | POST | 设置飞行模式 |

## 使用示例

### 1. 获取所有无人机状态

```bash
# 获取所有UAV Agent的Pod
kubectl get pods -l app=uav-agent -o json | \
  jq -r '.items[] | .status.hostIP + " " + .spec.nodeName' | \
  while read ip node; do
    echo "=== UAV on $node ($ip) ==="
    curl -s http://$ip:9090/api/v1/state | jq .
    echo ""
  done
```

### 2. 模拟飞行流程

```bash
# 1. 解锁
curl -X POST http://localhost:9090/api/v1/command/arm

# 2. 起飞到50米
curl -X POST http://localhost:9090/api/v1/command/takeoff \
  -H 'Content-Type: application/json' \
  -d '{"altitude": 50}'

# 3. 设置为自动飞行模式
curl -X POST http://localhost:9090/api/v1/command/mode \
  -H 'Content-Type: application/json' \
  -d '{"mode": "AUTO"}'

# 4. 查看飞行状态
curl http://localhost:9090/api/v1/flight | jq .

# 5. 查看GPS位置
curl http://localhost:9090/api/v1/gps | jq .

# 6. 查看电池状态
curl http://localhost:9090/api/v1/battery | jq .

# 7. 返航
curl -X POST http://localhost:9090/api/v1/command/rtl

# 8. 降落
curl -X POST http://localhost:9090/api/v1/command/land

# 9. 上锁
curl -X POST http://localhost:9090/api/v1/command/disarm
```

### 3. 监控电池电量

```bash
# 持续监控所有无人机电池
watch -n 2 'kubectl get pods -l app=uav-agent -o json | \
  jq -r ".items[] | .spec.nodeName + \": \" + .status.hostIP" | \
  while read info; do
    node=$(echo $info | cut -d: -f1)
    ip=$(echo $info | cut -d: -f2 | xargs)
    battery=$(curl -s http://$ip:9090/api/v1/battery | jq -r ".data.remaining_percent")
    echo "$node: Battery $battery%"
  done'
```

## 飞行模式说明

- **MANUAL**: 手动模式（完全手动控制）
- **STABILIZE**: 稳定模式（姿态稳定）
- **LOITER**: 定点模式（GPS定点悬停）
- **AUTO**: 自动模式（执行航线）
- **RTL**: 返航模式（Return To Launch）
- **LAND**: 降落模式

## 模拟特性

### 1. 飞行轨迹
- 解锁并设置为 AUTO 模式后，无人机会模拟圆形飞行轨迹
- 中心点：北京天安门附近（可配置）
- 半径：约100米
- 飞行高度：50米 + 正弦波动（±10米）

### 2. 电池消耗
- 解锁后自动模拟电池放电
- 放电速率：约0.1%/秒（可配置）
- 电量 < 20%：触发警告
- 电量 < 10%：触发严重警告，建议返航

### 3. 传感器数据
- GPS：3D定位，12颗卫星
- 姿态：实时模拟横滚、俯仰、偏航角
- 速度：地速、空速、垂直速度
- 温度：电池温度随放电增加

## 集成到监控系统

在主监控服务中添加以下功能：

```go
// 获取所有UAV状态
func (m *Manager) GetAllUAVStates(ctx context.Context) ([]*UAVState, error) {
    // 1. 获取所有 uav-agent Pod
    pods, err := m.k8sClient.GetPods("default")
    if err != nil {
        return nil, err
    }

    var states []*UAVState
    for _, pod := range pods {
        if pod.Labels["app"] == "uav-agent" {
            // 2. 调用每个UAV Agent的API
            url := fmt.Sprintf("http://%s:9090/api/v1/state", pod.IP)
            resp, err := http.Get(url)
            if err != nil {
                continue
            }
            defer resp.Body.Close()

            var state UAVState
            if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
                continue
            }

            states = append(states, &state)
        }
    }

    return states, nil
}
```

## 故障排查

### Pod 无法启动
```bash
# 查看Pod状态
kubectl describe pod -l app=uav-agent

# 查看日志
kubectl logs -l app=uav-agent --tail=50
```

### 端口冲突
如果 9090 端口被占用，修改 `deployments/uav-agent-daemonset.yaml` 中的端口配置。

### 镜像问题
```bash
# 重新构建镜像
docker build -f build/Dockerfile.uav-agent -t uav-agent:latest .

# 重新导入到k3d
k3d image import uav-agent:latest -c k8s-llm-monitor

# 重启Pod
kubectl rollout restart daemonset/uav-agent
```

## 下一步

1. **添加蜂群通信**：实现UAV之间的编队通信
2. **任务规划**：添加航点任务和自动航线
3. **故障注入**：模拟GPS失效、通信中断等故障
4. **3D可视化**：Web界面显示无人机位置和状态
5. **网络延迟矩阵**：测量所有UAV之间的通信延迟

## 参考资料

- [MAVLink协议](https://mavlink.io/)
- [ArduPilot文档](https://ardupilot.org/)
- [PX4飞控](https://px4.io/)
