package main

import (
	"strings"
	"testing"

	"legal-extractor/internal/config"
)

func TestNormalizeMachineID(t *testing.T) {
	for raw, want := range map[string]string{
		"70a2f8d6":   "70A2F8D6",
		" ABCD1234 ": "ABCD1234",
	} {
		got, err := normalizeMachineID(raw)
		if err != nil || got != want {
			t.Errorf("normalizeMachineID(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, bad := range []string{"", "short", "GHIJKLMN", "ABCD1234EXTRA"} {
		if _, err := normalizeMachineID(bad); err == nil {
			t.Errorf("normalizeMachineID(%q) 应报错", bad)
		}
	}
}

// TestGenerate_MatchesAppVerification 出码必须能通过应用端校验
// （同一密钥注入下 VerifyLicense 为真），且格式为 4-4-4-4。
func TestGenerate_MatchesAppVerification(t *testing.T) {
	const secret = "cross-check-secret"
	code, err := generate("70A2F8D6", secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 19 || strings.Count(code, "-") != 3 {
		t.Fatalf("激活码格式错误: %q", code)
	}

	prev := config.LicenseSecret
	config.LicenseSecret = secret
	defer func() { config.LicenseSecret = prev }()
	if !config.VerifyLicense("70A2F8D6", code) {
		t.Fatalf("生成的激活码未通过应用端校验: %q", code)
	}
	if config.VerifyLicense("BBBB0002", code) {
		t.Fatal("激活码不应对其他机器码有效")
	}
}

// TestGenerate_RestoresGlobalSecret 出码后必须还原包级密钥变量。
func TestGenerate_RestoresGlobalSecret(t *testing.T) {
	prev := config.LicenseSecret
	defer func() { config.LicenseSecret = prev }()
	config.LicenseSecret = "original"

	if _, err := generate("70A2F8D6", "temporary"); err != nil {
		t.Fatal(err)
	}
	if config.LicenseSecret != "original" {
		t.Fatalf("包级密钥未还原: %q", config.LicenseSecret)
	}
}
