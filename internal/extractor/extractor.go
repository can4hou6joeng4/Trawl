package extractor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

// defaultCacheCapacity bounds in-memory record cache. ~50 PDFs at typical
// extraction sizes (a few KB of records each) is well under 5 MB resident.
const defaultCacheCapacity = 50

// Extractor 处理器，负责协调不同格式的提取策略
//
// 不同策略的实现拆分到独立文件：
//   - PDF 本地文本层 / 云端 OCR 调度： pdf_local.go
//   - Windows 系统 OCR 兜底：          pdf_winocr.go
//   - DOCX 解析：                      docx.go
//   - 正则解析与文本归并：              parsing.go
type Extractor struct {
	logger      *slog.Logger
	baiduClient *BaiduClient
	cache       *RecordCache
}

// NewExtractor 创建一个新的提取器实例
func NewExtractor(logger *slog.Logger) *Extractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Extractor{
		logger:      logger,
		baiduClient: NewBaiduClient(logger),
		cache:       NewRecordCache(defaultCacheCapacity),
	}
}

// Logger 返回提取器的日志记录器
func (e *Extractor) Logger() *slog.Logger {
	return e.logger
}

// Record 代表一条提取的记录
type Record map[string]string

// ProgressCallback 进度回调函数
type ProgressCallback func(current, total int, message string)

// ExtractData 根据文件类型选择提取策略.
//
// ctx is honored at every IO boundary: HTTP requests to Baidu, sleeps between
// retry chunks, and worker-pool result drains. A pre-cancelled ctx returns
// ErrCancelled before any work is done.
func (e *Extractor) ExtractData(ctx context.Context, fileData []byte, fileName string, fields []string, onProgress ProgressCallback) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, ErrCancelled
	}
	e.logger.Info("开始提取数据", "file", fileName, "size", len(fileData), "fields", fields)
	ext := strings.ToLower(filepath.Ext(fileName))

	// 1. 检查缓存 (使用文件内容的 SHA256 哈希作为 Key)
	fileHash := e.calculateHash(fileData)
	if cached, ok := e.cache.Get(fileHash); ok {
		e.logger.Info("命中内容哈希缓存，跳过提取", "file", fileName, "hash", fileHash[:8])
		return cached, nil
	}

	var records []Record
	var err error

	switch ext {
	case ".pdf":
		records, err = e.extractPdf(ctx, fileData, fields, onProgress)
	case ".jpg", ".png", ".jpeg":
		return nil, fmt.Errorf("图片识别功能已暂时禁用（仅支持PDF）: %w", ErrUnsupportedFormat)
	case ".docx":
		e.logger.Info("使用本地原生逻辑提取 DOCX", "file", fileName)
		records, err = e.extractFromDocx(ctx, fileData, fields)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, ext)
	}

	if err != nil {
		return nil, err
	}

	// 2. 写入缓存 (仅当结果非空时)
	if len(records) > 0 {
		e.cache.Put(fileHash, records)
	}

	return records, nil
}

// calculateHash 计算文件内容的 SHA256 哈希值
func (e *Extractor) calculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
