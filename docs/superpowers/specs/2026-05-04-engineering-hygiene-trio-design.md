# Engineering Hygiene Trio — Iteration 4 Design

**状态：** Approved by user
**日期：** 2026-05-04
**前置：** Iteration 1（可靠性硬化）、Iteration 2（取消 + ETA）、Iteration 3（提取器拆分）已合入 main

## 1. 背景

前三轮迭代各自闭环良好，但留下三处工程化缺口：

1. **版本/变更日志漂移**——Iteration 2、3 合入 main 时未更新 `CHANGELOG.md` 与 `wails.json`；`cmd/server/main.go` 索引路由仍硬编码 `version: 3.0.0`。
2. **DOCX 取消盲区**——Iteration 1 把 `context.Context` 串到 PDF / 百度 / WinOCR 三条路径，唯独 `extractFromDocx` 仍是 ctx-less 的；大 docx 文件解析中途无法响应"停止"。
3. **前端零测试**——`frontend/` 目录从未引入测试框架；Iteration 2 新增的 `utils/eta.ts` 是有数学逻辑的纯函数，正适合首发覆盖。

本迭代以**工程化卫生补强**为主题，把三个缺口分别用三个最小化、可独立合并的 PR 逐一闭合。

## 2. 交付结构

**三个串行 PR，按 B → C → A 顺序合入 main。** 理由：
- 与 Iteration 1/2/3 的 PR 节奏一致
- B 极小（仅文档 + 三处版本字符串），先合入让 CHANGELOG 与代码同步
- C、A 之间无任何耦合；串行只是稳妥不浪费 CI 时间
- 任一子项 CI 红可独立修，不连累其它

| 子迭代 | 分支 | PR 标题 | 估算 |
|---|---|---|---|
| 4a (B) | `chore/release-3.3.0` | `chore: 发布 3.3.0 版本并补齐变更日志` | ~10 分钟 |
| 4b (C) | `feature/docx-cancellation` | `feat: docx 解析路径接入取消传播` | ~1 天 |
| 4c (A) | `feature/frontend-vitest` | `test: 引入前端单元测试基建并覆盖核心工具` | ~0.5–1 天 |

---

## 3. 子迭代 4a：CHANGELOG + 版本号补齐 (v3.3.0)

### 3.1 改动范围

| 文件 | 操作 | 详情 |
|---|---|---|
| `CHANGELOG.md` | 编辑 | 在 `[3.1.0]` 之上插入 `[3.2.0]`、`[3.3.0]` 两段；文末追加两条 release tag 链接 |
| `wails.json` | 编辑 | `productVersion` 与顶层 `version` 由 `3.1.0` → `3.3.0` |
| `cmd/server/main.go` | 编辑 | `handleIndex` 返回的 `"version": "3.0.0"` → `"3.3.0"` |

### 3.2 新增 CHANGELOG 段落（终稿）

```markdown
## [3.3.0] - 2026-05-04

### 重构
- 把 `internal/extractor/extractor.go` 按策略拆为 5 个聚焦文件：调度器（extractor.go）、本地 PDF 文本层与 Worker Pool（pdf_local.go）、Windows OCR 兜底（pdf_winocr.go）、DOCX 解析（docx.go）、正则解析与文本归并（parsing.go）。零行为变更，公共 API 与导出符号 byte-identical。

## [3.2.0] - 2026-05-04

### 新增
- 加载浮层加入"停止"按钮，桌面与 Web 模式均可中止进行中的提取任务
- 加载浮层显示估算剩余时间（"约 N 秒"/"约 N 分 M 秒"/"估算中..."/"即将完成"）
- 桌面端 `App.CancelExtraction()` Wails 绑定，串通 Iteration 1 的 ctx 链路
- Web 端 `AbortController` 路径，配合后端 `c.Request().Context().Done()` 真实中止
```

文末链接追加：

```markdown
[3.3.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v3.3.0
[3.2.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v3.2.0
```

### 3.3 验收

- `grep -n '"version"\|productVersion' wails.json` 两行均显示 `3.3.0`
- `grep -n '"version"' cmd/server/main.go` 显示 `"3.3.0"`
- `CHANGELOG.md` 头部按时间倒序列出 3.3.0、3.2.0、3.1.0、3.0.0 四段
- `go build ./... && go test ./... -race -count=1` 干净通过（仅文本/字符串改动，CI 必然全绿）

### 3.4 提交规范

单 commit：`chore: 发布 3.3.0 版本并补齐变更日志`

---

## 4. 子迭代 4b：DOCX 取消传播

### 4.1 改动范围

| 文件 | 操作 | 详情 |
|---|---|---|
| `internal/extractor/extractor.go` | 编辑 | dispatcher 把 `e.extractFromDocx(fileData, fields)` → `e.extractFromDocx(ctx, fileData, fields)` |
| `internal/extractor/docx.go` | 编辑 | `extractFromDocx` 与 `extractTextFromDocx` 签名加 `ctx context.Context`；进入即查 ctx；XML token 循环每 256 个 token 检查一次 |
| `internal/extractor/docx_test.go` | 新建 | 单测：预先取消的 ctx 立即返回 `ErrCancelled` |

### 4.2 ctx 检查策略（关键设计点）

**在 docx 路径加 3 个检查点：**
1. `extractFromDocx` 函数入口——拒绝预取消的 ctx
2. `extractTextFromDocx` 进入 zip reader 之后、XML decode 之前——拒绝预取消的 ctx
3. `extractTextFromDocx` 的 XML token 循环内，每处理 256 个 token 检查一次

**`parseCases` 不接 ctx 参数。** 理由：
- 它是 PDF / DOCX 共享的纯函数，加 ctx 会污染 PDF 路径所有调用点
- 内部是 regex over 内存字符串，最坏 megabyte 级 < 100ms，停不下来用户感知不到

### 4.3 错误返回

DOCX 路径 ctx 取消时返回 `ErrCancelled`（与 PDF 路径一致），由上层 `internal/app/app.go` / `cmd/server/main.go` 翻译成对应的用户可见错误码（HTTP 499 或前端"已取消"提示）。

### 4.4 测试（最小可信集）

`docx_test.go` 仅一个用例：

```go
func TestExtractFromDocx_PreCancelledCtx(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    e := NewExtractor(slog.Default())
    _, err := e.extractFromDocx(ctx, []byte("dummy"), nil)
    if !errors.Is(err, ErrCancelled) {
        t.Fatalf("expected ErrCancelled, got %v", err)
    }
}
```

不构造大 docx fixture——容易因 token 循环计数边界引入 flake，且收益边际。

### 4.5 验收

- `go test ./internal/extractor/ -race -count=1` 通过，新增 1 个测试
- 现有 33 个测试不变
- `go vet ./... && go build ./...` 干净
- CI 四项全绿

### 4.6 提交规范

单 commit：`feat: docx 解析路径接入取消传播`

---

## 5. 子迭代 4c：前端 Vitest 基建

### 5.1 工具与依赖

新增 devDependencies（运行时依赖不变）：
- `vitest` ——测试 runner，与 Vite 同源零配置
- `jsdom` ——`api.ts` 的 AbortController 行为测试需要 DOM 环境
- `@vitest/coverage-v8` ——覆盖率报告（不设阈值门槛）

**不引入** `@vue/test-utils`——首批测试只覆盖纯函数与 adapter 类，无 Vue 组件挂载需求；待后续真正测组件时再加。

### 5.2 文件布局

| 文件 | 操作 | 详情 |
|---|---|---|
| `frontend/package.json` | 编辑 | 加 3 个 devDependencies；加 `test` / `test:coverage` 脚本 |
| `frontend/package-lock.json` | 自动 | `npm install` 生成 |
| `frontend/vitest.config.ts` | 新建 | 最小配置：`environment: 'jsdom'`、glob `**/*.{test,spec}.ts` |
| `frontend/src/utils/eta.test.ts` | 新建 | 与 eta.ts 同目录 colocate |
| `frontend/src/services/api.test.ts` | 新建 | 与 api.ts 同目录 colocate |
| `.github/workflows/ci.yml` | 编辑 | 新增 `frontend-test` job，与 Go Lint/Test/Build 并行 |

### 5.3 `eta.test.ts` 测试矩阵（共 9 个用例）

覆盖 `formatEta(samples, total)` 全部分支：

| # | 输入 | 期望 |
|---|---|---|
| 1 | `total = 0` | `估算中...` |
| 2 | `samples = []` | `估算中...` |
| 3 | 8 秒滑窗内只有 1 个样本 | `估算中...` |
| 4 | 滑窗内 2 个样本但 dCur ≤ 0（卡住）| `估算中...` |
| 5 | 当前已到 total（remaining = 0） | `即将完成` |
| 6 | ETA = 12 秒 | `约 12 秒` |
| 7 | ETA = 120 秒（整 2 分） | `约 2 分` |
| 8 | ETA = 150 秒（2 分 30 秒） | `约 2 分 30 秒` |
| 9 | 早期慢启动（10 秒前的样本被滑窗丢弃，只看最近）| 滑窗外样本不影响估算 |

### 5.4 `api.test.ts` 测试矩阵（共 3 个用例）

| # | 场景 | 验证点 |
|---|---|---|
| 1 | `window.go` 存在 | `isDesktopMode()` 返回 `true` |
| 2 | `window.go` 不存在 | `isDesktopMode()` 返回 `false`、`isWebMode()` 返回 `true` |
| 3 | WebAdapter `cancelExtraction()` 中止 in-flight controller | mock fetch + AbortController；调用 cancel 后 `activeController` 被清空 |

mock 策略：用 `vi.stubGlobal('window', {...})` / `vi.stubGlobal('fetch', vi.fn())`，不真发请求。

### 5.5 CI 集成

`.github/workflows/ci.yml` 新增 job：

```yaml
frontend-test:
  name: Frontend Test
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: '20'
        cache: 'npm'
        cache-dependency-path: frontend/package-lock.json
    - name: Install
      working-directory: frontend
      run: npm ci
    - name: Run tests
      working-directory: frontend
      run: npm run test -- --run
```

与现有 Lint / Test / Build Check 并行，不阻塞 Go 链路；失败独立显示。

### 5.6 提交结构（4 个 commit）

| # | Commit | 范围 |
|---|---|---|
| 1 | `chore: 引入 vitest 前端测试基建` | package.json/package-lock.json/vitest.config.ts |
| 2 | `test: 覆盖 eta 估算辅助函数` | eta.test.ts |
| 3 | `test: 覆盖前端 api 适配层关键路径` | api.test.ts |
| 4 | `ci: 集成前端单元测试至持续集成` | ci.yml |

### 5.7 验收

- `cd frontend && npm install && npm run test -- --run` 全部通过
- 9 + 3 = 12 个测试 PASS
- `npm run test:coverage` 产出 utils/eta.ts ≈ 100% 覆盖、services/api.ts ≥ 60% 覆盖（不强制门槛）
- CI 多一项 `Frontend Test` 全绿
- Go 侧 4 项 CI 全绿、不受影响

---

## 6. 验收（整体）

- [ ] 三个 PR 均通过 CI 并合入 main
- [ ] `git log --oneline main` 头部按顺序：`chore: 发布 3.3.0...` → `feat: docx 解析路径接入取消传播` → `test: 引入前端单元测试基建...`
- [ ] CHANGELOG 与 wails.json、`cmd/server/main.go` 版本字符串全部对齐到 `3.3.0`
- [ ] DOCX 路径预取消测试通过；现有 33 个 extractor 测试不变
- [ ] 前端 12 个新增测试通过；CI 多一个 `Frontend Test` job
- [ ] 所有 commit 符合 `type: 中文描述`，无 scope、无英文 type、无 Co-Authored-By

## 7. 不在本迭代范围

明确排除（避免范围漂移）：

- macOS / Linux 的 OCR 兜底实现（独立大迭代，需评估 tesseract vs Vision）
- 多文件批量上传与队列
- Web 模式授权机制（已在上轮 brainstorm 决定**暂不做**）
- `baidu_client.go` 拆分（现 279 行，未达拆分阈值）
- 前端 Vue 组件级单测（待 utils/services 覆盖完后下一轮）
- 覆盖率门槛（先产出报告，下一轮再加阈值）
- Release tag 推送（CHANGELOG 文末写好链接，但不实际打 tag——属发布动作，独立流程）
