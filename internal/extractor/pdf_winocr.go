package extractor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// extractViaWinOcr 调用 Windows 系统原生 OCR 桥接工具 (并发加速版)
func (e *Extractor) extractViaWinOcr(ctx context.Context, fileData []byte, totalPages int, onProgress ProgressCallback) ([]Record, error) {
	// 1. 创建临时文件存储 PDF 内容
	tempFile, err := os.CreateTemp("", "legal_ocr_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()
	defer func() { _ = tempFile.Close() }()

	if _, err := tempFile.Write(fileData); err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 2. 定位桥接工具路径
	exePath, _ := os.Executable()
	baseDir := filepath.Dir(exePath)
	bridgePath := filepath.Join(baseDir, "bridge_bin", "WinOcrBridge.exe")

	if _, err := os.Stat(bridgePath); os.IsNotExist(err) {
		bridgePath = filepath.Join("internal", "extractor", "bridge_bin", "WinOcrBridge.exe")
		if _, err := os.Stat(bridgePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("找不到 Windows OCR 桥接工具 (WinOcrBridge.exe)")
		}
	}

	type pageResult struct {
		pageNum int
		records []Record
	}

	// 3. 并行执行 OCR 进程
	numWorkers := 4 // OCR 进程较重，限制并发数
	if numWorkers > totalPages {
		numWorkers = totalPages
	}

	jobs := make(chan int, totalPages)
	results := make(chan pageResult, totalPages)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNum := range jobs {
				cmd := exec.CommandContext(ctx, bridgePath, tempFile.Name(), fmt.Sprintf("%d", pageNum))
				output, err := cmd.CombinedOutput()
				if err != nil {
					results <- pageResult{pageNum: pageNum}
					continue
				}

				text := strings.TrimSpace(string(output))
				if text == "" {
					results <- pageResult{pageNum: pageNum}
					continue
				}

				pageRecords := e.parseCases(text, nil)
				for _, rec := range pageRecords {
					rec["page"] = fmt.Sprintf("%d", pageNum)
				}
				results <- pageResult{pageNum: pageNum, records: pageRecords}
			}
		}()
	}

	go func() {
		for i := 1; i <= totalPages; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var allPageResults []pageResult
	processed := 0
	for res := range results {
		if err := ctx.Err(); err != nil {
			go func() {
				for range results {
				}
			}()
			return nil, ErrCancelled
		}
		processed++
		if onProgress != nil {
			onProgress(processed, totalPages, fmt.Sprintf("正在调用系统识别引擎提取第 %d 页内容...", res.pageNum))
		}
		if len(res.records) > 0 {
			allPageResults = append(allPageResults, res)
		}
	}

	sort.Slice(allPageResults, func(i, j int) bool {
		return allPageResults[i].pageNum < allPageResults[j].pageNum
	})

	var finalRecords []Record
	for _, pr := range allPageResults {
		finalRecords = append(finalRecords, pr.records...)
	}

	return finalRecords, nil
}
