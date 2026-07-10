package extractor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// extractViaWinOcr 调用 Windows 系统原生 OCR 桥接工具 (并发加速版)
func (e *Extractor) extractViaWinOcr(ctx context.Context, fileData []byte, totalPages int, onProgress ProgressCallback) ([]Record, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("%w: 本地系统 OCR 兜底仅支持 Windows，请配置百度 OCR Token", ErrTokenMissing)
	}

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

	// 并行执行 OCR 进程（OCR 进程较重，限制并发数）
	const numWorkers = 4
	return e.collectPageRecords(ctx, totalPages, numWorkers,
		func(pageNum int) []Record {
			cmd := exec.CommandContext(ctx, bridgePath, tempFile.Name(), fmt.Sprintf("%d", pageNum))
			output, err := cmd.CombinedOutput()
			if err != nil {
				return nil
			}
			text := strings.TrimSpace(string(output))
			if text == "" {
				return nil
			}
			return e.parseCases(text, nil)
		},
		onProgress,
		func(pageNum int) string {
			return fmt.Sprintf("正在调用系统识别引擎提取第 %d 页内容...", pageNum)
		},
	)
}
