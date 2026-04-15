package config

import (
	"fmt"
	"testing"
	"time"
)

func TestGetTrialStatus_DevMode(t *testing.T) {
	// 未注入 BuildTime 时为开发模式，不限制试用
	origBT := BuildTime
	BuildTime = ""
	defer func() { BuildTime = origBT }()

	status := GetTrialStatus()
	if status.IsExpired {
		t.Error("开发模式不应过期")
	}
}

func TestGetTrialStatus_Expired(t *testing.T) {
	origBT := BuildTime
	// 设置一个 30 天前的时间戳
	BuildTime = fmt.Sprintf("%d", time.Now().Add(-30*24*time.Hour).Unix())
	defer func() { BuildTime = origBT }()

	status := GetTrialStatus()
	if !status.IsExpired {
		t.Error("30 天前的构建应该已过期")
	}
}

func TestGetTrialStatus_Active(t *testing.T) {
	origBT := BuildTime
	// 设置为当前时间戳
	BuildTime = fmt.Sprintf("%d", time.Now().Unix())
	defer func() { BuildTime = origBT }()

	status := GetTrialStatus()
	if status.IsExpired {
		t.Error("刚构建的版本不应过期")
	}
	if status.Days < 6 {
		t.Errorf("剩余天数应接近 7，实际 %d", status.Days)
	}
}

func TestGetTrialStatus_InvalidBuildTime(t *testing.T) {
	origBT := BuildTime
	BuildTime = "not-a-number"
	defer func() { BuildTime = origBT }()

	status := GetTrialStatus()
	if status.IsExpired {
		t.Error("无效时间戳不应报过期")
	}
}

func TestGetMachineID(t *testing.T) {
	id := GetMachineID()
	if len(id) != 8 {
		t.Errorf("机器码长度应为 8，实际 %d", len(id))
	}

	// 多次调用应返回相同值
	id2 := GetMachineID()
	if id != id2 {
		t.Error("同一设备机器码应稳定不变")
	}
}

func TestGenerateLicense_EmptySecret(t *testing.T) {
	origSecret := LicenseSecret
	LicenseSecret = ""
	defer func() { LicenseSecret = origSecret }()

	result := GenerateLicense("TEST1234")
	if result != "" {
		t.Errorf("空密钥应返回空字符串，实际 %q", result)
	}
}

func TestVerifyLicense_EmptySecret(t *testing.T) {
	origSecret := LicenseSecret
	LicenseSecret = ""
	defer func() { LicenseSecret = origSecret }()

	if VerifyLicense("TEST1234", "anything") {
		t.Error("空密钥应拒绝所有授权码")
	}
}

func TestGenerateAndVerifyLicense(t *testing.T) {
	origSecret := LicenseSecret
	LicenseSecret = "test-secret-key"
	defer func() { LicenseSecret = origSecret }()

	machineID := "ABCD1234"
	license := GenerateLicense(machineID)

	// 格式检查: XXXX-XXXX-XXXX-XXXX
	if len(license) != 19 {
		t.Fatalf("授权码长度应为 19，实际 %d: %q", len(license), license)
	}

	// 验证通过
	if !VerifyLicense(machineID, license) {
		t.Error("正确授权码验证应通过")
	}

	// 大小写不敏感
	if !VerifyLicense(machineID, license) {
		t.Error("授权码验证应大小写不敏感")
	}

	// 错误授权码
	if VerifyLicense(machineID, "XXXX-XXXX-XXXX-XXXX") {
		t.Error("错误授权码不应通过验证")
	}

	// 不同机器码
	if VerifyLicense("DIFF5678", license) {
		t.Error("不同机器码不应共享授权码")
	}
}
