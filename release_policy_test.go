package main

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseWorkflowDoesNotEmbedCloudSecrets(t *testing.T) {
	workflow := readReleaseWorkflow(t)

	for _, forbidden := range []string{
		"BAIDU_TOKEN",
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
