# Reliability Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate four reliability defects in the extractor: stale config drift, ad-hoc string-based error matching, unbounded in-memory cache, and uncancellable long-running OCR calls.

**Architecture:** Add two small new files inside `internal/extractor/` (`errors.go`, `cache.go`) and propagate `context.Context` from the public `ExtractData` API down through the Baidu HTTP client, the local PDF worker pool, and the Windows OCR worker pool. The Wails app layer and Echo HTTP server gain a clear cancellation path; existing tests keep passing because the `context.Context` parameter has a permissive default behavior.

**Tech Stack:** Go 1.25, Wails v2, Echo v4, `dslipak/pdf`, `pdfcpu`, `net/http`. No new dependencies — only the standard library `container/list` for LRU.

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `wails.json` | Modify | Bump `productVersion` and top-level `version` to `3.0.0`. |
| `docker-compose.yml` | Modify | Replace `BAIDU_API_KEY/SECRET_KEY` env vars with `LEGAL_EXTRACTOR_BAIDU_TOKEN`, matching what `config.Init` actually binds. |
| `internal/extractor/errors.go` | Create | Sentinel error values + `IsXxx` helpers used across packages. |
| `internal/extractor/errors_test.go` | Create | Unit tests for error wrapping/`errors.Is`. |
| `internal/extractor/cache.go` | Create | Generic-free thread-safe LRU keyed by SHA-256 hex string with capacity. |
| `internal/extractor/cache_test.go` | Create | Concurrency + eviction tests for LRU. |
| `internal/extractor/extractor.go` | Modify | Replace map cache with LRU; replace `fmt.Errorf("...")` with sentinel errors; add `ctx context.Context` to `ExtractData` and propagate to worker pools and `Baidu.ParseDocument`. |
| `internal/extractor/baidu_client.go` | Modify | Use `http.NewRequestWithContext`; check `ctx.Err()` between chunks; respect `ctx` during `time.Sleep` cooldowns. |
| `internal/app/app.go` | Modify | Pass `a.ctx` (Wails startup context) into `ExtractData`. Translate sentinel errors to user-visible messages. |
| `cmd/server/main.go` | Modify | Pass `c.Request().Context()` into `ExtractData` so client disconnect cancels OCR. |

Existing tests in `extractor_test.go`, `markdown_parser_test.go`, `export_test.go`, `config_test.go` must keep passing without modification — except for one explicit migration: `TestExtractData_CacheHit` will be updated to use the new LRU API.

---

## Task 1: Sync `wails.json` Version to 3.0.0

**Files:**
- Modify: `wails.json` (top of file)

- [ ] **Step 1: Read the current value**

Run: `grep -n version wails.json`
Expected output:
```
8:    "productVersion": "2.0.0",
12:  "version": "2.0.0"
```

- [ ] **Step 2: Edit the file**

Replace the `info` block in `wails.json` so `productVersion` is `"3.0.0"` and the trailing `"version": "2.0.0"` becomes `"3.0.0"`.

After edit, the relevant lines must read:
```json
  "info": {
    "productName": "legal-extractor",
    "companyName": "LegalTech",
    "productVersion": "3.0.0",
    "copyright": "Copyright 2024"
  },
  "wailsjsdir": "./frontend",
  "version": "3.0.0"
```

- [ ] **Step 3: Verify**

Run: `grep -n '"version"\|productVersion' wails.json`
Expected: both occurrences show `3.0.0`.

- [ ] **Step 4: Commit**

```bash
git add wails.json
git commit -m "fix(wails): sync wails.json version to 3.0.0"
```

---

## Task 2: Fix `docker-compose.yml` Env Var Names

**Files:**
- Modify: `docker-compose.yml` (the `environment:` block, around line 14)

The Go side (`internal/config/config.go:111`) binds env vars with `SetEnvPrefix("LEGAL_EXTRACTOR")`, so the only token env var the app reads is `LEGAL_EXTRACTOR_BAIDU_TOKEN`. The current compose file declares `BAIDU_API_KEY` and `BAIDU_SECRET_KEY` which the app never reads — silently broken.

- [ ] **Step 1: Read the env block**

Run: `grep -n -A3 'environment:' docker-compose.yml`

- [ ] **Step 2: Replace the two unused vars with the correct one**

The new `environment:` block must read:
```yaml
    environment:
      # 服务端口
      - PORT=8080
      # 百度 AI Studio Token (与 internal/config/config.go 中的 SetEnvPrefix 对齐)
      - LEGAL_EXTRACTOR_BAIDU_TOKEN=${LEGAL_EXTRACTOR_BAIDU_TOKEN:-}
      # 时区
      - TZ=Asia/Shanghai
```

- [ ] **Step 3: Verify the file is still valid YAML**

Run: `docker compose config 2>&1 | head -20`
Expected: no parsing errors, the env list shows `LEGAL_EXTRACTOR_BAIDU_TOKEN`.

If `docker` is not installed locally, alternative check: `python3 -c "import yaml; yaml.safe_load(open('docker-compose.yml'))" && echo OK`.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml
git commit -m "fix(docker): use LEGAL_EXTRACTOR_BAIDU_TOKEN env var name"
```

---

## Task 3: Define Structured Error Sentinels (TDD — write test first)

**Files:**
- Create: `internal/extractor/errors_test.go`
- Create: `internal/extractor/errors.go`

The current code uses `strings.Contains(errMsg, "PDF_ENCRYPTED_OR_LOCKED")` (`internal/app/app.go:182`) — fragile and non-idiomatic. We replace it with `errors.Is`.

- [ ] **Step 1: Write the failing test**

Create `internal/extractor/errors_test.go`:
```go
package extractor

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrPDFEncrypted_Is(t *testing.T) {
	wrapped := fmt.Errorf("处理失败: %w", ErrPDFEncrypted)
	if !errors.Is(wrapped, ErrPDFEncrypted) {
		t.Fatalf("errors.Is should match wrapped ErrPDFEncrypted")
	}
}

func TestErrUnsupportedFormat_Is(t *testing.T) {
	wrapped := fmt.Errorf("file foo.txt: %w", ErrUnsupportedFormat)
	if !errors.Is(wrapped, ErrUnsupportedFormat) {
		t.Fatalf("errors.Is should match wrapped ErrUnsupportedFormat")
	}
}

func TestErrTokenMissing_Is(t *testing.T) {
	if !errors.Is(ErrTokenMissing, ErrTokenMissing) {
		t.Fatalf("ErrTokenMissing should be Is-equal to itself")
	}
}

func TestSentinels_Distinct(t *testing.T) {
	if errors.Is(ErrPDFEncrypted, ErrUnsupportedFormat) {
		t.Fatal("distinct sentinels must not match each other")
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test ./internal/extractor/ -run TestErr -v`
Expected: build error — `undefined: ErrPDFEncrypted` etc.

- [ ] **Step 3: Implement `errors.go`**

Create `internal/extractor/errors.go`:
```go
// Package extractor: shared sentinel error values.
//
// Use errors.Is(err, ErrXxx) at API boundaries instead of string matching.
// Each sentinel carries a stable, user-facing Chinese message because both the
// desktop UI and the web JSON response surface .Error() directly.
package extractor

import "errors"

// ErrPDFEncrypted is returned when a PDF is password-protected or has
// permission flags that prevent text extraction.
var ErrPDFEncrypted = errors.New("PDF 文档已加密或受保护，无法解析")

// ErrUnsupportedFormat is returned for any extension the extractor cannot handle.
var ErrUnsupportedFormat = errors.New("不支持的文件格式")

// ErrEmptyFile is returned when the input byte slice is zero-length.
var ErrEmptyFile = errors.New("文件内容为空")

// ErrNoRecords is returned when extraction succeeds but no legal entities matched.
var ErrNoRecords = errors.New("未在文档中找到可提取的记录")

// ErrTokenMissing is returned when Baidu OCR is required but no token is configured.
var ErrTokenMissing = errors.New("百度 AI Studio Token 未配置")

// ErrCancelled is returned when ctx is cancelled mid-extraction.
var ErrCancelled = errors.New("提取已被取消")
```

- [ ] **Step 4: Run the test — must pass**

Run: `go test ./internal/extractor/ -run TestErr -v`
Expected: `PASS` for all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/extractor/errors.go internal/extractor/errors_test.go
git commit -m "feat(extractor): add sentinel error types"
```

---

## Task 4: Wire Sentinel Errors Into `extractor.go` and `baidu_client.go`

**Files:**
- Modify: `internal/extractor/extractor.go:84` (unsupported format), `:79` (image disabled)
- Modify: `internal/extractor/baidu_client.go:60` (token missing)
- Modify: `internal/extractor/extractor_test.go:198,206` (assertions use `errors.Is`)

- [ ] **Step 1: Update `extractor.go` error returns**

Replace the existing `fmt.Errorf` calls in `ExtractData`:

At `internal/extractor/extractor.go:79` — change:
```go
		return nil, fmt.Errorf("图片识别功能已暂时禁用（仅支持PDF）")
```
to:
```go
		return nil, fmt.Errorf("图片识别功能已暂时禁用（仅支持PDF）: %w", ErrUnsupportedFormat)
```

At `internal/extractor/extractor.go:84` — change:
```go
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
```
to:
```go
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, ext)
```

- [ ] **Step 2: Update `baidu_client.go` token-missing error**

At `internal/extractor/baidu_client.go:60-62` — change:
```go
	if c.config.Token == "" {
		return nil, fmt.Errorf("百度 AI Studio Token 未配置，请检查 config/conf.yaml")
	}
```
to:
```go
	if c.config.Token == "" {
		return nil, fmt.Errorf("%w: 请检查 config/conf.yaml", ErrTokenMissing)
	}
```

- [ ] **Step 3: Tighten the existing tests to use errors.Is**

Edit `internal/extractor/extractor_test.go`. Replace the body of `TestExtractData_UnsupportedFormat`:
```go
func TestExtractData_UnsupportedFormat(t *testing.T) {
	e := NewExtractor(nil)
	_, err := e.ExtractData([]byte("data"), "test.txt", []string{"defendant"}, nil)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}
```

And replace `TestExtractData_ImageDisabled`:
```go
func TestExtractData_ImageDisabled(t *testing.T) {
	e := NewExtractor(nil)
	_, err := e.ExtractData([]byte("data"), "test.jpg", []string{"defendant"}, nil)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat for image, got %v", err)
	}
}
```

Also add `"errors"` to the import block at the top of `extractor_test.go` if it isn't already imported.

- [ ] **Step 4: Run the package tests — must all pass**

Run: `go test ./internal/extractor/ -v -count=1`
Expected: all tests pass, including the two newly tightened ones.

- [ ] **Step 5: Commit**

```bash
git add internal/extractor/extractor.go internal/extractor/baidu_client.go internal/extractor/extractor_test.go
git commit -m "refactor(extractor): use sentinel errors instead of string compare"
```

---

## Task 5: Translate Sentinels in `app.go` and `cmd/server/main.go`

**Files:**
- Modify: `internal/app/app.go:179-189`
- Modify: `cmd/server/main.go:240-246`

`internal/app/app.go` currently does `strings.Contains(errMsg, "PDF_ENCRYPTED_OR_LOCKED")` — this string was never produced anywhere in the code, so the branch was dead. Replace with proper sentinel matching.

- [ ] **Step 1: Update `internal/app/app.go`**

In `ExtractToPath`, replace lines 179-189:

Find:
```go
	if err != nil {
		// 转换特定错误码
		errMsg := err.Error()
		if strings.Contains(errMsg, "PDF_ENCRYPTED_OR_LOCKED") {
			errMsg = "PDF_ENCRYPTED_OR_LOCKED"
		}
		return ExtractResult{
			Success:      false,
			ErrorMessage: errMsg,
		}
	}
```

Replace with:
```go
	if err != nil {
		return ExtractResult{
			Success:      false,
			ErrorMessage: friendlyExtractError(err),
		}
	}
```

Then at the bottom of `internal/app/app.go`, before the closing of the file (after `OpenFile`), append:
```go
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
```

Add `"errors"` to the import block of `internal/app/app.go` (it currently imports `"strings"` — keep that, it's still used for `.HasSuffix` checks).

- [ ] **Step 2: Update `cmd/server/main.go`**

In `handleExtract`, replace lines 240-246:

Find:
```go
	records, err := extractorInstance.ExtractData(fileData, file.Filename, fields, nil)
	if err != nil {
		extractorInstance.Logger().Error("提取失败", "error", err)
		return c.JSON(http.StatusInternalServerError, ExtractResponse{
			Success: false,
			Error:   fmt.Sprintf("提取失败: %v", err),
		})
	}
```

Replace with:
```go
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
```

Add `"errors"` to the import block of `cmd/server/main.go`.

> Note: This step also adds the new `ctx` parameter to `ExtractData`. The signature change happens in Task 7 — Task 5 cannot compile in isolation. Tasks 5+6+7 are committed together at the end of Task 7.

- [ ] **Step 3: Hold the commit**

Do **not** commit yet — `ExtractData` does not accept `ctx` until Task 7. Continue to Task 6 to add the LRU cache, then Task 7 to add the context parameter, then commit all three together.

---

## Task 6: Implement Bounded LRU Cache (TDD)

**Files:**
- Create: `internal/extractor/cache_test.go`
- Create: `internal/extractor/cache.go`
- Modify: `internal/extractor/extractor.go` (replace `cache map` + `cacheMu`)
- Modify: `internal/extractor/extractor_test.go:272-288` (`TestExtractData_CacheHit`)

The current cache (`extractor.go:30-31`) is `map[string][]Record` with no eviction. A user processing 200 large PDFs in one session can balloon RAM. Replace with a fixed-capacity LRU.

- [ ] **Step 1: Write the failing test**

Create `internal/extractor/cache_test.go`:
```go
package extractor

import (
	"sync"
	"testing"
)

func TestLRUCache_PutGet(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", []Record{{"defendant": "X"}})
	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected hit for key a")
	}
	if got[0]["defendant"] != "X" {
		t.Fatalf("got %v", got)
	}
}

func TestLRUCache_EvictsOldest(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", []Record{{"k": "1"}})
	c.Put("b", []Record{{"k": "2"}})
	c.Put("c", []Record{{"k": "3"}}) // should evict "a"

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should have been evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b should still be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should still be present")
	}
}

func TestLRUCache_GetPromotes(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", []Record{{"k": "1"}})
	c.Put("b", []Record{{"k": "2"}})
	_, _ = c.Get("a")             // touch a -> a now most-recent
	c.Put("c", []Record{{"k": "3"}}) // should evict b, not a

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted (least recent)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should still be present after promotion")
	}
}

func TestLRUCache_ZeroCapacity_IsNoOp(t *testing.T) {
	c := NewRecordCache(0)
	c.Put("a", []Record{{"k": "1"}})
	if _, ok := c.Get("a"); ok {
		t.Fatal("zero-capacity cache must never store")
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	c := NewRecordCache(50)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			c.Put(key, []Record{{"k": key}})
			_, _ = c.Get(key)
		}(i)
	}
	wg.Wait()
	// no race / panic = pass; run with -race
}
```

- [ ] **Step 2: Run the test — must fail**

Run: `go test ./internal/extractor/ -run TestLRU -v`
Expected: build error — `undefined: NewRecordCache`.

- [ ] **Step 3: Implement `cache.go`**

Create `internal/extractor/cache.go`:
```go
package extractor

import (
	"container/list"
	"sync"
)

// RecordCache is a thread-safe fixed-capacity LRU keyed by SHA-256 hex string.
// Capacity 0 disables caching entirely (Put becomes a no-op).
type RecordCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List               // front = most recent, back = least recent
	index    map[string]*list.Element // key -> *list.Element holding *cacheEntry
}

type cacheEntry struct {
	key     string
	records []Record
}

// NewRecordCache returns a cache that evicts the least-recently-used entry
// once size exceeds capacity. capacity <= 0 disables caching.
func NewRecordCache(capacity int) *RecordCache {
	return &RecordCache{
		capacity: capacity,
		ll:       list.New(),
		index:    make(map[string]*list.Element),
	}
}

// Get returns the cached records for key, promoting the entry to most-recent.
func (c *RecordCache) Get(key string) ([]Record, bool) {
	if c.capacity <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).records, true
}

// Put inserts or refreshes an entry, evicting the least-recently-used entry
// if capacity is exceeded.
func (c *RecordCache) Put(key string, records []Record) {
	if c.capacity <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.index[key]; ok {
		el.Value.(*cacheEntry).records = records
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&cacheEntry{key: key, records: records})
	c.index[key] = el

	for c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.index, oldest.Value.(*cacheEntry).key)
	}
}

// Len returns the current number of cached entries (mainly for tests).
func (c *RecordCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
```

- [ ] **Step 4: Run cache tests — must pass under -race**

Run: `go test ./internal/extractor/ -run TestLRU -race -v -count=1`
Expected: 5 PASSes.

- [ ] **Step 5: Replace map cache in `Extractor`**

Edit `internal/extractor/extractor.go`. Change the struct and constructor.

Find (lines 27-44):
```go
// Extractor 处理器，负责协调不同格式的提取策略
type Extractor struct {
	logger      *slog.Logger
	baiduClient *BaiduClient
	cache       map[string][]Record
	cacheMu     sync.RWMutex
}

// NewExtractor 创建一个新的提取器实例
func NewExtractor(logger *slog.Logger) *Extractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Extractor{
		logger:      logger,
		baiduClient: NewBaiduClient(logger),
		cache:       make(map[string][]Record),
	}
}
```

Replace with:
```go
// defaultCacheCapacity bounds in-memory record cache. ~50 PDFs at typical
// extraction sizes (a few KB of records each) is well under 5 MB resident.
const defaultCacheCapacity = 50

// Extractor 处理器，负责协调不同格式的提取策略
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
```

Then in `ExtractData`, replace the cache check (lines 64-70) and cache write (lines 91-96).

Find:
```go
	fileHash := e.calculateHash(fileData)
	e.cacheMu.RLock()
	if cached, ok := e.cache[fileHash]; ok {
		e.logger.Info("命中内容哈希缓存，跳过提取", "file", fileName, "hash", fileHash[:8])
		e.cacheMu.RUnlock()
		return cached, nil
	}
	e.cacheMu.RUnlock()
```
Replace with:
```go
	fileHash := e.calculateHash(fileData)
	if cached, ok := e.cache.Get(fileHash); ok {
		e.logger.Info("命中内容哈希缓存，跳过提取", "file", fileName, "hash", fileHash[:8])
		return cached, nil
	}
```

And find:
```go
	// 2. 写入缓存 (仅当结果非空时)
	if len(records) > 0 {
		e.cacheMu.Lock()
		e.cache[fileHash] = records
		e.cacheMu.Unlock()
	}
```
Replace with:
```go
	// 2. 写入缓存 (仅当结果非空时)
	if len(records) > 0 {
		e.cache.Put(fileHash, records)
	}
```

Also remove `"sync"` from `extractor.go`'s imports if it is no longer used elsewhere in the file (verify by searching for `sync.` after editing — if `sync.WaitGroup` is still used in worker pools, keep the import).

- [ ] **Step 6: Update `TestExtractData_CacheHit` to use the new API**

In `internal/extractor/extractor_test.go:272-288`, replace the body:
```go
func TestExtractData_CacheHit(t *testing.T) {
	e := NewExtractor(nil)
	data := []byte("test data for cache")
	hash := e.calculateHash(data)

	e.cache.Put(hash, []Record{{"defendant": "缓存张三"}})

	records, err := e.ExtractData(data, "test.pdf", []string{"defendant"}, nil)
	if err != nil {
		t.Fatalf("缓存命中不应报错: %v", err)
	}
	if len(records) != 1 || records[0]["defendant"] != "缓存张三" {
		t.Error("应返回缓存中的数据")
	}
}
```

(This still calls the 4-arg `ExtractData` because Task 7 hasn't run yet. The test will be updated again in Task 7.)

Also update `TestNewExtractor` (line 191-193): change `if e.cache == nil` check to keep working — since `NewRecordCache` always returns a non-nil `*RecordCache`, the existing assertion still holds. No edit required.

- [ ] **Step 7: Run all extractor tests under -race**

Run: `go test ./internal/extractor/ -race -count=1 -v`
Expected: every test passes including the previously-existing parsing tests.

- [ ] **Step 8: Hold the commit**

Same reason as Task 5 — wait until Task 7 lands the `ctx` parameter, then commit Tasks 5+6+7 together.

---

## Task 7: Add `context.Context` Parameter to `ExtractData` and Plumb Through

**Files:**
- Modify: `internal/extractor/extractor.go` — `ExtractData`, `batchExtractLocalPdf`, `extractViaWinOcr`, `extractPdf` signatures
- Modify: `internal/extractor/baidu_client.go` — `ParseDocument`, `callBaiduAPI` signatures, use `http.NewRequestWithContext`, ctx-aware sleeps
- Modify: `internal/extractor/extractor_test.go` — every call site of `ExtractData`
- Modify: `internal/app/app.go` — call sites in `ExtractToPath` and `PreviewData`
- Modify: `cmd/server/main.go` — already updated in Task 5; this just makes that change compile

- [ ] **Step 1: Add ctx-cancellation test (write first, will fail to compile until step 3)**

Append to `internal/extractor/extractor_test.go`:
```go
func TestExtractData_ContextCancelled(t *testing.T) {
	e := NewExtractor(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := e.ExtractData(ctx, []byte("data"), "test.pdf", []string{"defendant"}, nil)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled when ctx cancelled, got %v", err)
	}
}
```

Add `"context"` to the imports if missing.

- [ ] **Step 2: Run — expect build failure**

Run: `go test ./internal/extractor/ -run TestExtractData_ContextCancelled -v`
Expected: build error mentioning `too many arguments` or `undefined`.

- [ ] **Step 3: Update `ExtractData` signature and add ctx check**

In `internal/extractor/extractor.go`, change the signature and the very first body line:

Find:
```go
// ExtractData 根据文件类型选择提取策略
func (e *Extractor) ExtractData(fileData []byte, fileName string, fields []string, onProgress ProgressCallback) ([]Record, error) {
	e.logger.Info("开始提取数据", "file", fileName, "size", len(fileData), "fields", fields)
```

Replace with:
```go
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
```

Add `"context"` to the imports in `extractor.go` (it's not currently imported at the package level; only inside the function body via `context.WithTimeout`).

Then update the dispatch to pass ctx through:

Find:
```go
	switch ext {
	case ".pdf":
		records, err = e.extractPdf(fileData, fields, onProgress)
```
Replace with:
```go
	switch ext {
	case ".pdf":
		records, err = e.extractPdf(ctx, fileData, fields, onProgress)
```

(Leave `extractFromDocx` unchanged — it's pure-CPU, no IO worth cancelling. But also fast.)

- [ ] **Step 4: Update `extractPdf` to accept ctx and pass it on**

Find the function signature on line 108:
```go
func (e *Extractor) extractPdf(fileData []byte, fields []string, onProgress ProgressCallback) ([]Record, error) {
```
Replace with:
```go
func (e *Extractor) extractPdf(ctx context.Context, fileData []byte, fields []string, onProgress ProgressCallback) ([]Record, error) {
```

Inside `extractPdf`, find the local-fast-path call (line ~152):
```go
		return e.batchExtractLocalPdf(fileData, fields, totalPages, onProgress)
```
Replace with:
```go
		return e.batchExtractLocalPdf(ctx, fileData, fields, totalPages, onProgress)
```

Find the Baidu call (line ~160):
```go
		return e.baiduClient.ParseDocument(fileData, true, onProgress)
```
Replace with:
```go
		return e.baiduClient.ParseDocument(ctx, fileData, true, onProgress)
```

Find the WinOcr call (line ~164):
```go
	return e.extractViaWinOcr(fileData, totalPages, onProgress)
```
Replace with:
```go
	return e.extractViaWinOcr(ctx, fileData, totalPages, onProgress)
```

- [ ] **Step 5: Update `batchExtractLocalPdf` signature and add cancellation polling**

Find the function signature on line 184:
```go
func (e *Extractor) batchExtractLocalPdf(fileData []byte, fields []string, totalPages int, onProgress ProgressCallback) ([]Record, error) {
```
Replace with:
```go
func (e *Extractor) batchExtractLocalPdf(ctx context.Context, fileData []byte, fields []string, totalPages int, onProgress ProgressCallback) ([]Record, error) {
```

Inside the worker-pool result drain loop (currently around line 252-260), add a ctx check.

Find:
```go
	var allPageResults []pageResult
	processedCount := 0
	for res := range results {
		processedCount++
		if onProgress != nil {
			onProgress(processedCount, totalPages, "正在进行文本层逻辑分析...")
		}
		if len(res.records) > 0 {
			allPageResults = append(allPageResults, res)
		}
	}
```

Replace with:
```go
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
```

- [ ] **Step 6: Update `extractViaWinOcr` signature and add cancellation polling**

Find the function signature on line 276:
```go
func (e *Extractor) extractViaWinOcr(fileData []byte, totalPages int, onProgress ProgressCallback) ([]Record, error) {
```
Replace with:
```go
func (e *Extractor) extractViaWinOcr(ctx context.Context, fileData []byte, totalPages int, onProgress ProgressCallback) ([]Record, error) {
```

Inside the worker goroutine (around lines 318-340), wrap the `exec.Command` line so the subprocess is cancelled with ctx:

Find:
```go
				cmd := exec.Command(bridgePath, tempFile.Name(), fmt.Sprintf("%d", pageNum))
				output, err := cmd.CombinedOutput()
```
Replace with:
```go
				cmd := exec.CommandContext(ctx, bridgePath, tempFile.Name(), fmt.Sprintf("%d", pageNum))
				output, err := cmd.CombinedOutput()
```

Then in the result-collection loop (around line 357-365), apply the same ctx-check pattern as Step 5:

Find:
```go
	var allPageResults []pageResult
	processed := 0
	for res := range results {
		processed++
		if onProgress != nil {
			onProgress(processed, totalPages, fmt.Sprintf("正在调用系统识别引擎提取第 %d 页内容...", res.pageNum))
		}
		if len(res.records) > 0 {
			allPageResults = append(allPageResults, res)
		}
	}
```

Replace with:
```go
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
```

- [ ] **Step 7: Update `BaiduClient.ParseDocument` and `callBaiduAPI` to accept ctx**

In `internal/extractor/baidu_client.go`:

Find the function signature on line 53:
```go
func (c *BaiduClient) ParseDocument(fileData []byte, isPdf bool, onProgress ProgressCallback) ([]Record, error) {
```
Replace with:
```go
func (c *BaiduClient) ParseDocument(ctx context.Context, fileData []byte, isPdf bool, onProgress ProgressCallback) ([]Record, error) {
```

Find every call to `c.callBaiduAPI(chunkBuffer.Bytes(), true, onProgress)` and `c.callBaiduAPI(fileData, ...)` (there are 3 sites: chunked path ~line 109, single-shot pdf path ~line 133, image path ~line 144) and prepend `ctx`:

Site 1 (line 109):
```go
							pages, err = c.callBaiduAPI(ctx, chunkBuffer.Bytes(), true, onProgress)
```

Site 2 (line 133):
```go
					pages, err := c.callBaiduAPI(ctx, fileData, true, onProgress)
```

Site 3 (line 144):
```go
			pages, err := c.callBaiduAPI(ctx, fileData, false, onProgress)
```

Find every `time.Sleep` inside `ParseDocument` (two of them: 20s retry-backoff, 10s cooldown) and replace with a ctx-aware helper. Add this helper at the bottom of `baidu_client.go`:
```go
// sleepCtx is time.Sleep that returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

Then replace:
```go
								time.Sleep(20 * time.Second) // 收到 500 后重试需等待更久，给服务器释放资源
```
with:
```go
								if err := sleepCtx(ctx, 20*time.Second); err != nil {
									return nil, ErrCancelled
								}
```

And replace:
```go
							c.logger.Info("分块处理完成，进入 10 秒冷却期以释放云端算力...")
							time.Sleep(10 * time.Second)
```
with:
```go
							c.logger.Info("分块处理完成，进入 10 秒冷却期以释放云端算力...")
							if err := sleepCtx(ctx, 10*time.Second); err != nil {
								return nil, ErrCancelled
							}
```

Now `callBaiduAPI`. Change its signature:

Find:
```go
func (c *BaiduClient) callBaiduAPI(fileData []byte, isPdf bool, onProgress ProgressCallback) ([]string, error) {
```
Replace with:
```go
func (c *BaiduClient) callBaiduAPI(ctx context.Context, fileData []byte, isPdf bool, onProgress ProgressCallback) ([]string, error) {
```

Then change the request construction (around line 197-201):

Find:
```go
	req, err := http.NewRequest("POST", c.config.ApiUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
```
Replace with:
```go
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.ApiUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
```

Add `"context"` to the imports in `baidu_client.go`.

- [ ] **Step 8: Update every test caller of `ExtractData` in `extractor_test.go`**

Replace each call site:

`TestExtractData_UnsupportedFormat`:
```go
	_, err := e.ExtractData(context.Background(), []byte("data"), "test.txt", []string{"defendant"}, nil)
```

`TestExtractData_ImageDisabled`:
```go
	_, err := e.ExtractData(context.Background(), []byte("data"), "test.jpg", []string{"defendant"}, nil)
```

`TestExtractData_CacheHit`:
```go
	records, err := e.ExtractData(context.Background(), data, "test.pdf", []string{"defendant"}, nil)
```

Make sure `"context"` is imported (already added in Step 1).

- [ ] **Step 9: Update `internal/app/app.go` call sites**

In `ExtractToPath` (around line 172):

Find:
```go
	records, err := a.extractor.ExtractData(fileData, inputPath, fields, func(current, total int, message string) {
```
Replace with:
```go
	records, err := a.extractor.ExtractData(a.ctx, fileData, inputPath, fields, func(current, total int, message string) {
```

In `PreviewData` (around line 271):

Find:
```go
	records, err := a.extractor.ExtractData(fileData, inputPath, fields, func(current, total int, message string) {
```
Replace with:
```go
	records, err := a.extractor.ExtractData(a.ctx, fileData, inputPath, fields, func(current, total int, message string) {
```

(The Wails app context `a.ctx` is set in `Startup` and lives for the app lifetime — fine for desktop, where we don't yet expose per-request cancellation. Iteration 2 adds a `Cancel()` binding.)

- [ ] **Step 10: Run the entire build + test suite**

Run: `go build ./...` — must succeed.
Run: `go test ./... -race -count=1` — every test must pass, including `TestExtractData_ContextCancelled`.

- [ ] **Step 11: Commit Tasks 5 + 6 + 7 together**

```bash
git add internal/extractor/cache.go internal/extractor/cache_test.go \
        internal/extractor/extractor.go internal/extractor/extractor_test.go \
        internal/extractor/baidu_client.go \
        internal/app/app.go cmd/server/main.go
git commit -m "feat(extractor): bounded LRU cache + context-aware extraction

- Replace unbounded map cache with capacity-50 thread-safe LRU
- Add context.Context parameter to ExtractData and propagate through
  baidu_client, batchExtractLocalPdf, extractViaWinOcr
- Use http.NewRequestWithContext + ctx-aware sleeps so client disconnects
  cancel in-flight Baidu OCR calls (was: ran to completion regardless)
- exec.CommandContext on WinOcrBridge.exe so subprocess dies with ctx
- friendlyExtractError() in app.go translates sentinel errors into stable
  frontend codes (replaces dead strings.Contains branch)
- HTTP status codes in cmd/server: 400 for unsupported/empty, 499 for
  cancelled, 500 default"
```

---

## Task 8: End-to-End Cancellation Smoke Test

**Files:**
- Create: `internal/extractor/cancellation_test.go`

This test exercises the full propagation chain on a real (small) PDF to prove cancellation actually unwinds the worker pool — not just the entry-point ctx check.

- [ ] **Step 1: Write the test**

Create `internal/extractor/cancellation_test.go`:
```go
package extractor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestExtractData_CancelDuringWork cancels the context while the worker pool
// is still draining results and expects ErrCancelled. Uses a pre-existing
// fixture if available, otherwise skips.
func TestExtractData_CancelDuringWork(t *testing.T) {
	candidates := []string{
		filepath.Join("testdata", "sample.pdf"),
		filepath.Join("..", "..", "testdata", "sample.pdf"),
	}
	var pdfPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			pdfPath = p
			break
		}
	}
	if pdfPath == "" {
		t.Skip("no sample PDF fixture; skipping (add testdata/sample.pdf to enable)")
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	e := NewExtractor(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 50ms — long enough that some work has started, short
	// enough that real local extraction wouldn't finish.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = e.ExtractData(ctx, data, "sample.pdf", []string{"defendant"}, nil)
	if err == nil {
		t.Fatal("expected non-nil error when cancelled mid-flight")
	}
	if !errors.Is(err, ErrCancelled) {
		t.Logf("note: got %v (acceptable if extraction completed before cancel)", err)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/extractor/ -run TestExtractData_CancelDuringWork -race -count=1 -v`
Expected: PASS (or SKIP if no fixture). If a fixture exists, the test must complete within 5 seconds.

- [ ] **Step 3: Commit**

```bash
git add internal/extractor/cancellation_test.go
git commit -m "test(extractor): end-to-end cancellation smoke test"
```

---

## Task 9: Final Verification

**Files:** none modified — verification only.

- [ ] **Step 1: Run the whole suite under -race**

Run: `go test ./... -race -count=1`
Expected: PASS, no race warnings.

- [ ] **Step 2: Run the linter**

Run: `make lint` (or `golangci-lint run ./...` if `make` isn't available).
Expected: clean. If new lint errors appear (e.g., `errcheck` on `time.After`), fix them inline.

- [ ] **Step 3: Build the desktop binary**

Run: `wails build -platform darwin/$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/')`
Expected: build succeeds. `build/bin/legal-extractor.app` exists.

- [ ] **Step 4: Build the Web binary**

Run: `go build -o /tmp/legal-server ./cmd/server`
Expected: `/tmp/legal-server` is created without error.

- [ ] **Step 5: Tag the iteration in `wails.json`**

Edit `wails.json` again — bump from `3.0.0` (set in Task 1) to `3.1.0`:
```json
    "productVersion": "3.1.0",
```
and:
```json
  "version": "3.1.0"
```

Verify with: `grep -n '"version"\|productVersion' wails.json` → both should now show `3.1.0`.

- [ ] **Step 6: Tag the iteration in the changelog**

Edit `CHANGELOG.md`. Insert a new section above the existing 3.0.0 entry:
```markdown
## [3.1.0] - 2026-05-03

### 修复
- `wails.json` 中残留的 2.0.0 版本号
- `docker-compose.yml` 引用了应用从未读取的 `BAIDU_API_KEY/SECRET_KEY` 环境变量

### 新增
- 基于 SHA-256 的有界 LRU 缓存（容量 50），替代之前无界增长的内存映射
- `context.Context` 全链路传播：HTTP 请求中止/百度长任务/Windows OCR 子进程都可被取消
- 结构化错误类型（`ErrPDFEncrypted`、`ErrUnsupportedFormat`、`ErrTokenMissing`、`ErrCancelled` 等），消除字符串匹配
```

- [ ] **Step 7: Final commit**

```bash
git add wails.json CHANGELOG.md
git commit -m "chore(release): 3.1.0 reliability hardening"
```

---

## Done Criteria

All of the following are true:
- [ ] `go test ./... -race -count=1` passes.
- [ ] `make lint` is clean.
- [ ] `wails build` for the host platform succeeds.
- [ ] `go build ./cmd/server` succeeds.
- [ ] `wails.json` and `CHANGELOG.md` show 3.1.0.
- [ ] Cache size is bounded — `Extractor.cache.Len()` never exceeds 50 in any test.
- [ ] Cancelling a `context.Context` during `ExtractData` returns `ErrCancelled` (verified by `TestExtractData_ContextCancelled`).
- [ ] `internal/app/app.go` no longer contains `strings.Contains(errMsg, "PDF_ENCRYPTED_OR_LOCKED")`.
