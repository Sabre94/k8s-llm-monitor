.PHONY: build run test clean docker-build docker-run deps

# 默认目标
all: build

# 构建应用
build:
	go build -o bin/server cmd/server/main.go
	go build -o bin/agent cmd/agent/main.go
	go build -o bin/scheduler cmd/scheduler/main.go

# 运行应用
run: build
	./bin/server -config ./configs/config.yaml

# 运行开发模式
dev:
	go run cmd/server/main.go -config ./configs/config.yaml

# 运行测试
test:
	go test -v ./...

# 清理构建文件
clean:
	rm -rf bin/

# 下载依赖
deps:
	go mod download
	go mod tidy

# 格式化代码
fmt:
	go fmt ./...

# 检查代码
lint:
	golangci-lint run

# 构建Docker镜像
docker-build:
	docker build -t k8s-llm-monitor:latest .

# 运行Docker容器
docker-run:
	docker run -p 8080:8080 --env-file .env k8s-llm-monitor:latest

# 初始化开发环境
init: deps
	cp configs/config.yaml.example configs/config.yaml 2>/dev/null || true
	echo "Please edit configs/config.yaml with your settings"

# 生产构建
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/server-linux cmd/server/main.go

# 安装工具
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 测试K8s连接
test-k8s:
	go run cmd/test-k8s/main.go -config ./configs/config.yaml
