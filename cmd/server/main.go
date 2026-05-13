// Package main 提供 LegalExtractor 的 Web 服务入口
// 这是 V2.1 Web 兼容版的 HTTP 服务端，与桌面版共享核心业务逻辑
package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"legal-extractor/internal/config"
	"legal-extractor/internal/extractor"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// 全局提取器实例
var extractorInstance *extractor.Extractor

// IPRateLimiter 简单的 IP 限流器
type IPRateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int           // 限制次数
	window   time.Duration // 时间窗口
}

// NewIPRateLimiter 创建新的限流器
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	rl := &IPRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

// cleanup 定期清理过期记录，防止内存泄漏
func (r *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for ip, times := range r.requests {
			var valid []time.Time
			for _, t := range times {
				if now.Sub(t) < r.window {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(r.requests, ip)
			} else {
				r.requests[ip] = valid
			}
		}
		r.mu.Unlock()
	}
}

// Allow 检查 IP 是否允许请求
func (r *IPRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-r.window)

	// 清理过期记录
	var validRequests []time.Time
	for _, t := range r.requests[ip] {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	// 检查是否超过限制
	if len(validRequests) >= r.limit {
		r.requests[ip] = validRequests
		return false
	}

	// 添加新请求记录
	r.requests[ip] = append(validRequests, now)
	return true
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter *IPRateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !limiter.Allow(ip) {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "请求过于频繁，请稍后再试",
				})
			}
			return next(c)
		}
	}
}

// ExtractRequest 提取请求结构
type ExtractRequest struct {
	Fields []string `json:"fields"`
}

// ExtractResponse 提取响应结构
type ExtractResponse struct {
	Success     bool               `json:"success"`
	RecordCount int                `json:"recordCount"`
	Records     []extractor.Record `json:"records,omitempty"`
	FieldLabels map[string]string  `json:"fieldLabels,omitempty"`
	Error       string             `json:"error,omitempty"`
}

// ExportRequest 导出请求结构
type ExportRequest struct {
	Records []extractor.Record `json:"records"`
	Format  string             `json:"format"` // xlsx, csv, json
}

func main() {
	// 1. 初始化日志
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 2. 初始化配置
	if err := config.Init(""); err != nil {
		logger.Warn("配置加载失败", "error", err.Error())
	}

	// 3. 初始化提取器
	extractorInstance = extractor.NewExtractor(logger)

	// 3. 创建 Echo 实例
	e := echo.New()
	e.HideBanner = true

	// 4. 中间件
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.Info("request", "uri", v.URI, "status", v.Status, "error", v.Error)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS()) // 允许跨域请求

	// 限流：每 IP 每分钟最多 10 次请求
	limiter := NewIPRateLimiter(10, time.Minute)
	e.Use(RateLimitMiddleware(limiter))

	// 文件上传大小限制：50MB
	e.Use(middleware.BodyLimit("50M"))

	// 5. 路由
	e.GET("/", handleIndex)
	e.GET("/health", handleHealth)

	api := e.Group("/api")
	api.POST("/extract", handleExtract)
	api.POST("/export", handleExport)

	// 6. 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("LegalExtractor Web 服务启动", "port", port)
	e.Logger.Fatal(e.Start(":" + port))
}

// handleIndex 首页
func handleIndex(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"service": "LegalExtractor Web API",
		"version": "3.3.0",
		"status":  "running",
	})
}

// handleHealth 健康检查
func handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// handleExtract 处理文件提取请求
func handleExtract(c echo.Context) error {
	// 1. 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, ExtractResponse{
			Success: false,
			Error:   "请上传文件",
		})
	}

	// 2. 验证文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".pdf": true, ".docx": true}
	if !allowedExts[ext] {
		return c.JSON(http.StatusBadRequest, ExtractResponse{
			Success: false,
			Error:   fmt.Sprintf("不支持的文件格式: %s，支持 PDF 和 DOCX", ext),
		})
	}

	// 3. 读取文件内容到内存
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ExtractResponse{
			Success: false,
			Error:   "无法读取上传的文件",
		})
	}
	defer func() { _ = src.Close() }()

	fileData, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ExtractResponse{
			Success: false,
			Error:   "读取文件内容失败",
		})
	}

	// 4. 获取提取字段（可选）
	fields := c.QueryParams()["fields"]
	if len(fields) == 0 {
		fields = []string{"defendant", "idNumber", "request", "factsReason"}
	}

	// 5. 调用核心提取逻辑
	records, err := extractorInstance.ExtractData(c.Request().Context(), fileData, file.Filename, fields, nil)
	if err != nil {
		extractorInstance.Logger().Error("提取失败", "error", err)
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, extractor.ErrUnsupportedFormat), errors.Is(err, extractor.ErrEmptyFile):
			status = http.StatusBadRequest
		case errors.Is(err, extractor.ErrCancelled):
			status = 499 // client closed request
		}
		return c.JSON(status, ExtractResponse{
			Success: false,
			Error:   fmt.Sprintf("提取失败: %v", err),
		})
	}

	extractorInstance.Logger().Info("提取完成", "recordCount", len(records))

	// 6. 获取字段标签
	labels := make(map[string]string)
	for k, p := range extractor.PatternRegistry {
		labels[k] = p.Label
	}

	return c.JSON(http.StatusOK, ExtractResponse{
		Success:     true,
		RecordCount: len(records),
		Records:     records,
		FieldLabels: labels,
	})
}

// handleExport 处理数据导出请求
func handleExport(c echo.Context) error {
	var req ExportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "无效的请求数据",
		})
	}

	if len(req.Records) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "没有可导出的数据",
		})
	}

	// 默认格式为 xlsx
	format := strings.ToLower(req.Format)
	if format == "" {
		format = "xlsx"
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "export-*."+format)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "创建临时文件失败",
		})
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "关闭临时文件失败",
		})
	}
	defer func() { _ = os.Remove(tmpPath) }()

	// 导出到临时文件
	switch format {
	case "xlsx":
		err = extractor.ExportExcel(tmpPath, req.Records)
	case "csv":
		err = extractor.ExportCSV(tmpPath, req.Records)
	case "json":
		err = extractor.ExportJSON(tmpPath, req.Records)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("不支持的导出格式: %s", format),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("导出失败: %v", err),
		})
	}

	// 设置下载文件名
	filename := fmt.Sprintf("extracted_data.%s", format)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return c.File(tmpPath)
}
