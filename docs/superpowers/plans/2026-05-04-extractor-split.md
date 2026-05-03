# Extractor Strategy Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `internal/extractor/extractor.go` (578 lines) into five focused files, one per strategy, with zero behavior change.

**Architecture:** All new files stay in `package extractor` so methods on `*Extractor` keep their receivers and package-private symbols stay accessible without exports. Each split is a pure cut-paste followed by `go test ./internal/extractor/ -race -count=1` — the existing test suite (4 test files, 30+ tests covering parsing, caching, cancellation, errors, exports, markdown) is the regression net.

**Tech Stack:** Go 1.25, no new dependencies.

---

## File Structure

| File | Lines (approx) | Responsibility |
|---|---|---|
| `extractor.go` (slimmed) | ~110 | `Extractor` struct, `NewExtractor`, `Logger`, `Record`, `ProgressCallback`, `ExtractData` dispatcher, `calculateHash` |
| `local_pdf.go` (new) | ~175 | `extractPdf` (probe + dispatch), `extractPageTextLocally`, `batchExtractLocalPdf` worker pool |
| `winocr.go` (new) | ~110 | `extractViaWinOcr` worker pool calling `WinOcrBridge.exe` |
| `docx.go` (new) | ~70 | `extractFromDocx` + `extractTextFromDocx` zip/XML parser |
| `parsing.go` (new) | ~115 | `parseCases`, `smartMerge`, and the three `re*` package-level regex vars |

No tests are added or modified. No exports change. Function signatures stay byte-identical. Only the residence of each function changes.

---

## Task 1: Branch + Baseline Snapshot

**Files:** none modified — preparation only.

- [ ] **Step 1: Create branch from latest main**

```bash
git checkout main && git pull --ff-only
git checkout -b feature/extractor-split
```

- [ ] **Step 2: Capture baseline test output**

Run: `go test ./internal/extractor/ -race -count=1 -v 2>&1 | tee /tmp/extractor-baseline.txt | tail -3`
Expected: ends with `PASS` and `ok  legal-extractor/internal/extractor`.

This file is the regression target — every later task must match this `PASS / ok` line.

---

## Task 2: Extract `parsing.go`

**Files:**
- Create: `internal/extractor/parsing.go`
- Modify: `internal/extractor/extractor.go` (delete the moved chunk)

`parseCases` and `smartMerge` are pure text-processing helpers — moving them first is safe (no IO, no goroutines) and lets us validate the cut-and-paste workflow on the simplest case.

- [ ] **Step 1: Create `parsing.go` with the moved code**

Create `internal/extractor/parsing.go` with this exact content:

```go
package extractor

import (
	"regexp"
	"strings"
)

// parseCases 现有的本地正则解析逻辑 (用于 DOCX)
func (e *Extractor) parseCases(text string, fields []string) []Record {
	parts := DefaultPatterns.Split.Split(text, -1)
	var data []Record

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		record := make(Record)
		fieldSet := make(map[string]bool)
		for _, f := range fields {
			fieldSet[f] = true
		}

		// 1. 提取被告
		if fieldSet["defendant"] {
			loc := DefaultPatterns.DefStart.FindStringIndex(part)
			if loc != nil {
				startIdx := loc[1]
				remaining := part[startIdx:]
				cleanRemaining := strings.ReplaceAll(remaining, "\n", "")
				locEnd := DefaultPatterns.DefEnd.FindStringIndex(cleanRemaining)

				var name string
				if locEnd != nil {
					name = cleanRemaining[:locEnd[0]]
				} else {
					if len(cleanRemaining) > 50 {
						name = cleanRemaining[:50]
					} else {
						name = cleanRemaining
					}
				}
				record["defendant"] = strings.TrimSpace(name)
			}
		}

		// 2. 提取身份证
		if fieldSet["idNumber"] {
			matchID := DefaultPatterns.ID.FindStringSubmatch(part)
			if len(matchID) > 1 {
				record["idNumber"] = strings.TrimSpace(matchID[1])
			}
		}

		// 3. 提取请求
		if fieldSet["request"] {
			matchReq := DefaultPatterns.Request.FindStringSubmatch(part)
			if len(matchReq) > 1 {
				record["request"] = smartMerge(matchReq[1])
			}
		}

		// 4. 提取事实
		if fieldSet["factsReason"] {
			matchFact := DefaultPatterns.Facts.FindStringSubmatch(part)
			if len(matchFact) > 1 {
				record["factsReason"] = smartMerge(matchFact[1])
			}
		}

		if len(record) > 0 {
			data = append(data, record)
		}
	}
	return data
}

// smartMerge 预编译正则
var (
	reMultipleNL     = regexp.MustCompile(`\n+`)
	rePreserveAfter  = regexp.MustCompile(`([。；？！])\n`)
	rePreserveBefore = regexp.MustCompile(`\n(\s*(?:[一二三四五六七八九十\d]+[、．]|[(（][一二三四五六七八九十\d]+[)）]))`)
)

// smartMerge 智能合并换行符
// 逻辑：保留句号、分号、冒号后的换行，或者新条目序号（如"二、"）之前的换行，其他的换行符视作布局造成的干扰并予以合并。
func smartMerge(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// 1. 标准化换行符
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = reMultipleNL.ReplaceAllString(s, "\n")

	// 2. 标记需要保留的"逻辑断点"
	s = rePreserveAfter.ReplaceAllString(s, "$1[LOGICAL_NL]")
	s = rePreserveBefore.ReplaceAllString(s, "[LOGICAL_NL]$1")

	// 3. 合并 OCR 碎行：将剩余的非逻辑换行符替换为一个小空格，防止文字粘连
	s = strings.ReplaceAll(s, "\n", " ")

	// 4. 将占位符还原为真正的换行符
	s = strings.ReplaceAll(s, "[LOGICAL_NL]", "\n")

	// 5. 深度清理：合并每行内部的多余空格
	lines := strings.Split(s, "\n")
	var resultLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		resultLines = append(resultLines, strings.Join(fields, " "))
	}

	return strings.Join(resultLines, "\n")
}
```

- [ ] **Step 2: Delete the same code from `extractor.go`**

In `internal/extractor/extractor.go`, find the `parseCases` function (currently around line 467) and delete from the line `// parseCases 现有的本地正则解析逻辑 (用于 DOCX)` all the way to the closing brace of `smartMerge` (currently around line 578 — i.e., the end of file).

After deletion, the file's last function should be `extractTextFromDocx`, and the file should end after its closing `}`.

Also remove `regexp` from `extractor.go`'s imports if it is no longer used elsewhere in the file (it isn't — `regexp` was only used by parseCases/smartMerge/the moved regex vars).

- [ ] **Step 3: Run tests + vet**

Run: `go test ./internal/extractor/ -race -count=1 2>&1 | tail -3`
Expected: `PASS` + `ok  legal-extractor/internal/extractor`.

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/extractor/parsing.go internal/extractor/extractor.go
git commit -m "refactor: 抽出 parseCases 与 smartMerge 至独立文件"
```

---

## Task 3: Extract `docx.go`

**Files:**
- Create: `internal/extractor/docx.go`
- Modify: `internal/extractor/extractor.go`

`extractFromDocx` + `extractTextFromDocx` are self-contained (no shared state with PDF/OCR paths) — second easiest move.

- [ ] **Step 1: Create `docx.go`**

Create `internal/extractor/docx.go`:

```go
package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractFromDocx 保留原有的本地 DOCX 提取逻辑
func (e *Extractor) extractFromDocx(fileData []byte, fields []string) ([]Record, error) {
	text, err := extractTextFromDocx(fileData)
	if err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		for k := range PatternRegistry {
			fields = append(fields, k)
		}
	}

	return e.parseCases(text, fields), nil
}

// extractTextFromDocx 核心 DOCX 文本提取逻辑
func extractTextFromDocx(fileData []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
	}

	var documentXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			documentXML, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if documentXML == nil {
		return "", fmt.Errorf("word/document.xml not found")
	}
	defer func() { _ = documentXML.Close() }()

	decoder := xml.NewDecoder(documentXML)
	var sb strings.Builder

	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "t" {
				var s string
				if err := decoder.DecodeElement(&s, &se); err == nil {
					sb.WriteString(s)
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "p", "tr":
				sb.WriteString("\n")
			case "tc":
				sb.WriteString(" ")
			}
		}
	}

	return sb.String(), nil
}
```

- [ ] **Step 2: Delete from `extractor.go`**

In `internal/extractor/extractor.go`, find and delete:
- The `extractFromDocx` function (currently around line 399, starts with `// extractFromDocx 保留原有的本地 DOCX 提取逻辑`)
- The `extractTextFromDocx` function immediately after it (currently ending around line 465 with its closing `}`)

After this deletion, `extractor.go` should contain dispatcher + extractPdf + batchExtractLocalPdf + extractViaWinOcr only.

Also remove these imports from `extractor.go` if no longer used: `archive/zip`, `encoding/xml`, `io`. Verify by searching the file post-delete.

- [ ] **Step 3: Test + vet**

Run: `go test ./internal/extractor/ -race -count=1 2>&1 | tail -3`
Expected: `PASS` + `ok ...`.

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/extractor/docx.go internal/extractor/extractor.go
git commit -m "refactor: 抽出 DOCX 提取逻辑至独立文件"
```

---

## Task 4: Extract `winocr.go`

**Files:**
- Create: `internal/extractor/winocr.go`
- Modify: `internal/extractor/extractor.go`

`extractViaWinOcr` is the standalone Windows fallback path. Moving it before `local_pdf.go` because it is more self-contained (single big function, no recursion into local-pdf helpers).

- [ ] **Step 1: Create `winocr.go`**

Create `internal/extractor/winocr.go`:

```go
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
```

- [ ] **Step 2: Delete from `extractor.go`**

Delete the entire `extractViaWinOcr` function (around line 287 to line 396) including its leading comment `// extractViaWinOcr 调用 Windows 系统原生 OCR 桥接工具 (并发加速版)`.

After deletion, `extractor.go` should contain dispatcher + extractPdf + extractPageTextLocally + batchExtractLocalPdf only.

Verify imports in `extractor.go`. If `os/exec` is no longer used (only winocr used `exec.CommandContext`), remove it. Same for `path/filepath` if not used by anything else.

- [ ] **Step 3: Test + vet**

Run: `go test ./internal/extractor/ -race -count=1 2>&1 | tail -3`
Expected: `PASS` + `ok ...`.

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/extractor/winocr.go internal/extractor/extractor.go
git commit -m "refactor: 抽出 Windows OCR 兜底逻辑至独立文件"
```

---

## Task 5: Extract `local_pdf.go`

**Files:**
- Create: `internal/extractor/local_pdf.go`
- Modify: `internal/extractor/extractor.go`

The biggest single move. After this task, `extractor.go` should be the slim dispatcher only.

- [ ] **Step 1: Create `local_pdf.go`**

Create `internal/extractor/local_pdf.go`:

```go
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
```

- [ ] **Step 2: Delete from `extractor.go`**

Delete:
- `extractPdf` (around line 112)
- `extractPageTextLocally` (around line 173)
- `batchExtractLocalPdf` (around line 189)

After this, `extractor.go` should only contain: imports, the `Extractor` struct, `NewExtractor`, `Logger`, `Record`, `ProgressCallback`, `ExtractData`, `calculateHash`. Total around 105-115 lines.

Clean up imports in `extractor.go`. After this task it should only need: `context`, `crypto/sha256`, `fmt`, `log/slog`, `path/filepath`, `strings`. Remove: `bytes`, `runtime`, `sort`, `sync`, `time`, `github.com/dslipak/pdf`, `github.com/pdfcpu/pdfcpu/pkg/api`. (Verify by trying `go build ./...` and following the unused-import errors.)

- [ ] **Step 3: Test + vet + build**

Run: `go test ./internal/extractor/ -race -count=1 2>&1 | tail -3`
Expected: `PASS` + `ok ...`.

Run: `go vet ./... && go build ./...`
Expected: clean for both.

- [ ] **Step 4: Confirm `extractor.go` is now slim**

Run: `wc -l internal/extractor/*.go`
Expected:
```
   ~110 internal/extractor/extractor.go
   ~175 internal/extractor/local_pdf.go
   ~110 internal/extractor/winocr.go
    ~70 internal/extractor/docx.go
   ~115 internal/extractor/parsing.go
```
(Numbers may differ ±5 due to import groupings; what matters is `extractor.go` < 130 lines and no single new file > 200.)

- [ ] **Step 5: Commit**

```bash
git add internal/extractor/local_pdf.go internal/extractor/extractor.go
git commit -m "refactor: 抽出本地 PDF 解析逻辑至独立文件"
```

---

## Task 6: Final Verification + PR

**Files:** none modified — verification only.

- [ ] **Step 1: Diff against main**

Run: `git diff main..HEAD --stat`
Expected: 5 files modified/created in `internal/extractor/`. Net diff should be near-zero (lines moved, not added/removed). Tests file unchanged.

- [ ] **Step 2: Full test run + build**

Run:
```bash
go test ./... -race -count=1
go vet ./...
go build ./...
go build -o /tmp/legal-server ./cmd/server
```
All four must succeed.

- [ ] **Step 3: Confirm test count is unchanged from baseline**

Run: `go test ./internal/extractor/ -count=1 -v 2>&1 | grep -c '^--- PASS:'`
Compare against: `grep -c '^--- PASS:' /tmp/extractor-baseline.txt`
Expected: identical numbers — no tests gained, no tests lost.

- [ ] **Step 4: Push**

```bash
git push -u origin feature/extractor-split
```

- [ ] **Step 5: Open PR**

```bash
gh pr create --base main --head feature/extractor-split \
  --title "refactor: 提取器按策略拆分为独立文件" \
  --body "$(cat <<'EOF'
## 概述

把 \`internal/extractor/extractor.go\`（578 行）按职责拆为 5 个聚焦文件，零行为变更。

## 拆分映射

| 文件 | 职责 |
|---|---|
| extractor.go | 调度器：Extractor、NewExtractor、ExtractData、calculateHash |
| local_pdf.go | extractPdf 探测 + batchExtractLocalPdf 并行提取 + extractPageTextLocally |
| winocr.go | extractViaWinOcr：Windows OCR 桥接 |
| docx.go | extractFromDocx + extractTextFromDocx |
| parsing.go | parseCases + smartMerge + 相关 regex |

## 验证

- 所有现有测试不变，覆盖率不变
- go test ./... -race -count=1 全绿
- go vet ./... 干净
- 桌面 / Web 二进制构建成功
- 公共 API 零变化（包仍为 extractor，导出符号不变）
EOF
)"
```

- [ ] **Step 6: Wait for CI + merge**

```bash
sleep 60
gh pr view <NUM> --json statusCheckRollup --jq '.statusCheckRollup[] | {name, conclusion}'
```

If all green, squash-merge with compliant subject:

```bash
gh pr merge <NUM> --squash --delete-branch \
  --subject "refactor: 提取器按策略拆分为独立文件" \
  --body "把 internal/extractor/extractor.go 拆为 5 个聚焦文件：extractor.go (调度器)、local_pdf.go、winocr.go、docx.go、parsing.go。零行为变更，所有测试与构建通过。"
```

---

## Done Criteria

- [ ] All tests in `internal/extractor/` still pass under `-race` after each individual task and again at the end.
- [ ] `git diff main..HEAD --shortstat` shows roughly equal +/- (a clean move, not net additions).
- [ ] `extractor.go` ≤ 130 lines.
- [ ] No new file exceeds 200 lines.
- [ ] CI on the feature branch passes all four checks (Lint / Test / Build Check / Security).
- [ ] PR merged to main with a compliant squash-commit subject.
