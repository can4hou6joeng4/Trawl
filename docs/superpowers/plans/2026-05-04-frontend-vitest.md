# 子迭代 4c：前端 Vitest 基建 + 首批单测 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 `frontend/` 引入 Vitest 测试框架，覆盖 ETA 估算函数（9 用例）与 API 适配层关键路径（3 用例），并接入 GitHub Actions CI。

**Architecture:** Vitest 与 Vite 同源，零额外构建配置。devDependencies 仅新增 3 个：`vitest`、`jsdom`（DOM 模拟）、`@vitest/coverage-v8`（覆盖率）。**不引入** `@vue/test-utils`——首批不测 Vue 组件挂载。测试文件与源文件 colocate 在同目录，glob `**/*.{test,spec}.ts` 自动发现。CI 中 `frontend-test` 作为独立 job 与现有 Go Lint/Test/Build 并行。

**Tech Stack:** Vitest 3.x、jsdom 25.x、@vitest/coverage-v8 3.x。运行时依赖（vue 等）零变化。

**Spec:** `docs/superpowers/specs/2026-05-04-engineering-hygiene-trio-design.md` 第 5 节

---

## File Structure

| 文件 | 操作 | 责任 |
|---|---|---|
| `frontend/package.json` | 编辑 | 加 3 个 devDeps；加 `test` / `test:coverage` 脚本 |
| `frontend/package-lock.json` | 自动 | `npm install` 生成 |
| `frontend/vitest.config.ts` | 新建 | 最小配置：`environment: 'jsdom'`、glob `**/*.{test,spec}.ts` |
| `frontend/src/utils/eta.test.ts` | 新建 | 与 eta.ts 同目录 colocate；9 个测试用例 |
| `frontend/src/services/api.test.ts` | 新建 | 与 api.ts 同目录 colocate；3 个测试用例 |
| `.github/workflows/ci.yml` | 编辑 | 新增 `frontend-test` job，与 `lint` / `test` 并行 |

---

## Task 1: 创建 feature 分支

**Files:** 无（仅分支操作）

- [ ] **Step 1: 确认在 main 且工作树干净**

Run: `git status && git rev-parse --abbrev-ref HEAD`
Expected: `nothing to commit, working tree clean` 且分支名为 `main`。

- [ ] **Step 2: 同步远端 + 创建分支**

Run:
```bash
git pull --ff-only
git checkout -b feature/frontend-vitest
```

Expected: `Switched to a new branch 'feature/frontend-vitest'`

---

## Task 2: 安装 Vitest 基建（commit 1）

**Files:**
- Modify: `frontend/package.json`
- Auto: `frontend/package-lock.json`
- Create: `frontend/vitest.config.ts`

- [ ] **Step 1: 修改 `frontend/package.json` 加入测试脚本与 devDeps**

用 Edit 工具，old_string 为：

```json
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.5.13"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.3",
    "typescript": "~5.7.0",
    "vite": "^6.3.4",
    "vue-tsc": "^2.2.8"
  }
```

new_string 为：

```json
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview",
    "test": "vitest run --passWithNoTests",
    "test:watch": "vitest",
    "test:coverage": "vitest run --coverage"
  },
  "dependencies": {
    "vue": "^3.5.13"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.3",
    "@vitest/coverage-v8": "^3.0.0",
    "jsdom": "^25.0.0",
    "typescript": "~5.7.0",
    "vite": "^6.3.4",
    "vitest": "^3.0.0",
    "vue-tsc": "^2.2.8"
  }
```

- [ ] **Step 2: 创建 `frontend/vitest.config.ts`**

写入以下内容：

```typescript
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// 最小化 vitest 配置：复用 vite 的 vue 插件以便后续测组件，
// 现阶段仅需 jsdom 环境跑 utils/services 单测。
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/**/*.ts'],
      exclude: ['src/**/*.{test,spec}.ts', 'src/main.ts', 'src/env.d.ts'],
    },
  },
})
```

- [ ] **Step 3: 安装依赖**

Run: `cd frontend && npm install 2>&1 | tail -5`

Expected: 类似 `added N packages, and audited M packages in Xs` 的输出。
若出现 `peer dependency` 警告但无 `ERESOLVE` 错误，可忽略。

- [ ] **Step 4: 验证 vitest 可执行**

Run: `cd frontend && npx vitest --version`
Expected: 形如 `vitest/3.X.X` 的版本号。

- [ ] **Step 5: 验证 npm test 在无测试时通过**

Run: `cd frontend && npm test 2>&1 | tail -10`
Expected: vitest 报告"no test files found"但**退出码 0**（因为 `--passWithNoTests` 标志）。

如果退出码非 0：检查 `package.json` 的 `test` 脚本是否含 `--passWithNoTests`。

- [ ] **Step 6: 提交 commit 1**

Run:
```bash
git add frontend/package.json frontend/package-lock.json frontend/vitest.config.ts
git commit -m "chore: 引入 vitest 前端测试基建"
```

Expected: 一个新 commit，改动 3 个文件。

---

## Task 3: 写 ETA 测试（commit 2）

**Files:**
- Create: `frontend/src/utils/eta.test.ts`

`formatEta` 已在 main 中实现。本任务只是补回归测试，9 个用例覆盖全部分支。

- [ ] **Step 1: 创建 `frontend/src/utils/eta.test.ts`**

写入以下内容：

```typescript
import { describe, it, expect } from 'vitest';
import { formatEta, type ProgressSample } from './eta';

describe('formatEta', () => {
  it('returns "估算中..." when total is zero', () => {
    expect(formatEta([{ t: 0, current: 0 }, { t: 1000, current: 5 }], 0)).toBe('估算中...');
  });

  it('returns "估算中..." when samples array is empty', () => {
    expect(formatEta([], 100)).toBe('估算中...');
  });

  it('returns "估算中..." when only one sample is available', () => {
    expect(formatEta([{ t: 0, current: 0 }], 100)).toBe('估算中...');
  });

  it('returns "估算中..." when window has zero progress (dCur = 0)', () => {
    const samples: ProgressSample[] = [
      { t: 0, current: 5 },
      { t: 2000, current: 5 },
    ];
    expect(formatEta(samples, 100)).toBe('估算中...');
  });

  it('returns "即将完成" when current already reached total', () => {
    const samples: ProgressSample[] = [
      { t: 0, current: 50 },
      { t: 2000, current: 100 },
    ];
    expect(formatEta(samples, 100)).toBe('即将完成');
  });

  it('formats short ETAs in seconds', () => {
    // rate = (10 - 0)/(1000ms - 0ms) = 10/sec; remaining = 100 - 10 = 90 → 9 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 10 },
    ];
    expect(formatEta(samples, 100)).toBe('约 9 秒');
  });

  it('formats whole-minute ETAs without seconds', () => {
    // rate = 1/sec; remaining = 120 → 120 秒 = 2 分整
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 1 },
    ];
    expect(formatEta(samples, 121)).toBe('约 2 分');
  });

  it('formats minute+second ETAs', () => {
    // rate = 1/sec; remaining = 150 → 2 分 30 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 1 },
    ];
    expect(formatEta(samples, 151)).toBe('约 2 分 30 秒');
  });

  it('uses 8-second sliding window so old slow samples do not poison the rate', () => {
    // 早期 10 秒慢启动（0 → 10）；最近 0.5 秒高速（10 → 15）。
    // 滑窗截止 = 10500 - 8000 = 2500，丢弃 t=0 那条。
    // 窗内：first={t:10000, current:10}, last={t:10500, current:15}
    // dt = 0.5s, dCur = 5, rate = 10/sec
    // remaining = 100 - 15 = 85 → 9 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 10000, current: 10 },
      { t: 10500, current: 15 },
    ];
    expect(formatEta(samples, 100)).toBe('约 9 秒');
  });
});
```

- [ ] **Step 2: 跑测试**

Run: `cd frontend && npm test 2>&1 | tail -15`

Expected: `9 passed`，类似输出：
```
 ✓ src/utils/eta.test.ts (9)
   ✓ formatEta (9)
     ✓ returns "估算中..." when total is zero
     ...

 Test Files  1 passed (1)
      Tests  9 passed (9)
```

如有 FAIL：检查测试用例与 `eta.ts` 实际行为是否对得上（特别是用例 #9 的滑窗边界算术）。

- [ ] **Step 3: 提交 commit 2**

Run:
```bash
git add frontend/src/utils/eta.test.ts
git commit -m "test: 覆盖 eta 估算辅助函数"
```

---

## Task 4: 写 API 适配层测试（commit 3）

**Files:**
- Create: `frontend/src/services/api.test.ts`

3 个用例：模式检测（2）+ Web 模式取消（1）。

- [ ] **Step 1: 创建 `frontend/src/services/api.test.ts`**

写入以下内容：

```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';

// api.ts 含 module-level singleton（apiServiceInstance），所以每个测试前
// 都用 vi.resetModules() 让 dynamic import 拿到全新模块状态。

describe('mode detection (isDesktopMode / isWebMode)', () => {
  beforeEach(() => {
    vi.resetModules();
    delete (window as any).go;
  });

  it('returns desktop mode when window.go exists (Wails injected namespace)', async () => {
    (window as any).go = { app: { App: {} } };
    const { isDesktopMode, isWebMode } = await import('./api');
    expect(isDesktopMode()).toBe(true);
    expect(isWebMode()).toBe(false);
  });

  it('returns web mode when window.go is absent', async () => {
    const { isDesktopMode, isWebMode } = await import('./api');
    expect(isDesktopMode()).toBe(false);
    expect(isWebMode()).toBe(true);
  });
});

describe('WebAdapter cancellation flow', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    delete (window as any).go;
  });

  it('cancelExtraction() aborts the in-flight previewData fetch signal', async () => {
    // 捕获 fetch 拿到的 AbortSignal
    let captured: AbortSignal | undefined;
    const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
      captured = init?.signal ?? undefined;
      // 永远 pending —— 只能由 cancel 触发的 abort 中断
      return new Promise<Response>(() => undefined);
    });
    vi.stubGlobal('fetch', fetchMock);

    const { getApiService } = await import('./api');
    const service = getApiService();

    const file = new File([new Uint8Array([1, 2, 3])], 'sample.docx');
    // 不 await：让 fetch 进入 pending 状态
    const promise = service.previewData(file as any, ['defendant']);
    // 静默 reject（对 AbortError 在 WebAdapter 内已被吞为 success:false）
    promise.catch(() => undefined);

    // 让微任务跑完，确保 fetch 已被调用、signal 已被记录
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(captured).toBeDefined();
    expect(captured!.aborted).toBe(false);

    // 触发取消
    await service.cancelExtraction();

    expect(captured!.aborted).toBe(true);
  });
});
```

- [ ] **Step 2: 跑全部测试**

Run: `cd frontend && npm test 2>&1 | tail -20`

Expected: `12 passed`，2 个 test file（eta.test.ts + api.test.ts）。

如果 api 测试 FAIL：常见原因
- jsdom 没有原生 `File` 构造器 → vitest 3 + jsdom 25 默认应有；如缺失，添加 `import 'jsdom'` 显式（多半不需要）
- `vi.stubGlobal('fetch', ...)` 不生效 → 检查 `vitest.config.ts` 的 `environment` 是否真为 `jsdom`

- [ ] **Step 3: 提交 commit 3**

Run:
```bash
git add frontend/src/services/api.test.ts
git commit -m "test: 覆盖前端 api 适配层关键路径"
```

---

## Task 5: CI 集成（commit 4）

**Files:**
- Modify: `.github/workflows/ci.yml`

在现有 `lint` / `test` / `build` 三个 job 之后追加第四个 `frontend-test` job，与前两个并行（不阻塞 build，因为 build 仍 `needs: [lint, test]`）。

- [ ] **Step 1: 编辑 `.github/workflows/ci.yml`**

用 Edit 工具，old_string 为（文件最后一段，build job 整段）：

```yaml
  build:
    name: Build Check
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: "go.mod"
          cache: true

      - name: Create embedded config for compilation
        run: |
          mkdir -p internal/config
          echo 'baidu:
            token: ""
            api_url: ""' > internal/config/baked_conf.yaml

      - name: Verify build
        run: go build ./internal/... ./cmd/...
```

new_string 为（保留原 build job，紧接着追加 frontend-test job）：

```yaml
  build:
    name: Build Check
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: "go.mod"
          cache: true

      - name: Create embedded config for compilation
        run: |
          mkdir -p internal/config
          echo 'baidu:
            token: ""
            api_url: ""' > internal/config/baked_conf.yaml

      - name: Verify build
        run: go build ./internal/... ./cmd/...

  frontend-test:
    name: Frontend Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: "npm"
          cache-dependency-path: frontend/package-lock.json

      - name: Install
        working-directory: frontend
        run: npm ci

      - name: Run tests
        working-directory: frontend
        run: npm test
```

- [ ] **Step 2: 验证 YAML 合法**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo OK`
Expected: `OK`。

如果 python3 没装 yaml 模块：`pip3 install pyyaml --quiet 2>/dev/null && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo OK`。

- [ ] **Step 3: 验证新 job 已就位**

Run: `grep -E "^\s*\w+:.*$" .github/workflows/ci.yml | head -20`
Expected: 输出中能看到 `frontend-test:` 一行。

- [ ] **Step 4: 提交 commit 4**

Run:
```bash
git add .github/workflows/ci.yml
git commit -m "ci: 集成前端单元测试至持续集成"
```

---

## Task 6: 推送 + PR + 合并

**Files:** 无（仅 git 操作）

- [ ] **Step 1: 检查分支提交序列**

Run: `git log --oneline main..HEAD`
Expected: 4 个 commit，由新到旧依次为：
```
<hash> ci: 集成前端单元测试至持续集成
<hash> test: 覆盖前端 api 适配层关键路径
<hash> test: 覆盖 eta 估算辅助函数
<hash> chore: 引入 vitest 前端测试基建
```

- [ ] **Step 2: 推送**

Run: `git push -u origin feature/frontend-vitest`
Expected: 远端创建分支并 set upstream。

- [ ] **Step 3: 开 PR**

Run:
```bash
gh pr create --base main --head feature/frontend-vitest \
  --title "test: 引入前端单元测试基建并覆盖核心工具" \
  --body "$(cat <<'EOF'
## 概述

给 \`frontend/\` 引入 Vitest 测试框架，覆盖 ETA 估算函数与 API 适配层关键路径，并接入 GitHub Actions CI。前端从此告别"零测试"状态。

## 改动

| 文件 | 改动 |
|---|---|
| \`frontend/package.json\` | 加 3 个 devDeps（vitest / jsdom / @vitest/coverage-v8）；加 \`test\` / \`test:watch\` / \`test:coverage\` 脚本 |
| \`frontend/package-lock.json\` | npm install 自动更新 |
| \`frontend/vitest.config.ts\` | 新建：jsdom 环境、glob \`src/**/*.{test,spec}.ts\` |
| \`frontend/src/utils/eta.test.ts\` | 新建 9 用例覆盖 \`formatEta\` 全分支（估算中/即将完成/秒/分/分秒/滑窗）|
| \`frontend/src/services/api.test.ts\` | 新建 3 用例覆盖模式检测 + WebAdapter 取消流 |
| \`.github/workflows/ci.yml\` | 新增 \`frontend-test\` job 与 Go Lint/Test/Build 并行 |

## 设计抉择

- **不引入 \`@vue/test-utils\`**：首批仅测纯函数 + adapter 类，无 Vue 组件挂载需求；待后续真正测组件时再加
- **测试 colocate 于源文件目录**：vitest 默认友好，IDE 跳转便利
- **不设覆盖率门槛**：先产出报告，下一轮再加阈值

## 验证

- \`cd frontend && npm test\` 全 12 用例 PASS（9 eta + 3 api）
- \`cd frontend && npm run test:coverage\` 产出覆盖率报告
- CI 多一个独立 \`Frontend Test\` job 与 Go 链路并行
- Go 侧 4 项 CI 不受影响
EOF
)"
```

Expected: 输出 PR URL。

- [ ] **Step 4: 等 CI**

Run: `gh pr checks --watch`
Expected: Lint / Test / Build Check / GitGuardian Security / **Frontend Test** 五项全 `pass`。

如果 Frontend Test FAIL：拉日志看是 `npm ci` 失败（多半是 lock 不同步）还是测试 FAIL；按错误提示回 Task 2/3/4 修。

- [ ] **Step 5: Squash merge**

把 `<NUM>` 替换为上一步的 PR 编号。

Run:
```bash
gh pr merge <NUM> --squash --delete-branch \
  --subject "test: 引入前端单元测试基建并覆盖核心工具" \
  --body "给 frontend/ 引入 Vitest 与 jsdom，新增 9 个 ETA 单测 + 3 个 API 适配层单测，并在 CI 中加一个独立 frontend-test job 与现有 Go 链路并行。前端从零测试进入有回归网状态，为后续组件级测试铺路。"
```

Expected: PR 合入；远端分支自动删除；本地 main fast-forward。

- [ ] **Step 6: 同步本地**

Run:
```bash
git checkout main
git pull --ff-only
git fetch --prune
```

Expected: 本地 main 与 origin/main 一致；本地 `feature/frontend-vitest` 自动消失。

---

## Done Criteria

- [ ] PR 已通过 CI（含新 `Frontend Test` job）并合入 main
- [ ] `git log --oneline -3` 头部显示 `test: 引入前端单元测试基建并覆盖核心工具` 的 squash commit
- [ ] `cd frontend && npm test` 输出 `12 passed`
- [ ] `cd frontend && npm run test:coverage` 跑通并生成报告（不强制阈值）
- [ ] `.github/workflows/ci.yml` 含 `frontend-test:` job
- [ ] Go 侧 `go test ./... -race -count=1 && go vet ./... && go build ./...` 全干净（前端基建不影响后端）
- [ ] 4 个 commit 与 PR 标题均符合 `type: 中文描述` 规范
