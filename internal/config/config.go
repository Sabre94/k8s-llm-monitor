package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	K8s        K8sConfig        `mapstructure:"k8s"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Metrics    MetricsConfig    `mapstructure:"metrics"` // 新增指标采集配置
	Analysis   AnalysisConfig   `mapstructure:"analysis"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host  string `mapstructure:"host"`
	Port  int    `mapstructure:"port"`
	Debug bool   `mapstructure:"debug"`
}

// K8sConfig K8s配置
type K8sConfig struct {
	Kubeconfig      string `mapstructure:"kubeconfig"`
	Namespace       string `mapstructure:"namespace"`
	WatchNamespaces string `mapstructure:"watch_namespaces"`
}

// LLMConfig LLM配置
type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
	Timeout     int     `mapstructure:"timeout"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type     string         `mapstructure:"type"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Postgres PostgresConfig `mapstructure:"postgres"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// PostgresConfig PostgreSQL配置
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	MetricsInterval int `mapstructure:"metrics_interval"`
	EventRetention  int `mapstructure:"event_retention"`
	LogRetention    int `mapstructure:"log_retention"`
}

// MetricsConfig 指标采集配置
type MetricsConfig struct {
	Enabled         bool     `mapstructure:"enabled"`           // 是否启用指标采集
	CollectInterval int      `mapstructure:"collect_interval"`  // 采集间隔（秒）
	Namespaces      []string `mapstructure:"namespaces"`        // 要监控的命名空间列表
	EnableNode      bool     `mapstructure:"enable_node"`       // 启用节点指标
	EnablePod       bool     `mapstructure:"enable_pod"`        // 启用Pod指标
	EnableNetwork   bool     `mapstructure:"enable_network"`    // 启用网络指标
	EnableCustom    bool     `mapstructure:"enable_custom"`     // 启用自定义CRD指标
	CacheRetention  int      `mapstructure:"cache_retention"`   // 缓存保留时间（秒）
}

// AnalysisConfig 分析配置
type AnalysisConfig struct {
	EnablePrediction bool `mapstructure:"enable_prediction"`
	EnableAutoFix    bool `mapstructure:"enable_auto_fix"`
	MaxContextEvents int  `mapstructure:"max_context_events"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)

	// 设置默认值
	setDefaults()

	// 读取环境变量
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析环境变量
	processEnvVars()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.debug", false)

	viper.SetDefault("k8s.kubeconfig", "")
	viper.SetDefault("k8s.namespace", "default")
	viper.SetDefault("k8s.watch_namespaces", "default")

	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.model", "gpt-4")
	viper.SetDefault("llm.max_tokens", 2000)
	viper.SetDefault("llm.temperature", 0.1)
	viper.SetDefault("llm.timeout", 30)

	viper.SetDefault("storage.type", "memory")

	viper.SetDefault("monitoring.metrics_interval", 30)
	viper.SetDefault("monitoring.event_retention", 168)
	viper.SetDefault("monitoring.log_retention", 24)

	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.collect_interval", 30)
	viper.SetDefault("metrics.namespaces", []string{"default"})
	viper.SetDefault("metrics.enable_node", true)
	viper.SetDefault("metrics.enable_pod", true)
	viper.SetDefault("metrics.enable_network", false)
	viper.SetDefault("metrics.enable_custom", false)
	viper.SetDefault("metrics.cache_retention", 300)

	viper.SetDefault("analysis.enable_prediction", true)
	viper.SetDefault("analysis.enable_auto_fix", false)
	viper.SetDefault("analysis.max_context_events", 100)

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
}

// processEnvVars 处理环境变量
func processEnvVars() {
	// 处理LLM API Key
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.Set("llm.api_key", apiKey)
	}

	// 处理Base URL
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		viper.Set("llm.base_url", baseURL)
	}
}
