#!/bin/bash

# 在所有 metrics 相关文件中替换导入
find . -name "*.go" -type f -exec sed -i '' 's|github.com/yourusername/k8s-llm-monitor/internal/metrics"|github.com/yourusername/k8s-llm-monitor/pkg/metrics"|g' {} \;

# 修复 sources 包中的类型引用
find ./internal/metrics/sources -name "*.go" -type f -exec sed -i '' 's|\*metrics\.|\*metricstypes.|g' {} \;
find ./internal/metrics/sources -name "*.go" -type f -exec sed -i '' 's|metrics\.|\metricstypes.|g' {} \;

# 修复 manager.go  
sed -i '' 's|\*metrics\.|\*metricstypes.|g' ./internal/metrics/manager.go
sed -i '' 's|metrics\.|\metricstypes.|g' ./internal/metrics/manager.go

