package extractor

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dslipak/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// extractPdf 处理 PDF 提取（优先本地提取文本层）
func (e *Extractor) extractPdf(ctx context.Context, fileData []byte, fields []string, onProgress ProgressCallback) ([]Record, error) {
	e.logger.Info("正在解析 PDF 结构...", "bytes", len(fileData))

	// 1. 获取总页数 (增加多库回退逻辑以提高鲁棒性)
	totalPages := 1
	e.logger.Debug("尝试使用 dslipak/pdf 获取页数")
	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err == nil {
		totalPages = r.NumPage()
		e.logger.Info("dslipak/pdf 解析成功", "totalPages", totalPages)
	} else {
		e.logger.Warn("dslipak/pdf 解析失败，尝试回退到 pdfcpu", "error", err)
		// 回退到 pdfcpu
		pageCount, err := api.PageCount(bytes.NewReader(fileData), nil)
		if err == nil {
			totalPages = pageCount
			e.logger.Info("pdfcpu 解析成功", "totalPages", totalPages)
		} else {
			e.logger.Error("所有 PDF 库解析页数均失败", "error", err)
		}
	}

	// 2. 探测第一页文本层 (带超时保护，防止复杂 PDF 导致挂起)
	e.logger.Info("正在尝试提取第一页文本层以判断解析模式...")

	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
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
	if e.baiduClient.config.Token != "" {
		e.logger.Info("使用 [百度云端引擎] 进行解析")
		return e.baiduClient.ParseDocument(ctx, fileData, true, onProgress)
	}

	e.logger.Info("未配置百度 Token，回退至 [本地系统识别] 模式")
	return e.extractViaWinOcr(ctx, fileData, totalPages, onProgress)
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
	e.logger.Info("启动并行提取引擎", "workers", runtime.NumCPU())

	// 1. 预解析一次 Reader，供所有子任务复用 (dslipak/pdf 是并发安全的)
	r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return nil, fmt.Errorf("创建 PDF 阅读器失败: %w", err)
	}

	type pageResult struct {
		pageNum int
		records []Record
	}

	// 2. 准备并行任务
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // 限制最大并发，防止内存波动过大
	}
	if numWorkers > totalPages {
		numWorkers = totalPages
	}

	jobs := make(chan int, totalPages)
	results := make(chan pageResult, totalPages)

	// 3. 启动 Worker Pool
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNum := range jobs {
				// 提取并解析
				p := r.Page(pageNum)
				text, _ := p.GetPlainText(nil)

				if strings.TrimSpace(text) == "" {
					results <- pageResult{pageNum: pageNum}
					continue
				}

				pageRecords := e.parseCases(text, fields)
				for _, rec := range pageRecords {
					rec["page"] = fmt.Sprintf("%d", pageNum)
				}
				results <- pageResult{pageNum: pageNum, records: pageRecords}
			}
		}()
	}

	// 4. 发送任务
	go func() {
		for i := 1; i <= totalPages; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	// 5. 进度收集与结果汇总
	go func() {
		wg.Wait()
		close(results)
	}()

	var allPageResults []pageResult
	processedCount := 0
	for res := range results {
		if err := ctx.Err(); err != nil {
			// drain remaining results to avoid goroutine leak, then bail
			go func() {
				for range results {
				}
			}()
			return nil, ErrCancelled
		}
		processedCount++
		if onProgress != nil {
			onProgress(processedCount, totalPages, "正在进行文本层逻辑分析...")
		}
		if len(res.records) > 0 {
			allPageResults = append(allPageResults, res)
		}
	}

	// 6. 按照页码排序，保证输出顺序一致
	sort.Slice(allPageResults, func(i, j int) bool {
		return allPageResults[i].pageNum < allPageResults[j].pageNum
	})

	var finalRecords []Record
	for _, pr := range allPageResults {
		finalRecords = append(finalRecords, pr.records...)
	}

	return finalRecords, nil
}
