# 子迭代 4b：DOCX 取消传播 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `context.Context` 串到 DOCX 解析路径，让大 docx 文件在解析中途也能响应"停止"按钮。

**Architecture:** Iteration 1 已经把 ctx 传到 PDF / 百度 / WinOCR 三条路径，DOCX 路径是唯一遗漏的盲区。本迭代修改 `docx.go` 两个函数的签名加入 `ctx context.Context` 参数，在三个关键点（函数入口、zip 解析后、XML token 循环每 256 个 token）检查 `ctx.Err()`，命中则返回 `ErrCancelled`。`parseCases` 不动——它是 PDF / DOCX 共享的纯函数，加 ctx 会污染 PDF 路径所有调用点；其内部是 regex over 内存字符串，最坏 megabyte 级 < 100ms，停不下来用户感知不到。

**Tech Stack:** Go 1.25，标准库 `context` / `archive/zip` / `encoding/xml`。无新增依赖。

**Spec:** `docs/superpowers/specs/2026-05-04-engineering-hygiene-trio-design.md` 第 4 节

---

## File Structure

| 文件 | 操作 | 责任 |
|---|---|---|
| `internal/extractor/docx.go` | 编辑 | `extractFromDocx` / `extractTextFromDocx` 签名加 `ctx`；三处 `ctx.Err()` 检查；导入 `context` 包 |
| `internal/extractor/extractor.go` | 编辑 | dispatcher 把 `e.extractFromDocx(fileData, fields)` 改为 `e.extractFromDocx(ctx, fileData, fields)` |
| `internal/extractor/docx_test.go` | 新建 | 单测：预先取消的 ctx 立即返回 `ErrCancelled` |

---

## Task 1: 创建 feature 分支

**Files:** 无（仅分支操作）

- [ ] **Step 1: 确认在 main 且工作树干净**

Run: `git status && git rev-parse --abbrev-ref HEAD`
Expected: `nothing to commit, working tree clean` 且分支名为 `main`。

- [ ] **Step 2: 同步远端**

Run: `git pull --ff-only`
Expected: `Already up to date.` 或正常 fast-forward。

- [ ] **Step 3: 创建分支**

Run: `git checkout -b feature/docx-cancellation`
Expected: `Switched to a new branch 'feature/docx-cancellation'`

---

## Task 2: 写失败测试（TDD red 阶段）

**Files:**
- Create: `internal/extractor/docx_test.go`

- [ ] **Step 1: 创建 docx_test.go**

写入以下内容到 `internal/extractor/docx_test.go`：

```go
package extractor

import (
	"context"
	"errors"
	"testing"
)

// TestExtractFromDocx_PreCancelledCtx 验证 DOCX 路径在 ctx 已取消时立即返回 ErrCancelled，
// 不进行任何 zip / xml 解析工作。该测试是 Iteration 4b 把 context 传播补齐到 DOCX 路径的回归网。
func TestExtractFromDocx_PreCancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 在调用之前就取消

	e := NewExtractor(nil)
	// fileData 故意是非法 docx —— 如果 ctx 检查没有提前生效，zip.NewReader 会报"非 zip"错误，
	// 我们就会拿到那个错误而不是 ErrCancelled，测试失败。
	_, err := e.extractFromDocx(ctx, []byte("not-a-real-docx"), []string{"defendant"})
	if err == nil {
		t.Fatal("expected error from pre-cancelled ctx, got nil")
	}
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试验证它失败（红灯阶段）**

Run: `go test ./internal/extractor/ -run TestExtractFromDocx_PreCancelledCtx -count=1 2>&1 | tail -20`

Expected: **编译失败**——错误信息会类似：
```
./docx_test.go:XX:XX: not enough arguments in call to e.extractFromDocx
	have (context.Context, []byte, []string)
	want ([]byte, []string)
```

这正是 TDD 红灯：测试期望 `extractFromDocx` 接受 ctx，但当前签名还没 ctx。后续 Task 3 修签名后就会 GREEN。

如果不是这个错，停下检查：可能 docx.go 已被改过、或 import 写错了。

---

## Task 3: 修改 `docx.go`，加入 ctx 与三处检查点

**Files:**
- Modify: `internal/extractor/docx.go`

完全重写 `docx.go` 以加入 ctx 传播。三处检查点：
1. `extractFromDocx` 入口
2. `extractTextFromDocx` 解析完 zip 之后、XML decode 之前
3. XML token 循环内，每 256 个 token 检查一次

- [ ] **Step 1: 用 Write 工具完整覆写 `internal/extractor/docx.go`**

写入以下内容（这是整个文件的最终形态，直接覆盖现有 78 行）：

```go
package extractor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// docxCtxCheckInterval 是 XML token 循环里检查 ctx.Err() 的间隔。
// 实测大 docx 单页 ~1500 token，256 给到约每页 5-6 次中断机会，
// 既保证 100ms 量级响应，又不让分支预测开销显著。
const docxCtxCheckInterval = 256

// extractFromDocx 保留原有的本地 DOCX 提取逻辑。
//
// ctx 在三处生效：本函数入口、extractTextFromDocx 解析 zip 之后，以及
// XML token 循环内每 docxCtxCheckInterval 次迭代一次。任意一处命中
// 取消都会返回 ErrCancelled，与 PDF / 百度 / WinOCR 路径保持一致。
func (e *Extractor) extractFromDocx(ctx context.Context, fileData []byte, fields []string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, ErrCancelled
	}

	text, err := extractTextFromDocx(ctx, fileData)
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

// extractTextFromDocx 核心 DOCX 文本提取逻辑。
func extractTextFromDocx(ctx context.Context, fileData []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", ErrCancelled
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

	tokenCount := 0
	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		tokenCount++
		if tokenCount%docxCtxCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return "", ErrCancelled
			}
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

> 与原文件相比的变化：
> - 新增 import `"context"`
> - 新增 const `docxCtxCheckInterval = 256`
> - `extractFromDocx` 加 `ctx` 参数 + 入口检查
> - `extractTextFromDocx` 加 `ctx` 参数 + zip 后检查 + token 循环每 256 次检查
> - 增加完整 doc comment 解释 ctx 生效点

---

## Task 4: 更新 dispatcher 调用点

**Files:**
- Modify: `internal/extractor/extractor.go`

dispatcher 当前调用 `e.extractFromDocx(fileData, fields)`，签名改了之后必须传 ctx。

- [ ] **Step 1: 修改 dispatcher 调用**

用 Edit 工具，old_string 为：

```go
	case ".docx":
		e.logger.Info("使用本地原生逻辑提取 DOCX", "file", fileName)
		records, err = e.extractFromDocx(fileData, fields)
```

new_string 为：

```go
	case ".docx":
		e.logger.Info("使用本地原生逻辑提取 DOCX", "file", fileName)
		records, err = e.extractFromDocx(ctx, fileData, fields)
```

---

## Task 5: 验证测试通过 + 全量回归

**Files:** 无（仅校验）

- [ ] **Step 1: 跑新增的 DOCX 取消测试（TDD green 阶段）**

Run: `go test ./internal/extractor/ -run TestExtractFromDocx_PreCancelledCtx -count=1 -v 2>&1 | tail -10`

Expected: `--- PASS: TestExtractFromDocx_PreCancelledCtx` 然后 `PASS` + `ok`。

- [ ] **Step 2: 跑 extractor 包全部测试 -race**

Run: `go test ./internal/extractor/ -race -count=1 2>&1 | tail -3`

Expected: `PASS` + `ok  legal-extractor/internal/extractor`。

- [ ] **Step 3: 验证测试数变成 34**

Run: `go test ./internal/extractor/ -count=1 -v 2>&1 | grep -c '^--- PASS:'`

Expected: `34` （baseline 33 + 新增 1）。

- [ ] **Step 4: 跑全工程测试 + vet + build**

Run: `go test ./... -race -count=1 2>&1 | tail -10`
Expected: 三行 `ok ...`，无 FAIL。

Run: `go vet ./...`
Expected: 无输出。

Run: `go build ./...`
Expected: 无输出。

---

## Task 6: 提交并推送 PR

**Files:** 无（仅 git 操作）

- [ ] **Step 1: 检查改动**

Run: `git status --short && echo --- && git diff --stat`
Expected：仅以下三个文件变化：
```
 M internal/extractor/docx.go
 M internal/extractor/extractor.go
 ?? internal/extractor/docx_test.go
```

- [ ] **Step 2: 提交**

Run:
```bash
git add internal/extractor/docx.go internal/extractor/extractor.go internal/extractor/docx_test.go
git commit -m "feat: docx 解析路径接入取消传播"
```

Expected: 一个新 commit。

- [ ] **Step 3: 推送**

Run: `git push -u origin feature/docx-cancellation`
Expected: 远端创建分支并 set upstream。

- [ ] **Step 4: 开 PR**

Run:
```bash
gh pr create --base main --head feature/docx-cancellation \
  --title "feat: docx 解析路径接入取消传播" \
  --body "$(cat <<'EOF'
## 概述

把 \`context.Context\` 串到 DOCX 解析路径，闭合 Iteration 1 留下的取消传播盲区——大 docx 文件在解析中途也能响应"停止"按钮。

## 改动

| 文件 | 改动 |
|---|---|
| \`internal/extractor/docx.go\` | \`extractFromDocx\` / \`extractTextFromDocx\` 签名加 \`ctx context.Context\`；新增三处 \`ctx.Err()\` 检查（函数入口、zip 解析后、XML token 循环每 256 次）；命中取消返回 \`ErrCancelled\` |
| \`internal/extractor/extractor.go\` | dispatcher 把 \`e.extractFromDocx(fileData, fields)\` 改为 \`e.extractFromDocx(ctx, fileData, fields)\` |
| \`internal/extractor/docx_test.go\` | 新增单测验证预先取消的 ctx 立即返回 \`ErrCancelled\` |

## 设计抉择

- **\`parseCases\` 不接 ctx**：它是 PDF / DOCX 共享的纯函数，加 ctx 会污染 PDF 路径所有调用点；内部是 regex over 内存字符串，最坏 megabyte 级 < 100ms，停不下来用户感知不到
- **token 循环间隔 256**：大 docx 单页 ~1500 token，256 给到每页约 5-6 次中断机会，响应延迟 ≤ 100ms，分支预测开销可忽略

## 验证

- 新增 1 个测试 \`TestExtractFromDocx_PreCancelledCtx\` PASS
- \`go test ./internal/extractor/ -race -count=1\` 测试数从 33 → 34，全 PASS
- \`go test ./... -race -count=1\` 全工程绿
- \`go vet ./...\` / \`go build ./...\` 干净
EOF
)"
```

Expected: 输出 PR URL。

- [ ] **Step 5: 等 CI**

Run: `gh pr checks --watch`
Expected: Lint / Test / Build Check / GitGuardian Security 四项全部 `pass`。

如果有 FAIL：停下，按错误回 Task 3/4 修正。不要 force-push。

- [ ] **Step 6: Squash merge**

把 `<NUM>` 替换为上一步的 PR 编号。

Run:
```bash
gh pr merge <NUM> --squash --delete-branch \
  --subject "feat: docx 解析路径接入取消传播" \
  --body "把 context.Context 串到 docx.go 的 extractFromDocx / extractTextFromDocx 两个函数，加入三处取消检查（函数入口、zip 解析后、XML token 循环每 256 次），命中返回 ErrCancelled。同时更新 dispatcher 传 ctx 与新增预取消测试。闭合 Iteration 1 留下的 DOCX 路径取消传播盲区。"
```

Expected: PR 合入；远端分支自动删除；本地 main fast-forward。

- [ ] **Step 7: 同步本地**

Run:
```bash
git checkout main
git pull --ff-only
git fetch --prune
```

Expected: 本地 main 与 origin/main 一致；本地 `feature/docx-cancellation` 自动消失。

---

## Done Criteria

- [ ] PR 已通过 CI 并合入 main
- [ ] `git log --oneline -3` 头部显示 `feat: docx 解析路径接入取消传播` 的 squash commit
- [ ] `go test ./internal/extractor/ -count=1 -v 2>&1 | grep -c '^--- PASS:'` 输出 `34`
- [ ] `go test ./... -race -count=1 && go vet ./... && go build ./...` 全部干净
- [ ] `grep -n 'context.Context' internal/extractor/docx.go` 显示 `extractFromDocx` 与 `extractTextFromDocx` 都有 ctx 参数
- [ ] commit / PR 标题均符合 `type: 中文描述` 规范，无 scope、无英文 type、无 Co-Authored-By
