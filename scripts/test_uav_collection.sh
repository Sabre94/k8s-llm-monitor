#!/bin/bash

# UAV数据采集测试脚本
# 用于验证UAV指标采集功能

set -e

# 配置
SERVER_URL="http://localhost:8081"
COLOR_GREEN='\033[0;32m'
COLOR_RED='\033[0;31m'
COLOR_YELLOW='\033[1;33m'
COLOR_NC='\033[0m' # No Color

# 打印函数
print_success() {
    echo -e "${COLOR_GREEN}✓ $1${COLOR_NC}"
}

print_error() {
    echo -e "${COLOR_RED}✗ $1${COLOR_NC}"
}

print_info() {
    echo -e "${COLOR_YELLOW}ℹ $1${COLOR_NC}"
}

print_header() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

# 检查依赖
check_dependencies() {
    print_header "检查依赖"

    if ! command -v curl &> /dev/null; then
        print_error "curl未安装"
        exit 1
    fi
    print_success "curl已安装"

    if ! command -v jq &> /dev/null; then
        print_error "jq未安装，某些功能可能无法使用"
        echo "安装方法: brew install jq (macOS) 或 apt-get install jq (Linux)"
    else
        print_success "jq已安装"
    fi

    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl未安装"
        exit 1
    fi
    print_success "kubectl已安装"
}

# 检查服务器状态
check_server() {
    print_header "检查服务器状态"

    if curl -s -f "${SERVER_URL}/health" > /dev/null 2>&1; then
        print_success "服务器运行中"
        curl -s "${SERVER_URL}/health" | jq . 2>/dev/null || cat
    else
        print_error "服务器未运行或无法访问"
        print_info "请确保服务器在${SERVER_URL}上运行"
        exit 1
    fi
}

# 检查UAV Agent部署
check_uav_agents() {
    print_header "检查UAV Agent部署"

    UAV_COUNT=$(kubectl get pods -l app=uav-agent --field-selector=status.phase=Running -o json | jq '.items | length')

    if [ "$UAV_COUNT" -eq 0 ]; then
        print_error "未找到运行中的UAV Agent Pod"
        print_info "运行以下命令部署UAV Agent:"
        echo "  kubectl apply -f deployments/uav-agent-daemonset.yaml"
        exit 1
    else
        print_success "找到 ${UAV_COUNT} 个运行中的UAV Agent"
        kubectl get pods -l app=uav-agent -o wide
    fi
}

# 测试获取所有UAV
test_get_all_uavs() {
    print_header "测试: 获取所有UAV状态"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    if echo "$RESPONSE" | jq -e '.status == "success"' > /dev/null 2>&1; then
        UAV_COUNT=$(echo "$RESPONSE" | jq '.count')
        print_success "成功获取 ${UAV_COUNT} 个UAV的状态"

        # 显示摘要
        echo "$RESPONSE" | jq -r '.data | to_entries[] | "\(.key): Battery \(.value.battery.remaining_percent)%, Mode: \(.value.flight.mode), Status: \(.value.health.system_status)"'
    else
        print_error "获取UAV状态失败"
        echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"
        exit 1
    fi
}

# 测试获取单个UAV
test_get_single_uav() {
    print_header "测试: 获取单个UAV状态"

    # 获取第一个节点名
    NODE_NAME=$(kubectl get pods -l app=uav-agent -o jsonpath='{.items[0].spec.nodeName}')

    if [ -z "$NODE_NAME" ]; then
        print_error "无法获取节点名"
        return 1
    fi

    print_info "测试节点: ${NODE_NAME}"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav/${NODE_NAME}")

    if echo "$RESPONSE" | jq -e '.status == "success"' > /dev/null 2>&1; then
        print_success "成功获取节点 ${NODE_NAME} 的UAV状态"
        echo "$RESPONSE" | jq '.data' 2>/dev/null || echo "$RESPONSE"
    else
        print_error "获取单个UAV状态失败"
        echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"
        return 1
    fi
}

# 测试数据完整性
test_data_integrity() {
    print_header "测试: 数据完整性检查"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    # 检查必要字段
    CHECKS=(
        '.data | to_entries[0].value.uav_id != null'
        '.data | to_entries[0].value.gps != null'
        '.data | to_entries[0].value.battery != null'
        '.data | to_entries[0].value.flight != null'
        '.data | to_entries[0].value.health != null'
    )

    for check in "${CHECKS[@]}"; do
        if echo "$RESPONSE" | jq -e "$check" > /dev/null 2>&1; then
            print_success "字段检查通过: $check"
        else
            print_error "字段检查失败: $check"
        fi
    done
}

# 监控低电量UAV
test_low_battery_monitoring() {
    print_header "测试: 低电量监控"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    LOW_BATTERY=$(echo "$RESPONSE" | jq -r '.data | to_entries[] | select(.value.battery.remaining_percent < 20) | "\(.key): \(.value.battery.remaining_percent)%"')

    if [ -z "$LOW_BATTERY" ]; then
        print_success "所有UAV电量正常 (>20%)"
    else
        print_error "发现低电量UAV:"
        echo "$LOW_BATTERY"
    fi
}

# 监控GPS信号
test_gps_monitoring() {
    print_header "测试: GPS信号监控"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    WEAK_GPS=$(echo "$RESPONSE" | jq -r '.data | to_entries[] | select(.value.gps.satellite_count < 10) | "\(.key): \(.value.gps.satellite_count) satellites"')

    if [ -z "$WEAK_GPS" ]; then
        print_success "所有UAV GPS信号良好 (>=10颗卫星)"
    else
        print_error "发现GPS信号较弱的UAV:"
        echo "$WEAK_GPS"
    fi
}

# 健康状态检查
test_health_status() {
    print_header "测试: 健康状态检查"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    UNHEALTHY=$(echo "$RESPONSE" | jq -r '.data | to_entries[] | select(.value.health.system_status != "OK") | "\(.key): \(.value.health.system_status)"')

    if [ -z "$UNHEALTHY" ]; then
        print_success "所有UAV健康状态正常"
    else
        print_error "发现健康状态异常的UAV:"
        echo "$UNHEALTHY"
    fi
}

# 性能测试
test_performance() {
    print_header "测试: 采集性能"

    print_info "执行10次请求测试..."

    TOTAL_TIME=0
    for i in {1..10}; do
        START=$(date +%s%N)
        curl -s "${SERVER_URL}/api/v1/metrics/uav" > /dev/null
        END=$(date +%s%N)
        TIME=$((($END - $START) / 1000000)) # 转换为毫秒
        TOTAL_TIME=$(($TOTAL_TIME + $TIME))
    done

    AVG_TIME=$(($TOTAL_TIME / 10))
    print_success "平均响应时间: ${AVG_TIME}ms"

    if [ $AVG_TIME -lt 1000 ]; then
        print_success "性能良好 (<1秒)"
    elif [ $AVG_TIME -lt 3000 ]; then
        print_info "性能一般 (1-3秒)"
    else
        print_error "性能较慢 (>3秒)"
    fi
}

# 生成报告
generate_report() {
    print_header "测试报告"

    RESPONSE=$(curl -s "${SERVER_URL}/api/v1/metrics/uav")

    echo "采集时间: $(echo "$RESPONSE" | jq -r '.timestamp')"
    echo "UAV数量: $(echo "$RESPONSE" | jq '.count')"
    echo ""
    echo "详细状态:"
    echo "$RESPONSE" | jq -r '.data | to_entries[] | "  \(.key):\n    电池: \(.value.battery.remaining_percent)%\n    GPS: \(.value.gps.satellite_count)颗卫星\n    模式: \(.value.flight.mode)\n    状态: \(.value.health.system_status)\n"'
}

# 主函数
main() {
    echo ""
    echo "======================================"
    echo "  UAV数据采集功能测试"
    echo "======================================"
    echo ""

    check_dependencies
    check_server
    check_uav_agents

    test_get_all_uavs
    test_get_single_uav
    test_data_integrity
    test_low_battery_monitoring
    test_gps_monitoring
    test_health_status
    test_performance

    generate_report

    print_header "测试完成"
    print_success "所有测试通过！"
}

# 运行测试
main "$@"
