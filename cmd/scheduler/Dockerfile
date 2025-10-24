# 多阶段构建Dockerfile for UAV调度器
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git python3 py3-pip

# 复制go mod文件
COPY cmd/scheduler/go.mod cmd/scheduler/go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY cmd/scheduler/ ./

# 编译二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o uav-scheduler .

# 运行阶段
FROM python:3.11-slim

# 安装必要的系统包
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 创建应用用户
RUN useradd -m -u 1001 uav-scheduler

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/uav-scheduler .

# 复制NSGA-II算法脚本
COPY cmd/scheduler/RUN2.py .

# 安装Python依赖
RUN pip3 install --no-cache-dir numpy scipy matplotlib deap

# 设置文件权限
RUN chmod +x uav-scheduler RUN2.py

# 创建临时目录
RUN mkdir -p /tmp && chmod 777 /tmp

# 切换到应用用户
USER uav-scheduler

# 暴露端口
EXPOSE 8080 8081

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

# 启动命令
CMD ["./uav-scheduler"]