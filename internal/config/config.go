package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var (
	// BuildTime is injected via -ldflags at build time (Unix timestamp)
	BuildTime string = ""
)

const TrialDurationDays = 7

// TrialStatus represents the current trial state
type TrialStatus struct {
	IsActivated bool          `json:"isActivated"`
	IsExpired   bool          `json:"isExpired"`
	Remaining   time.Duration `json:"remaining"` // Duration until expiry
	Days        int           `json:"days"`      // Remaining whole days
	Hours       int           `json:"hours"`     // Remaining hours (modulo days)
}

// GetTrialStatus calculates the remaining trial time
func GetTrialStatus() TrialStatus {
	// 优先检查激活状态
	if IsActivated() {
		return TrialStatus{
			IsActivated: true,
			IsExpired:   false,
			Remaining:   9999 * time.Hour,
			Days:        999,
			Hours:       0,
		}
	}

	if BuildTime == "" {
		// Dev mode or local run without injection: no trial limit
		return TrialStatus{IsExpired: false, Remaining: 999 * time.Hour}
	}

	bt, err := strconv.ParseInt(BuildTime, 10, 64)
	if err != nil {
		return TrialStatus{IsExpired: false}
	}

	expiryTime := time.Unix(bt, 0).AddDate(0, 0, TrialDurationDays)
	remaining := time.Until(expiryTime)

	if remaining <= 0 {
		return TrialStatus{IsExpired: true, Remaining: 0}
	}

	return TrialStatus{
		IsExpired: false,
		Remaining: remaining,
		Days:      int(remaining.Hours() / 24),
		Hours:     int(remaining.Hours()) % 24,
	}
}

// Config 应用配置结构
type Config struct {
	Baidu BaiduConfig `mapstructure:"baidu"` // 旧配置节：仅 token 迁移读取（等价 ocr.token）
	Ocr   OcrConfig   `mapstructure:"ocr"`
}

// BaiduConfig 旧版配置节（v4.1 及之前），保留供老用户的 token 继续生效
type BaiduConfig struct {
	Token string `mapstructure:"token"`
}

// OcrConfig 云端 OCR 配置（v2 作业式接口）
type OcrConfig struct {
	Token      string `mapstructure:"token"`       // 自备 AI Studio Token → 直连官方接口（优先于网关）
	JobUrl     string `mapstructure:"job_url"`     // 直连模式作业接口
	GatewayUrl string `mapstructure:"gateway_url"` // 中继网关作业接口（应用不携带密钥）
	Model      string `mapstructure:"model"`
}

var (
	// 全局配置实例
	cfg *Config
	v   *viper.Viper
)

// Init 初始化配置系统
func Init(configPath string) error {
	v = viper.New()

	// 1. 获取可执行文件所在目录，确保生产环境下路径正确
	exePath, err := os.Executable()
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(exePath)
	} else {
		baseDir = "." // 回退到当前目录
	}

	// 设置默认值。云 OCR 密钥绝不内置——请求经中继网关换真实 Token，
	// 或由用户自备 Token 直连。
	v.SetDefault("baidu.token", "")
	v.SetDefault("ocr.token", "")
	v.SetDefault("ocr.job_url", "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs")
	v.SetDefault("ocr.gateway_url", "https://ocr-gw.bobochang.cn/api/v2/ocr/jobs")
	v.SetDefault("ocr.model", "PaddleOCR-VL-1.6")

	// 绑定环境变量 (前缀 LEGAL_EXTRACTOR_)
	v.SetEnvPrefix("LEGAL_EXTRACTOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 配置文件设置
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 默认查找路径
		v.SetConfigName("conf")
		v.SetConfigType("yaml")
		v.AddConfigPath(filepath.Join(baseDir, "config")) // 1. 锁定可执行文件同级的 config 目录
		v.AddConfigPath(baseDir)                          // 2. 锁定可执行文件同级
		v.AddConfigPath("./config")                       // 3. 兼容开发模式：当前工作目录下的 config
		v.AddConfigPath(".")                              // 4. 兼容开发模式：当前工作目录
	}

	// 尝试读取配置文件
	err = v.ReadInConfig()

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	// 如果最终密钥仍然为空，且之前是因为文件不存在才进来的，则创建默认模板
	if v.GetString("baidu.token") == "" {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			defaultPath := filepath.Join(baseDir, "config", "conf.yaml")
			if createErr := ensureConfigFile(defaultPath); createErr != nil {
				return fmt.Errorf("创建默认配置失败: %w", createErr)
			}
			v.SetConfigFile(defaultPath)
			_ = v.ReadInConfig()
		}
	}

	// 解析到结构体
	cfg = &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	return nil
}

// ensureConfigFile 确保配置文件存在，不存在则创建默认配置
func ensureConfigFile(configPath string) error {
	// 如果传入的是空或相对路径，尝试将其转换为基于可执行文件目录的绝对路径
	if !filepath.IsAbs(configPath) {
		exePath, _ := os.Executable()
		baseDir := filepath.Dir(exePath)
		if configPath == "" || configPath == "config/conf.yaml" {
			configPath = filepath.Join(baseDir, "config", "conf.yaml")
		} else {
			configPath = filepath.Join(baseDir, configPath)
		}
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入默认配置
	defaultConfig := `# Legal Extractor 配置文件
# 支持通过环境变量覆盖，前缀为 LEGAL_EXTRACTOR_
# 例如: LEGAL_EXTRACTOR_OCR_TOKEN=xxx

ocr:
  token: ""        # 可选：自备 AI Studio Token 时直连官方接口（优先于网关）
  gateway_url: "https://ocr-gw.bobochang.cn/api/v2/ocr/jobs"  # OCR 中继网关（默认即可用）
  model: "PaddleOCR-VL-1.6"
`
	return os.WriteFile(configPath, []byte(defaultConfig), 0644)
}

// Get 获取当前配置
func Get() *Config {
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

// GetOcr 获取云端 OCR 配置。经 viper 实时读取：环境变量修改即刻生效，
// conf.yaml 修改需重启应用。旧版 baidu.token 迁移生效（等价 ocr.token）。
func GetOcr() OcrConfig {
	if v == nil {
		return OcrConfig{}
	}
	o := OcrConfig{
		Token:      v.GetString("ocr.token"),
		JobUrl:     v.GetString("ocr.job_url"),
		GatewayUrl: v.GetString("ocr.gateway_url"),
		Model:      v.GetString("ocr.model"),
	}
	if o.Token == "" {
		o.Token = v.GetString("baidu.token")
	}
	return o
}

// GetLicenseKey 返回已保存的授权码（未激活为空串），供网关鉴权头使用。
func GetLicenseKey() string {
	if v == nil {
		return ""
	}
	return v.GetString("license_key")
}

// ResetForTest 清空全局配置状态，仅供测试构造「配置未初始化」场景使用。
func ResetForTest() {
	cfg = nil
	v = nil
}
