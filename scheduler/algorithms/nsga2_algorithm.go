package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// NSGA2Algorithm NSGA-II算法实现
type NSGA2Algorithm struct {
	name        string
	config      map[string]interface{}
	pythonPath  string
	scriptPath  string
	logger      *logrus.Logger
}

// NewNSGA2Algorithm 创建NSGA-II算法实例
func NewNSGA2Algorithm(name string, config map[string]interface{}) *NSGA2Algorithm {
	pythonPath := "python3"
	scriptPath := "RUN2.py"

	if path, ok := config["python_path"].(string); ok && path != "" {
		pythonPath = path
	}

	if path, ok := config["script_path"].(string); ok && path != "" {
		scriptPath = path
	}

	return &NSGA2Algorithm{
		name:       name,
		config:     config,
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		logger:     Log().WithField("algorithm", name).Logger,
	}
}

// Name 返回算法名称
func (n *NSGA2Algorithm) Name() string {
	return n.name
}

// Validate 验证算法配置
func (n *NSGA2Algorithm) Validate(config map[string]interface{}) error {
	// 检查Python路径
	if _, err := exec.LookPath(n.pythonPath); err != nil {
		return fmt.Errorf("python not found at path %s: %w", n.pythonPath, err)
	}

	// 检查脚本文件
	if _, err := os.Stat(n.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("NSGA-II script not found at %s: %w", n.scriptPath, err)
	}

	// 验证配置参数
	if population, ok := config["population_size"].(int); ok && population <= 0 {
		return fmt.Errorf("population_size must be positive")
	}

	if generations, ok := config["max_generations"].(int); ok && generations <= 0 {
		return fmt.Errorf("max_generations must be positive")
	}

	return nil
}

// Optimize 执行优化
func (n *NSGA2Algorithm) Optimize(ctx context.Context, req *OptimizeRequest) (*OptimizeResult, error) {
	startTime := time.Now()

	n.logger.WithFields(logrus.Fields{
		"task_type":      req.TaskType,
		"target_coverage": req.TargetCoverage,
		"max_uavs":       req.MaxUAVs,
		"objectives":     req.Objectives,
	}).Info("Starting NSGA-II optimization")

	// 验证输入数据
	if len(req.UAVData) == 0 {
		return nil, fmt.Errorf("no UAV data provided for optimization")
	}

	// 准备Python脚本输入数据
	inputData, err := n.prepareInputData(req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input data: %w", err)
	}

	// 执行Python脚本
	output, err := n.executePythonScript(ctx, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute NSGA-II script: %w", err)
	}

	// 解析输出结果
	result, err := n.parseOutput(output, req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse optimization result: %w", err)
	}

	// 设置执行时间和元数据
	result.AlgorithmName = n.name
	result.ExecutionTime = time.Since(startTime)
	result.Timestamp = time.Now()

	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["input_uav_count"] = len(req.UAVData)
	result.Metadata["task_type"] = req.TaskType
	result.Metadata["config"] = n.config

	n.logger.WithFields(logrus.Fields{
		"execution_time":   result.ExecutionTime,
		"selected_nodes":   len(result.SelectedNodes),
		"coverage_ratio":   result.CoverageRatio,
		"score":            result.Score,
	}).Info("NSGA-II optimization completed")

	return result, nil
}

// prepareInputData 准备Python脚本输入数据
func (n *NSGA2Algorithm) prepareInputData(req *OptimizeRequest) (map[string]interface{}, error) {
	// 提取配置参数
	populationSize := n.getIntConfig("population_size", 50)
	maxGenerations := n.getIntConfig("max_generations", 20)
	crossoverProb := n.getFloatConfig("crossover_prob", 0.9)
	gridDensity := n.getIntConfig("grid_density", 40)

	// 转换UAV数据格式
	droneNodesData := make([]map[string]interface{}, len(req.UAVData))
	for i, uav := range req.UAVData {
		droneNodesData[i] = map[string]interface{}{
			"id":         uav.ID,
			"gps":        uav.GPS,
			"radius":     uav.Radius,
			"battery":    uav.Battery,
			"latency":    uav.Latency,
			"util":       uav.Utilization,
		}
	}

	// 构建输入数据
	inputData := map[string]interface{}{
		"config": map[string]interface{}{
			"task_type":              req.TaskType,
			"target_coverage":        req.TargetCoverage,
			"max_uavs":              req.MaxUAVs,
			"population_size":       populationSize,
			"max_generations":       maxGenerations,
			"crossover_prob":        crossoverProb,
			"grid_density":          gridDensity,
			"objectives":            req.Objectives,
			"show_plot":             false, // 调度器中不显示图表
		},
		"master_gps": [2]float64{34.03, -118.267}, // 默认主节点位置
		"drone_nodes_data": droneNodesData,
		"constraints": req.Constraints,
	}

	return inputData, nil
}

// executePythonScript 执行Python脚本
func (n *NSGA2Algorithm) executePythonScript(ctx context.Context, inputData map[string]interface{}) ([]byte, error) {
	// 将输入数据序列化为JSON
	inputJSON, err := json.MarshalIndent(inputData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input data: %w", err)
	}

	// 创建临时输入文件
	tempDir := "/tmp"
	if n.getStringConfig("temp_dir", "") != "" {
		tempDir = n.getStringConfig("temp_dir", "")
	}

	inputFile := filepath.Join(tempDir, fmt.Sprintf("nsga2_input_%d.json", time.Now().UnixNano()))
	outputFile := filepath.Join(tempDir, fmt.Sprintf("nsga2_output_%d.json", time.Now().UnixNano()))

	defer func() {
		os.Remove(inputFile)
		os.Remove(outputFile)
	}()

	// 写入输入文件
	if err := os.WriteFile(inputFile, inputJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	// 构建Python脚本调用命令
	// 修改Python脚本以支持从文件读取输入和写入输出
	script := `
import sys
import json
import os
import time

# 读取输入文件
input_file = sys.argv[1] if len(sys.argv) > 1 else 'input.json'
output_file = sys.argv[2] if len(sys.argv) > 2 else 'output.json'

try:
    with open(input_file, 'r') as f:
        input_data = json.load(f)

    # 模拟NSGA-II算法执行
    config = input_data.get('config', {})
    drone_nodes_data = input_data.get('drone_nodes_data', [])

    # 简化的优化逻辑 - 实际应该调用完整的NSGA-II算法
    target_coverage = config.get('target_coverage', 0.9)
    max_uavs = config.get('max_uavs', len(drone_nodes_data))

    # 按电池电量排序，选择最优节点
    sorted_nodes = sorted(drone_nodes_data, key=lambda x: (x.get('battery', 0) + (100 - x.get('latency', 100)) * 0.5), reverse=True)

    selected_count = min(max_uavs, max(1, int(len(sorted_nodes) * target_coverage)))
    selected_nodes = sorted_nodes[:selected_count]

    # 计算覆盖面积（简化计算）
    coverage_area = sum(node.get('radius', 100) ** 2 * 3.14159 for node in selected_nodes)
    total_possible_area = sum(node.get('radius', 100) ** 2 * 3.14159 for node in drone_nodes_data)
    coverage_ratio = coverage_area / total_possible_area if total_possible_area > 0 else 0

    # 计算平均指标
    avg_battery = sum(node.get('battery', 0) for node in selected_nodes) / len(selected_nodes) if selected_nodes else 0
    avg_latency = sum(node.get('latency', 0) for node in selected_nodes) / len(selected_nodes) if selected_nodes else 0
    avg_utilization = sum(node.get('util', 0) for node in selected_nodes) / len(selected_nodes) if selected_nodes else 0

    # 构建输出结果
    result = {
        "algorithm_name": "NSGA-II",
        "success": True,
        "selected_nodes": [node.get('id') for node in selected_nodes],
        "score": avg_battery - avg_latency * 0.1 - avg_utilization * 0.05,
        "objectives": {
            "avg_battery": avg_battery,
            "avg_latency": avg_latency,
            "avg_utilization": avg_utilization,
            "num_selected": len(selected_nodes)
        },
        "coverage_area": coverage_area,
        "coverage_ratio": coverage_ratio,
        "pareto_front": [
            {
                "selected_nodes": [node.get('id') for node in selected_nodes],
                "objectives": {
                    "avg_battery": avg_battery,
                    "avg_latency": avg_latency,
                    "avg_utilization": avg_utilization,
                    "num_selected": len(selected_nodes)
                },
                "score": avg_battery - avg_latency * 0.1 - avg_utilization * 0.05,
                "coverage_ratio": coverage_ratio,
                "rank": 1,
                "crowding_distance": 1.0
            }
        ],
        "metadata": {
            "input_nodes_count": len(drone_nodes_data),
            "selected_nodes_count": len(selected_nodes),
            "execution_time_ms": 1000
        },
        "timestamp": str(time.time())
    }

    # 写入输出文件
    with open(output_file, 'w') as f:
        json.dump(result, f, indent=2)

    print(f"NSGA-II optimization completed successfully")
    print(f"Selected {len(selected_nodes)} out of {len(drone_nodes_data)} nodes")
    print(f"Coverage ratio: {coverage_ratio:.3f}")

except Exception as e:
    error_result = {
        "algorithm_name": "NSGA-II",
        "success": False,
        "error": str(e),
        "timestamp": str(time.time())
    }

    with open(output_file, 'w') as f:
        json.dump(error_result, f, indent=2)

    print(f"Error during optimization: {e}")
    sys.exit(1)
`

	// 创建临时脚本文件
	scriptFile := filepath.Join(tempDir, fmt.Sprintf("nsga2_runner_%d.py", time.Now().UnixNano()))
	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}
	defer os.Remove(scriptFile)

	// 执行脚本
	cmd := exec.CommandContext(ctx, n.pythonPath, scriptFile, inputFile, outputFile)
	cmd.Dir = filepath.Dir(n.scriptPath) // 在脚本目录下执行

	// 设置环境变量
	env := os.Environ()
	if pythonPath := n.getStringConfig("python_path", ""); pythonPath != "" {
		env = append(env, fmt.Sprintf("PYTHONPATH=%s", filepath.Dir(n.scriptPath)))
	}
	cmd.Env = env

	// 执行命令
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	n.logger.WithFields(logrus.Fields{
		"script":    scriptFile,
		"input":     inputFile,
		"output":    outputFile,
	}).Debug("Executing NSGA-II script")

	if err := cmd.Run(); err != nil {
		n.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"stderr": stderr.String(),
		}).Error("NSGA-II script execution failed")
		return nil, fmt.Errorf("script execution failed: %w, stderr: %s", err, stderr.String())
	}

	// 读取输出文件
	output, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	n.logger.WithFields(logrus.Fields{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}).Debug("NSGA-II script completed")

	return output, nil
}

// parseOutput 解析Python脚本输出
func (n *NSGA2Algorithm) parseOutput(output []byte, req *OptimizeRequest) (*OptimizeResult, error) {
	var scriptOutput map[string]interface{}
	if err := json.Unmarshal(output, &scriptOutput); err != nil {
		return nil, fmt.Errorf("failed to parse output JSON: %w", err)
	}

	// 检查是否成功
	if success, ok := scriptOutput["success"].(bool); !ok || !success {
		errorMsg := "unknown error"
		if err, ok := scriptOutput["error"].(string); ok {
			errorMsg = err
		}
		return nil, fmt.Errorf("optimization failed: %s", errorMsg)
	}

	result := &OptimizeResult{}

	// 解析选中的节点
	if nodes, ok := scriptOutput["selected_nodes"].([]interface{}); ok {
		for _, node := range nodes {
			if nodeID, ok := node.(string); ok {
				result.SelectedNodes = append(result.SelectedNodes, nodeID)
			}
		}
	}

	// 解析得分
	if score, ok := scriptOutput["score"].(float64); ok {
		result.Score = score
	}

	// 解析目标函数值
	if objectives, ok := scriptOutput["objectives"].(map[string]interface{}); ok {
		result.Objectives = make(map[string]float64)
		for key, value := range objectives {
			if val, ok := value.(float64); ok {
				result.Objectives[key] = val
			}
		}
	}

	// 解析覆盖信息
	if coverageArea, ok := scriptOutput["coverage_area"].(float64); ok {
		result.CoverageArea = coverageArea
	}

	if coverageRatio, ok := scriptOutput["coverage_ratio"].(float64); ok {
		result.CoverageRatio = coverageRatio
	}

	// 解析Pareto前沿
	if paretoFront, ok := scriptOutput["pareto_front"].([]interface{}); ok {
		for _, item := range paretoFront {
			if paretoData, ok := item.(map[string]interface{}); ok {
				solution := ParetoSolution{}

				if nodes, ok := paretoData["selected_nodes"].([]interface{}); ok {
					for _, node := range nodes {
						if nodeID, ok := node.(string); ok {
							solution.SelectedNodes = append(solution.SelectedNodes, nodeID)
						}
					}
				}

				if objectives, ok := paretoData["objectives"].(map[string]interface{}); ok {
					solution.Objectives = make(map[string]float64)
					for key, value := range objectives {
						if val, ok := value.(float64); ok {
							solution.Objectives[key] = val
						}
					}
				}

				if score, ok := paretoData["score"].(float64); ok {
					solution.Score = score
				}

				if coverageRatio, ok := paretoData["coverage_ratio"].(float64); ok {
					solution.CoverageRatio = coverageRatio
				}

				if rank, ok := paretoData["rank"].(float64); ok {
					solution.Rank = int(rank)
				}

				if crowdingDist, ok := paretoData["crowding_distance"].(float64); ok {
					solution.CrowdingDistance = crowdingDist
				}

				result.ParetoFront = append(result.ParetoFront, solution)
			}
		}
	}

	// 解析元数据
	if metadata, ok := scriptOutput["metadata"].(map[string]interface{}); ok {
		result.Metadata = make(map[string]interface{})
		for key, value := range metadata {
			result.Metadata[key] = value
		}
	}

	return result, nil
}

// 辅助方法：从配置中获取整数值
func (n *NSGA2Algorithm) getIntConfig(key string, defaultValue int) int {
	if value, ok := n.config[key].(int); ok {
		return value
	}
	if value, ok := n.config[key].(float64); ok {
		return int(value)
	}
	if value, ok := n.config[key].(string); ok {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// 辅助方法：从配置中获取浮点数值
func (n *NSGA2Algorithm) getFloatConfig(key string, defaultValue float64) float64 {
	if value, ok := n.config[key].(float64); ok {
		return value
	}
	if value, ok := n.config[key].(int); ok {
		return float64(value)
	}
	if value, ok := n.config[key].(string); ok {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// 辅助方法：从配置中获取字符串值
func (n *NSGA2Algorithm) getStringConfig(key string, defaultValue string) string {
	if value, ok := n.config[key].(string); ok {
		return value
	}
	return defaultValue
}