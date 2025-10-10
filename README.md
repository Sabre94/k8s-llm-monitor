# K8s LLM Monitor

基于大语言模型的K8s集群智能监控系统

## 功能特性

- 🤖 **智能分析**: 使用LLM进行深度故障诊断和根因分析
- 🔍 **网络诊断**: 智能分析Pod间通信问题和网络策略
- 📊 **实时监控**: 实时收集和分析K8s集群状态
- 🚨 **异常检测**: 基于AI的异常模式识别
- 💬 **自然语言交互**: 用自然语言查询集群状态和问题
- 🔄 **自动修复**: 智能生成和执行修复命令（可选）

## 快速开始

### 环境要求

- Go 1.21+
- K8s集群访问权限
- OpenAI API Key或其他LLM服务

### 安装

1. 克隆项目
```bash
git clone https://github.com/yourusername/k8s-llm-monitor.git
cd k8s-llm-monitor
```

2. 初始化依赖
```bash
make init
```

3. 配置环境变量
```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # 可选
```

4. 编辑配置文件
```bash
cp configs/config.yaml.example configs/config.yaml
# 编辑configs/config.yaml
```

5. 运行
```bash
make run
```

### 开发

```bash
# 开发模式运行
make dev

# 运行测试
make test

# 代码检查
make lint

# 构建生产版本
make build-prod
```

## API文档

### 健康检查
```
GET /health
```

### 获取集群状态
```
GET /api/v1/cluster/status
```

### 分析Pod通信
```
POST /api/v1/analyze/pod-communication
{
  "pod_a": "namespace/pod-name",
  "pod_b": "namespace/pod-name"
}
```

### 自然语言查询
```
POST /api/v1/query
{
  "question": "为什么我的pod频繁重启？"
}
```

## 配置说明

主要配置项：

- `server`: 服务器配置
- `k8s`: K8s集群连接配置
- `llm`: LLM服务配置
- `storage`: 数据存储配置
- `monitoring`: 监控配置

详细配置请参考 `configs/config.yaml`

## 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Frontend  │    │   API Server    │    │   Analysis      │
│                 │◄──►│                 │◄──►│   Engine        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Data Storage  │    │   K8s Client    │
                       │                 │    │                 │
                       └─────────────────┘    └─────────────────┘
                                                       │
                                                       ▼
                                              ┌─────────────────┐
                                              │   K8s Cluster   │
                                              │                 │
                                              └─────────────────┘
```

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

MIT License