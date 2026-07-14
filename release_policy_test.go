package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseWorkflowDoesNotEmbedCloudSecrets(t *testing.T) {
	workflow := readReleaseWorkflow(t)

	for _, forbidden := range []string{
		"BAIDU_TOKEN",
		"OCR_TOKEN",
		"OCR_GW_TOKEN",          // 网关的上游密钥只存在于网关部署环境，永不进入应用构建
		"OCR_GW_LICENSE_SECRET", // 同上：网关侧授权校验密钥
		"TENCENT_SECRET_ID",
		"TENCENT_SECRET_KEY",
		"baked_conf.yaml",
		"EmbeddedBaiduToken",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow must not embed cloud credential material: found %q", forbidden)
		}
	}
}

// TestConfigTemplateShipsNoCredential 守护「密钥永不随应用分发」（PRD 路线③）：
// 配置模板的 token 必须为空串，云端能力经中继网关（gateway_url 非密钥）提供。
func TestConfigTemplateShipsNoCredential(t *testing.T) {
	cfg := readTextFile(t, "internal/config/config.go")

	if !strings.Contains(cfg, `token: ""`) {
		t.Fatal("conf.yaml 模板的 ocr.token 必须为空串")
	}
	// AI Studio token 形态为 40 位十六进制：源码任何位置不得出现类似明文
	if tokenLike := regexp.MustCompile(`"[0-9a-f]{40}"`); tokenLike.MatchString(cfg) {
		t.Fatal("配置源码中疑似出现云端密钥明文")
	}
}

// TestReleaseWorkflowInjectsLicenseSecret 守护激活体系端到端可用：
// 发布构建必须注入 LicenseSecret（历史版本曾漏注入导致激活码永远无效）。
func TestReleaseWorkflowInjectsLicenseSecret(t *testing.T) {
	workflow := readReleaseWorkflow(t)

	if !strings.Contains(workflow, "config.LicenseSecret=") {
		t.Fatal("release workflow 必须经 ldflags 注入 config.LicenseSecret，否则发布产物无法激活")
	}
}

func TestReleaseWorkflowPublishesDesktopInstallerMatrix(t *testing.T) {
	workflow := readReleaseWorkflow(t)

	required := []string{
		"wails build -platform darwin/universal",
		"legal-extractor_${VERSION}_darwin_universal.dmg",
		"wails build -platform windows/amd64",
		"wails build -platform windows/amd64 -nsis",
		"legal-extractor_${VERSION}_windows_amd64_setup.exe",
	}
	for _, want := range required {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}

func TestWindowsInstallerIncludesOcrBridge(t *testing.T) {
	installer := readTextFile(t, "build/windows/installer/project.nsi")

	for _, want := range []string{
		`SetOutPath "$INSTDIR\bridge_bin"`,
		`File /nonfatal "..\..\bin\bridge_bin\*.*"`,
	} {
		if !strings.Contains(installer, want) {
			t.Fatalf("windows installer must include OCR bridge: missing %q", want)
		}
	}
}

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	return readTextFile(t, ".github/workflows/build.yml")
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
