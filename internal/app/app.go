package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"legal-extractor/internal/config"
	"legal-extractor/internal/extractor"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	extractor *extractor.Extractor

	// cancelMu guards cancelFn. cancelFn is non-nil exactly when an
	// extraction is in flight; calling it aborts that extraction.
	cancelMu sync.Mutex
	cancelFn context.CancelFunc
}

// NewApp creates a new App application struct
func NewApp(e *extractor.Extractor) *App {
	return &App{
		extractor: e,
	}
}

// Startup is called when the app starts
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// GetTrialStatus 返回试用期状态
func (a *App) GetTrialStatus() config.TrialStatus {
	return config.GetTrialStatus()
}

// GetMachineID 返回当前设备的唯一机器码
func (a *App) GetMachineID() string {
	return config.GetMachineID()
}

// Activate 验证并激活授权码
func (a *App) Activate(licenseKey string) (bool, error) {
	machineID := config.GetMachineID()
	if config.VerifyLicense(machineID, licenseKey) {
		err := config.SaveLicense(licenseKey)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, fmt.Errorf("授权码无效，请检查后重试")
}

// SelectFile opens a file dialog to select a supported legal document.
func (a *App) SelectFile() (string, error) {
	file, err := wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{
		Title: "Select Legal Document",
		Filters: []wr.FileFilter{
			{
				DisplayName: "Legal Documents (*.docx;*.pdf;*.jpg;*.jpeg;*.png)",
				Pattern:     "*.docx;*.pdf;*.jpg;*.jpeg;*.png",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return file, nil
}

// ExtractResult holds the extraction result
type ExtractResult struct {
	Success      bool               `json:"success"`
	RecordCount  int                `json:"recordCount"`
	OutputPath   string             `json:"outputPath"`
	ErrorMessage string             `json:"errorMessage,omitempty"`
	Records      []extractor.Record `json:"records,omitempty"`
	FieldLabels  map[string]string  `json:"fieldLabels,omitempty"` // Map of key -> Chinese label
}

// FieldOption represents a selectable extraction field
type FieldOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ScanFields 返回系统支持的可提取字段列表 (优化：本地静态返回，避免 API 调用费)
func (a *App) ScanFields(inputFile string) ([]FieldOption, error) {
	var options []FieldOption
	// 定义提取器支持的核心字段
	orderedKeys := []string{"defendant", "idNumber", "request", "factsReason"}

	for _, k := range orderedKeys {
		if p, ok := extractor.PatternRegistry[k]; ok {
			options = append(options, FieldOption{
				Key:   k,
				Label: p.Label,
			})
		}
	}

	return options, nil
}

// SelectOutputPath opens a save dialog for the user to choose destination
func (a *App) SelectOutputPath(defaultName string) (string, error) {
	if defaultName == "" {
		defaultName = "extracted_data.csv"
	}

	// Ensure default name has correct extension base logic if needed,
	// but mostly we trust the caller or just use a generic name.
	// Actually, best to let frontend pass the input filename so we can suggest input_extracted.csv

	outputFile, err := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{
		Title:           "Select Output Location",
		DefaultFilename: defaultName,
		Filters: []wr.FileFilter{
			{
				DisplayName: "Excel Files (*.xlsx)",
				Pattern:     "*.xlsx",
			},
			{
				DisplayName: "CSV Files (*.csv)",
				Pattern:     "*.csv",
			},
			{
				DisplayName: "JSON Files (*.json)",
				Pattern:     "*.json",
			},
		},
	})

	if err != nil {
		return "", err
	}
	return outputFile, nil
}

// errTrialExpired 试用期结束的稳定文案，PreviewData 与 ExtractToPath 共用。
var errTrialExpired = errors.New("试用期已结束（限 7 天），功能已锁定。请联系开发者获取正式版。")

// runExtraction 是 PreviewData 与 ExtractToPath 的公共骨架：
// 试用校验 → 读取本地文件 → 在可取消的 ctx 中执行提取。
func (a *App) runExtraction(inputPath string, fields []string) ([]extractor.Record, error) {
	if config.GetTrialStatus().IsExpired {
		return nil, errTrialExpired
	}

	// 适配器层：负责读取本地文件
	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	if len(fileData) == 0 {
		return nil, fmt.Errorf("%w，请检查文件是否损坏", extractor.ErrEmptyFile)
	}

	ctx := a.startExtraction()
	defer a.finishExtraction()
	return a.extractor.ExtractData(ctx, fileData, inputPath, fields, a.emitExtractionProgress)
}

// ExtractToPath processes the input file and saves to the specific output path
func (a *App) ExtractToPath(inputPath, outputPath string, fields []string) ExtractResult {
	if inputPath == "" || outputPath == "" {
		return ExtractResult{
			Success:      false,
			ErrorMessage: "输入或输出路径无效",
		}
	}

	records, err := a.runExtraction(inputPath, fields)
	if err != nil {
		return ExtractResult{
			Success:      false,
			ErrorMessage: friendlyExtractError(err),
		}
	}

	if len(records) == 0 {
		return ExtractResult{
			Success:      false,
			ErrorMessage: extractor.ErrNoRecords.Error(),
		}
	}

	// Save based on extension
	return a.ExportData(records, outputPath)
}

// ExportData 接收用户编辑后的数据并直接保存到指定路径
func (a *App) ExportData(records []extractor.Record, outputPath string) ExtractResult {
	if len(records) == 0 || outputPath == "" {
		return ExtractResult{
			Success:      false,
			ErrorMessage: "无有效数据或未指定输出路径",
		}
	}

	var err error
	lowerPath := strings.ToLower(outputPath)
	if strings.HasSuffix(lowerPath, ".json") {
		err = extractor.ExportJSON(outputPath, records)
	} else if strings.HasSuffix(lowerPath, ".xlsx") {
		err = extractor.ExportExcel(outputPath, records)
	} else {
		err = extractor.ExportCSV(outputPath, records)
	}

	if err != nil {
		return ExtractResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("导出失败: %v", err),
		}
	}

	return ExtractResult{
		Success:     true,
		RecordCount: len(records),
		OutputPath:  outputPath,
	}
}

// PreviewData extracts and returns records for preview (without saving)
func (a *App) PreviewData(inputPath string, fields []string) ExtractResult {
	a.extractor.Logger().Info("收到预览请求", "path", inputPath)
	if inputPath == "" {
		return ExtractResult{
			Success:      false,
			ErrorMessage: "未选择文件",
		}
	}

	records, err := a.runExtraction(inputPath, fields)
	if err != nil {
		return ExtractResult{
			Success:      false,
			ErrorMessage: friendlyExtractError(err),
		}
	}

	if len(records) == 0 {
		return ExtractResult{
			Success:      false,
			ErrorMessage: extractor.ErrNoRecords.Error(),
		}
	}

	// Get labels for UI
	labels := make(map[string]string)
	for k, p := range extractor.PatternRegistry {
		labels[k] = p.Label
	}

	return ExtractResult{
		Success:     true,
		RecordCount: len(records),
		Records:     records,
		FieldLabels: labels,
	}
}

// OpenFile opens the file at the given path using the system's default application
func (a *App) OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func (a *App) emitExtractionProgress(current, total int, message string) {
	if a.ctx == nil || a.ctx.Value("frontend") == nil {
		if a.extractor != nil {
			a.extractor.Logger().Debug("跳过进度事件：Wails 上下文尚未初始化", "current", current, "total", total)
		}
		return
	}
	defer func() {
		if r := recover(); r != nil {
			a.extractor.Logger().Warn("发送进度事件失败，已忽略", "recover", r)
		}
	}()
	wr.EventsEmit(a.ctx, "extraction_progress", map[string]interface{}{
		"current": current,
		"total":   total,
		"message": message,
	})
}

// friendlyExtractError converts known sentinel errors into a stable code string
// the frontend can branch on, while preserving the original message for any
// unknown failure mode.
func friendlyExtractError(err error) string {
	switch {
	case errors.Is(err, extractor.ErrPDFEncrypted):
		return "PDF_ENCRYPTED_OR_LOCKED"
	case errors.Is(err, extractor.ErrUnsupportedFormat):
		return "UNSUPPORTED_FORMAT: " + err.Error()
	case errors.Is(err, extractor.ErrTokenMissing):
		return "BAIDU_TOKEN_MISSING"
	case errors.Is(err, extractor.ErrCancelled):
		return "CANCELLED"
	default:
		return err.Error()
	}
}

// startExtraction returns a new cancellable child of a.ctx and registers its
// cancel func as the active one. Any previously in-flight extraction is
// pre-emptively cancelled — the frontend should not allow concurrent
// extractions, but defending against races here is cheap.
func (a *App) startExtraction() context.Context {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFn != nil {
		a.cancelFn()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelFn = cancel
	return ctx
}

// finishExtraction releases the active cancel func without cancelling. Safe
// to call multiple times. Always paired via defer with startExtraction.
func (a *App) finishExtraction() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

// CancelExtraction aborts the in-flight extraction (if any). Bound to the
// frontend via Wails. No-op when no extraction is running.
func (a *App) CancelExtraction() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}
