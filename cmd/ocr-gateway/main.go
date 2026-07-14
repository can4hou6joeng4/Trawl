// ocr-gateway 是 Legal Extractor 的 OCR 中继网关：
// 校验机器码/授权 → 换上真实 AI Studio Token → 转发 v2 作业式接口。
// 应用侧不携带任何云端密钥；结果 JSONL 为签名 URL，客户端直连下载不过网关。
//
// 全部配置经环境变量：
//
//	OCR_GW_TOKEN           必填，真实 AI Studio Token
//	OCR_GW_LICENSE_SECRET  与应用构建注入的 LicenseSecret 一致；缺省则全部按试用档
//	OCR_GW_UPSTREAM        上游作业接口（默认官方端点）
//	OCR_GW_LISTEN          监听地址（默认 :8787）
//	OCR_GW_TRIAL_DAILY     试用档每日提交上限（默认 20）
//	OCR_GW_LICENSED_DAILY  授权档每日提交上限（默认 200）
//
// 部署指引见 docs/ocr-gateway-deploy.md。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"legal-extractor/internal/gateway"
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	g, err := gateway.New(gateway.Config{
		UpstreamJobURL: os.Getenv("OCR_GW_UPSTREAM"),
		UpstreamToken:  os.Getenv("OCR_GW_TOKEN"),
		LicenseSecret:  os.Getenv("OCR_GW_LICENSE_SECRET"),
		TrialDaily:     envInt("OCR_GW_TRIAL_DAILY", 0),
		LicensedDaily:  envInt("OCR_GW_LICENSED_DAILY", 0),
	}, logger)
	if err != nil {
		logger.Error("网关启动失败", "error", err)
		os.Exit(1)
	}

	listen := os.Getenv("OCR_GW_LISTEN")
	if listen == "" {
		listen = ":8787"
	}
	srv := &http.Server{
		Addr:              listen,
		Handler:           g,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      120 * time.Second, // 覆盖上游 90s 超时 + 回写
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("OCR 中继网关已启动", "listen", listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("服务异常退出", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("收到退出信号，优雅关闭中...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
