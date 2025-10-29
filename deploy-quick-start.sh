#!/bin/bash

# 🚀 UAV Monitor - 一键部署脚本
# 适用于新的Kubernetes集群

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查必要工具
check_prerequisites() {
    log_info "检查前置条件..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装或未配置"
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "无法连接到Kubernetes集群"
        exit 1
    fi

    log_success "前置条件检查通过"
}

# 创建命名空间
create_namespaces() {
    log_info "创建命名空间..."

    kubectl create namespace uav-demo --dry-run=client -o yaml | kubectl apply -f -
    kubectl create namespace uav-system --dry-run=client -o yaml | kubectl apply -f -

    log_success "命名空间创建完成"
}

# 安装Istio Ambient模式
install_istio() {
    log_info "检查Istio安装状态..."

    if kubectl get pods -n istio-system | grep -q "istiod"; then
        log_warning "Istio 已安装，跳过安装步骤"
        return
    fi

    log_info "下载并安装Istio Ambient模式..."

    # 下载Istio
    if [ ! -d "istio-1.27.3" ]; then
        curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.27.3 sh -
    fi

    export PATH="$PATH:$PWD/istio-1.27.3/bin"

    # 安装Ambient模式
    istioctl install --set profile=ambient -y

    # 为UAV命名空间启用Ambient
    kubectl label namespace uav-demo istio.io/rev=default --overwrite
    kubectl label namespace uav-system istio.io/rev=default --overwrite

    log_success "Istio Ambient模式安装完成"
}

# 部署CRD
deploy_crds() {
    log_info "部署CRD定义..."

    # 部署UAV监控CRD
    if [ -f "api/crd/uav-metrics.yaml" ]; then
        kubectl apply -f api/crd/uav-metrics.yaml
        log_success "UAV Metrics CRD部署完成"
    else
        log_error "找不到 api/crd/uav-metrics.yaml"
        exit 1
    fi

    # 部署UAV路由CRD
    if [ -f "api/crd/uav-routing.yaml" ]; then
        kubectl apply -f api/crd/uav-routing.yaml
        log_success "UAV Routing CRD部署完成"
    else
        log_error "找不到 api/crd/uav-routing.yaml"
        exit 1
    fi

    # 验证CRD
    log_info "验证CRD部署..."
    kubectl get crd | grep uav || {
        log_error "CRD部署失败"
        exit 1
    }

    log_success "所有CRD部署完成"
}

# 部署UAV节点
deploy_uav_nodes() {
    log_info "部署UAV节点..."

    # 查找UAV节点部署文件
    UAV_DEPLOY_FILE=""
    if [ -f "infrastructure/kubernetes/uav-nodes.yaml" ]; then
        UAV_DEPLOY_FILE="infrastructure/kubernetes/uav-nodes.yaml"
    elif [ -f "deploy/uav-nodes.yaml" ]; then
        UAV_DEPLOY_FILE="deploy/uav-nodes.yaml"
    else
        log_warning "找不到UAV节点部署文件，创建默认部署..."
        create_default_uav_deployment
        UAV_DEPLOY_FILE="temp-uav-nodes.yaml"
    fi

    kubectl apply -f "$UAV_DEPLOY_FILE"

    log_info "等待UAV节点就绪..."
    kubectl wait --for=condition=ready pod -l app=uav-node -n uav-demo --timeout=120s

    log_success "UAV节点部署完成"
}

# 创建默认UAV部署（如果找不到部署文件）
create_default_uav_deployment() {
    log_info "创建默认UAV节点部署..."

    cat > temp-uav-nodes.yaml << 'EOF'
---
# UAV Smart Service
apiVersion: v1
kind: Service
metadata:
  name: uav-smart-service
  namespace: uav-demo
spec:
  selector:
    app: uav-node
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
---
# UAV Node 1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-node-1
  namespace: uav-demo
  labels:
    app: uav-node
    node: uav-node-1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: uav-node
      node: uav-node-1
  template:
    metadata:
      labels:
        app: uav-node
        node: uav-node-1
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "echo 'UAV Node 1 - Downtown LA (34.0522, -118.2437)' > /usr/share/nginx/html/index.html"]
---
# UAV Node 2
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-node-2
  namespace: uav-demo
  labels:
    app: uav-node
    node: uav-node-2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: uav-node
      node: uav-node-2
  template:
    metadata:
      labels:
        app: uav-node
        node: uav-node-2
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "echo 'UAV Node 2 - Santa Monica (34.0195, -118.4912)' > /usr/share/nginx/html/index.html"]
---
# UAV Node 3
apiVersion: apps/v1
kind: Deployment
metadata:
  name: uav-node-3
  namespace: uav-demo
  labels:
    app: uav-node
    node: uav-node-3
spec:
  replicas: 1
  selector:
    matchLabels:
      app: uav-node
      node: uav-node-3
  template:
    metadata:
      labels:
        app: uav-node
        node: uav-node-3
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "echo 'UAV Node 3 - Pasadena (34.1478, -118.1445)' > /usr/share/nginx/html/index.html"]
EOF
}

# 配置智能路由
configure_routing() {
    log_info "配置智能路由..."

    # 查找路由配置文件
    ROUTING_FILE=""
    if [ -f "infrastructure/istio/ambient/routing-rules.yaml" ]; then
        ROUTING_FILE="infrastructure/istio/ambient/routing-rules.yaml"
    else
        log_warning "找不到路由配置文件，创建默认路由规则..."
        create_default_routing_rules
        ROUTING_FILE="temp-routing-rules.yaml"
    fi

    kubectl apply -f "$ROUTING_FILE"

    log_success "路由配置完成"
}

# 创建默认路由规则（如果找不到配置文件）
create_default_routing_rules() {
    log_info "创建默认路由规则..."

    cat > temp-routing-rules.yaml << 'EOF'
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: uav-smart-service
  namespace: uav-demo
spec:
  hosts:
  - uav-smart-service
  http:
  - match:
    - headers:
        x-source-location:
          exact: "downtown-la"
    route:
    - destination:
        host: uav-smart-service
        subset: uav-node-1
      weight: 60
    - destination:
        host: uav-smart-service
        subset: uav-node-2
      weight: 25
    - destination:
        host: uav-smart-service
        subset: uav-node-3
      weight: 15
  - match:
    - headers:
        x-source-location:
          exact: "santa-monica"
    route:
    - destination:
        host: uav-smart-service
        subset: uav-node-2
      weight: 70
    - destination:
        host: uav-smart-service
        subset: uav-node-1
      weight: 20
    - destination:
        host: uav-smart-service
        subset: uav-node-3
      weight: 10
  - route:
    - destination:
        host: uav-smart-service
        subset: uav-node-1
      weight: 33
    - destination:
        host: uav-smart-service
        subset: uav-node-2
      weight: 33
    - destination:
        host: uav-smart-service
        subset: uav-node-3
      weight: 34
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: uav-smart-service
  namespace: uav-demo
spec:
  host: uav-smart-service
  subsets:
  - name: uav-node-1
    labels:
      node: uav-node-1
  - name: uav-node-2
    labels:
      node: uav-node-2
  - name: uav-node-3
    labels:
      node: uav-node-3
EOF
}

# 验证部署
verify_deployment() {
    log_info "验证部署状态..."

    # 检查Pod状态
    log_info "检查Pod状态..."
    kubectl get pods -n uav-demo

    # 检查Istio状态
    log_info "检查Istio状态..."
    kubectl get pods -n istio-system | head -5

    # 检查CRD
    log_info "检查CRD..."
    kubectl get crd | grep uav

    # 检查路由
    log_info "检查路由配置..."
    kubectl get virtualservices -n uav-demo

    log_success "部署验证完成"
}

# 创建测试客户端
create_test_client() {
    log_info "创建测试客户端..."

    kubectl run test-client --image=curlimages/curl:latest -n uav-demo --restart=Never -- sleep=3600 --dry-run=client -o yaml | kubectl apply -f -

    log_info "等待测试客户端就绪..."
    kubectl wait --for=condition=ready pod/test-client -n uav-demo --timeout=30s

    log_success "测试客户端创建完成"
}

# 运行测试
run_tests() {
    log_info "运行路由测试..."

    # 测试默认路由
    log_info "测试默认路由..."
    kubectl exec -it test-client -n uav-demo -- curl -s http://uav-smart-service.uav-demo.svc.cluster.local || log_warning "默认路由测试失败"

    # 测试Downtown LA路由
    log_info "测试Downtown LA路由..."
    kubectl exec -it test-client -n uav-demo -- curl -H "x-source-location: downtown-la" -s http://uav-smart-service.uav-demo.svc.cluster.local || log_warning "Downtown LA路由测试失败"

    # 测试Santa Monica路由
    log_info "测试Santa Monica路由..."
    kubectl exec -it test-client -n uav-demo -- curl -H "x-source-location: santa-monica" -s http://uav-smart-service.uav-demo.svc.cluster.local || log_warning "Santa Monica路由测试失败"

    log_success "路由测试完成"
}

# 清理临时文件
cleanup() {
    log_info "清理临时文件..."
    rm -f temp-uav-nodes.yaml temp-routing-rules.yaml
}

# 显示成功信息
show_success() {
    log_success "🎉 UAV Monitor系统部署成功！"
    echo ""
    echo "📊 下一步操作："
    echo "1. 查看Pod状态: kubectl get pods -n uav-demo"
    echo "2. 查看UAV数据: kubectl get uavmetrics -n uav-demo"
    echo "3. 查看路由配置: kubectl get virtualservices -n uav-demo"
    echo "4. 继续测试: kubectl exec -it test-client -n uav-demo -- sh"
    echo ""
    echo "📚 更多信息请参考："
    echo "- 快速开始指南: QUICK_START.md"
    echo "- 项目结构: PROJECT_STRUCTURE.md"
    echo "- 迁移指南: MIGRATION_GUIDE.md"
    echo ""
    echo "🚁 恭喜！您的智能UAV集群已经准备就绪！"
}

# 主函数
main() {
    echo "🚀 UAV Monitor - 一键部署脚本"
    echo "================================"
    echo ""

    check_prerequisites
    create_namespaces
    install_istio
    deploy_crds
    deploy_uav_nodes
    configure_routing
    verify_deployment
    create_test_client
    run_tests
    cleanup
    show_success
}

# 错误处理
trap 'log_error "部署过程中发生错误，请检查上面的错误信息"; cleanup; exit 1' ERR

# 执行主函数
main "$@"