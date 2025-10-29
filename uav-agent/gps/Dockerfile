# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o uav-gps-service ./cmd/uav-gps-service

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /app/uav-gps-service .

# 暴露端口
EXPOSE 9090

# 运行应用
CMD ["./uav-gps-service"]