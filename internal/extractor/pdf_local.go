package extractor

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/dslipak/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// extractPdf 处理 PDF 提取（优先本地提取文本层）
func (e *Extractor) extractPdf(ctx context.Context, fileData []byte, fields []string, onProgress ProgressCallback) ([]Record, error) {
	e.logger.Info("正在解析 PDF 结构...", "bytes", len(fileData))

	// 1. 获取总页数（多库回退提高鲁棒性；全部失败时按 1 页继续尝试）
	totalPages, err := e.pdfPageCount(fileData)
	if err != nil {
		totalPages = 1
	}

	// 2. 探测第一页文本层 (带超时保护，防止复杂 PDF 导致挂起)
	e.logger.Info("正在尝试提取第一页文本层以判断解析模式...")

	probeTimeout := 2 * time.Second
	if len(fileData) < 512*1024 && totalPages <= 3 {
		probeTimeout = 8 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	textChan := make(chan string, 1)
	go func() {
		t, _ := e.extractPageTextLocally(fileData, 1)
		textChan <- t
	}()

	var firstPageText string
	select {
	case firstPageText = <-textChan:
		e.logger.Debug("文本层探测完成")
	case <-probeCtx.Done():
		e.logger.Warn("文本层探测超时，自动切换至 OCR 模式")
	}

	if len(strings.TrimSpace(firstPageText)) > 20 {
		e.logger.Info("检测到 PDF 文本层，切换至 [本地高速解析] 模式")
		return e.batchExtractLocalPdf(ctx, fileData, fields, totalPages, onProgress)
	}

	e.logger.Info("未检测到 PDF 文本层或文本过少，切换至 [云端识别] 模式")

	// 3. 如果配置了百度 Token，则优先使用百度 PaddleOCR-VL (Layout Parsing)
	if e.baiduClient.HasToken() {
		e.logger.Info("使用 [百度云端引擎] 进行解析")
		return e.baiduClient.ParseDocument(ctx, fileData, totalPages, true, onProgress)
	}

	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("%w: 扫描件或无文本层 PDF 需要配置百度 OCR Token", ErrTokenMissing)
	}

	e.logger.Info("未配置百度 Token，回退至 [本地系统识别] 模式")
	return e.extractViaWinOcr(ctx, fileData, totalPages, onProgress)
}

// pdfPageCount 依次尝试 dslipak/pdf 与 pdfcpu 获取页数，供本地与云端路径共用。
func (e *Extractor) pdfPageCount(fileData []byte) (int, error) {
	e.logger.Debug("尝试使用 dslipak/pdf 获取页数")
	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err == nil {
		e.logger.Info("dslipak/pdf 解析成功", "totalPages", r.NumPage())
		return r.NumPage(), nil
	}

	e.logger.Warn("dslipak/pdf 解析失败，尝试回退到 pdfcpu", "error", err)
	pageCount, err2 := api.PageCount(bytes.NewReader(fileData), nil)
	if err2 == nil {
		e.logger.Info("pdfcpu 解析成功", "totalPages", pageCount)
		return pageCount, nil
	}

	e.logger.Error("所有 PDF 库解析页数均失败", "dslipak", err, "pdfcpu", err2)
	return 0, fmt.Errorf("解析 PDF 页数失败: %w", err2)
}

// extractPageTextLocally 本地提取指定页码的文本
func (e *Extractor) extractPageTextLocally(fileData []byte, pageNum int) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
	}

	if pageNum > r.NumPage() {
		return "", fmt.Errorf("页码 %d 超出范围 (总页数: %d)", pageNum, r.NumPage())
	}

	p := r.Page(pageNum)
	text, _ := p.GetPlainText(nil)
	return text, nil
}

// batchExtractLocalPdf 批量本地提取 PDF 文本层 (并发加速版)
func (e *Extractor) batchExtractLocalPdf(ctx context.Context, fileData []byte, fields []string, totalPages int, onProgress ProgressCallback) ([]Record, error) {
	numWorkers := min(runtime.NumCPU(), 8) // 限制最大并发，防止内存波动过大
	e.logger.Info("启动并行提取引擎", "workers", numWorkers)

	// 预解析一次 Reader，供所有子任务复用 (dslipak/pdf 是并发安全的)
	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return nil, fmt.Errorf("创建 PDF 阅读器失败: %w", err)
	}

	return e.collectPageRecords(ctx, totalPages, numWorkers,
		func(pageNum int) []Record {
			p := r.Page(pageNum)
			text, _ := p.GetPlainText(nil)
			if strings.TrimSpace(text) == "" {
				return nil
			}
			return e.parseCases(text, fields)
		},
		onProgress,
		func(int) string { return "正在进行文本层逻辑分析..." },
	)
}
