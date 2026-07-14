// license-gen 为客户机器生成激活码（所有者运维工具，随仓库分发但不进安装包）。
//
// 用法：
//
//	make license MACHINE=XXXXXXXX          # 推荐入口
//	go run ./cmd/license-gen XXXXXXXX      # 等价
//
// 机器码即应用「激活」界面显示的 8 位码。密钥来源（按优先级）：
//  1. 环境变量 LICENSE_SECRET
//  2. ~/.secrets/license-secret 文件
//
// 生成算法与应用端 config.GenerateLicense、网关端授权校验完全同构
// （直接复用应用端实现，杜绝三方漂移）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"legal-extractor/internal/config"
)

var machineIDPattern = regexp.MustCompile(`^[0-9A-F]{8}$`)

func main() {
	if len(os.Args) != 2 {
		fatal("用法: license-gen <机器码>（应用激活界面显示的 8 位码）")
	}

	machineID, err := normalizeMachineID(os.Args[1])
	if err != nil {
		fatal(err.Error())
	}

	secret, source, err := loadSecret()
	if err != nil {
		fatal(err.Error())
	}

	code, err := generate(machineID, secret)
	if err != nil {
		fatal(err.Error())
	}

	// 人读信息走 stderr，激活码单独走 stdout（便于脚本取用/管道复制）
	fmt.Fprintf(os.Stderr, "机器码: %s（密钥来源: %s）\n激活码: ", machineID, source)
	fmt.Println(code)
}

// normalizeMachineID 归一化并校验机器码（8 位十六进制，容忍大小写与首尾空白）。
func normalizeMachineID(raw string) (string, error) {
	id := strings.ToUpper(strings.TrimSpace(raw))
	if !machineIDPattern.MatchString(id) {
		return "", fmt.Errorf("机器码格式错误: %q（应为 8 位十六进制，如 70A2F8D6）", raw)
	}
	return id, nil
}

// loadSecret 读取授权密钥：环境变量优先，其次本地密钥文件。
func loadSecret() (secret, source string, err error) {
	if s := strings.TrimSpace(os.Getenv("LICENSE_SECRET")); s != "" {
		return s, "环境变量 LICENSE_SECRET", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("无法定位用户目录: %w", err)
	}
	path := filepath.Join(home, ".secrets", "license-secret")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("未找到授权密钥：请设置环境变量 LICENSE_SECRET，或将密钥存入 %s", path)
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "", "", fmt.Errorf("密钥文件为空: %s", path)
	}
	return s, path, nil
}

// generate 复用应用端实现出码（临时注入包级密钥变量，用后即还原）。
func generate(machineID, secret string) (string, error) {
	prev := config.LicenseSecret
	config.LicenseSecret = secret
	defer func() { config.LicenseSecret = prev }()

	code := config.GenerateLicense(machineID)
	if code == "" {
		return "", fmt.Errorf("激活码生成失败（密钥为空）")
	}
	return code, nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
