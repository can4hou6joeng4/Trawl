package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LicenseSecret 授权密钥，构建时通过 -ldflags 注入
var LicenseSecret string = ""

// GetMachineID 获取当前设备的唯一识别短码
func GetMachineID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	hash := sha256.Sum256([]byte(hostname + "legal-extractor-machine"))
	return strings.ToUpper(fmt.Sprintf("%x", hash)[:8])
}

// VerifyLicense 校验授权码是否合法
func VerifyLicense(machineID, licenseCode string) bool {
	if LicenseSecret == "" {
		return false
	}
	expected := GenerateLicense(machineID)
	return strings.EqualFold(licenseCode, expected)
}

// GenerateLicense 基于密钥生成授权码
func GenerateLicense(machineID string) string {
	if LicenseSecret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(LicenseSecret))
	mac.Write([]byte(machineID))
	raw := fmt.Sprintf("%x", mac.Sum(nil))
	code := strings.ToUpper(raw[:16])
	return fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
}

// IsActivated 检查是否已激活
func IsActivated() bool {
	if v == nil {
		return false
	}
	license := v.GetString("license_key")
	if license == "" {
		return false
	}
	return VerifyLicense(GetMachineID(), license)
}

// SaveLicense 保存授权码
func SaveLicense(code string) error {
	if v == nil {
		return fmt.Errorf("config system not initialized")
	}
	v.Set("license_key", code)

	// Fix: Config File "conf" Not Found error
	// 如果 Viper 没有关联配置文件（说明启动时未找到文件），直接 WriteConfig 会报错
	// 此时我们需要显式指定路径写入
	if v.ConfigFileUsed() == "" {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// 默认写入路径: ./config/conf.yaml
		configDir := filepath.Join(filepath.Dir(exePath), "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		configPath := filepath.Join(configDir, "conf.yaml")
		return v.WriteConfigAs(configPath)
	}

	return v.WriteConfig()
}
