# 组件级单测与覆盖率门槛 — 设计

- 状态：草案
- 日期：2026-05-05
- 作者：bobochang
- 目标版本：v3.4.0（前端测试基建第二期）

## 背景

v3.3.0（2026-05-04）落地了前端 Vitest 基建：9 用例覆盖 `formatEta`、3 用例覆盖 WebAdapter 取消流，CI 接入 `frontend-test` job。当时刻意未引入 `@vue/test-utils`，覆盖率也未设阈值，作为「先有测试再说」的最小起点。

本期目标：把前端测试从「零组件覆盖」推进到「中小组件有回归网 + 覆盖率门槛防回归」。

## 范围

### 本轮做

1. 引入 `@vue/test-utils` 作为前端组件测试框架。
2. 给 3 个中小组件补单测：`ResultCard.vue`（153 行）、`PreviewTable.vue`（243 行）、`MainDropZone.vue`（269 行）。
3. 在 `vitest.config.ts` 中设覆盖率门槛，分两档：
   - **纯函数模块档**（`src/utils/**/*.ts`、`src/services/**/*.ts`）：lines 80 / branches 70 / functions 80
   - **本轮覆盖的组件档**（3 个 `.vue` 文件）：lines 70 / branches 60 / functions 70
4. CI `frontend-test` job 改用 `npm run test:coverage`，让阈值不达标会让 CI 红。
5. `coverage.include` 增加 `src/**/*.vue`，让 v8 覆盖率把 SFC 的 `<script>` 段也算上。

### 本轮不做

- 不动 `App.vue`（1067 行巨石）。该组件依赖 Wails runtime、子组件、配置 store，硬测要 mock 半个世界，且测试本身也掩盖「该拆分」的事实。**留给独立一轮先做拆分再覆盖。**
- 不动 `ConfigPanel.vue`（684 行）。同理，体量过大，独立一轮处理。
- 不引入 `@testing-library/vue`。本项目没用 ARIA role 优先的 markup，`@vue/test-utils` 的 `find()` + `text()` 已足够，且与已有 vitest 风格一致。
- 不引入 `shallowMount`。3 个目标组件没有深嵌套，shallow 带来的隔离收益约等于零。
- 不上传前端覆盖率到 Codecov（避免与 Go 链路的 codecov 配置耦合，独立一轮处理）。
- 不写 E2E 集成测试（独立候选）。

## 设计

### 1. 测试框架与依赖

**新增 devDependency**：

| 包 | 版本 | 用途 |
|---|---|---|
| `@vue/test-utils` | `^2.4.0` | Vue 3 官方组件测试库 |

`vitest`、`jsdom`、`@vitejs/plugin-vue` 已有，无需变更。

### 2. 文件布局

测试文件 colocate 于源文件同目录，沿用 v3.3.0 约定：

```
frontend/src/
├── components/
│   ├── ResultCard.vue
│   ├── ResultCard.test.ts          ← 新增
│   ├── PreviewTable.vue
│   ├── PreviewTable.test.ts        ← 新增
│   ├── MainDropZone.vue
│   ├── MainDropZone.test.ts        ← 新增
│   ├── ConfigPanel.vue             （本轮不动）
├── test-utils/                     ← 新增目录
│   └── mockApi.ts                  ← 新增：集中 mock services 模块
├── utils/
│   ├── eta.ts
│   └── eta.test.ts                 （已有）
├── services/
│   ├── api.ts
│   └── api.test.ts                 （已有）
```

### 3. 共享测试 helper：`mockApi.ts`

3 个目标组件都从 `../services` 导入 `api`，需要 mock。集中到一个 helper 避免重复：

```ts
// frontend/src/test-utils/mockApi.ts
import { vi } from 'vitest'

export interface MockApi {
  isDesktop: boolean
  service: {
    openFile: ReturnType<typeof vi.fn>
    selectFile: ReturnType<typeof vi.fn>
    // 仅列出本轮 3 个目标组件实际使用的方法
  }
}

export function makeMockApi(overrides: Partial<MockApi> = {}): MockApi {
  return {
    isDesktop: false,                          // 默认 Web 模式，避开 Wails runtime 动态 import
    service: {
      openFile: vi.fn().mockResolvedValue(undefined),
      selectFile: vi.fn().mockResolvedValue(null),
      ...overrides.service,
    },
    ...overrides,
  }
}
```

测试文件用法：

```ts
import { makeMockApi } from '../test-utils/mockApi'

vi.mock('../services', () => ({
  api: makeMockApi(),
  // ExtractResult / Record 类型重导出（如组件用到）
}))
```

**设计决策**：不用 `vi.hoisted` 的全局 mock，而是各测试文件独立 `vi.mock`，因为不同组件需要不同 `api` 形态（如 `MainDropZone` 在某些用例里需要 `isDesktop: true`）。

### 4. 三个组件的测试范围

#### 4.1 `ResultCard.vue`（最小，先打样）

实际行为（来自源码）：
- props：`result: ExtractResult | null`
- emit：`notification(message, type)`
- 内部行为：`result === null` 不渲染；`result.success === true` 显示成功状态 + `recordCount` + 可点击的 `outputPath`；`result.success === false` 显示 `errorMessage`；点击 `outputPath` 调 `api.service.openFile`，失败 emit `notification("无法打开文件", "error")`

测试用例（约 5 个）：
1. `result === null` 时不渲染主容器（`wrapper.find('.result-card').exists()` === false）
2. 成功结果渲染「提取成功」标题 + `recordCount` 数字
3. 失败结果渲染 `errorMessage` + `error` class
4. 点击 `outputPath` 调用 `api.service.openFile(path)` 一次
5. `openFile` 抛错时 emit `notification` 含「无法打开文件」

#### 4.2 `PreviewTable.vue`

实际行为：
- props：`records: Record[]`、`fieldLabels: Record`
- 列由 `computed` 动态生成：`["defendant", "idNumber", "request", "factsReason"]` 中只保留实际出现在记录里的键，标签从 `fieldLabels` 取
- `request` / `factsReason` 渲染为 `<textarea>`，其他为 `<input>`，都 `v-model` 绑定到 `records[index][col.key]`

测试用例（约 5 个）：
1. `records: []` 时表头列数为 0（`columns` 为空）
2. records 含全部 4 个字段时，渲染 4 列表头，标签来自 `fieldLabels`
3. records 仅含 2 个字段时，仅渲染 2 列（按 orderedKeys 顺序，未出现的过滤）
4. `<textarea>` 渲染于 `request`/`factsReason` 列，`<input>` 渲染于其他列
5. 修改 input 的 value 后，`records[index][col.key]` 同步更新（v-model 双向绑定）

#### 4.3 `MainDropZone.vue`

实际行为：
- props：`selectedFile: string | File | null`、`fileName: string`
- emit：`update:selectedFile`、`notification`
- `displayPath`：string → 直接返回；File → 格式化字节数为 B/KB/MB
- 点击容器 → 调 `api.service.selectFile()`，结果非空则 emit
- Web drop（`api.isDesktop === false`）：从 `e.dataTransfer.files` 取第一个，扩展名校验，合法则 emit + 通知，非法则 emit error
- Desktop drop（`api.isDesktop === true`）：动态 import wailsjs runtime，本轮**不测此分支**（涉及动态 import + Wails 全局，性价比低；isDesktop=false 路径已能覆盖文件校验逻辑）

测试用例（约 6 个）：
1. `selectedFile === null` 时显示「点击或拖拽上传文件」文案
2. `selectedFile` 是 string 时，`displayPath` 等于该字符串
3. `selectedFile` 是 `File`（500 KB）时，`displayPath` 显示 `"500.0 KB"`
4. 点击容器调用 `api.service.selectFile()` 且返回值非空时 emit `update:selectedFile`
5. Web drop 合法 `.pdf` 文件时 emit `update:selectedFile` + `notification("文件已加载", "success")`
6. Web drop 非法 `.txt` 文件时 emit `notification("不支持的文件格式", "error")` 且不 emit `update:selectedFile`

**总计：约 16 个新增组件测试用例。**

### 5. 覆盖率门槛配置

`vitest.config.ts` 改动：

```ts
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/**/*.ts', 'src/**/*.vue'],          // ← 加 .vue
      exclude: [
        'src/**/*.{test,spec}.ts',
        'src/main.ts',
        'src/env.d.ts',
        'src/vite-env.d.ts',
        'src/test-utils/**',                              // ← 新增
      ],
      thresholds: {                                       // ← 新增
        'src/utils/**/*.ts': {
          lines: 80, branches: 70, functions: 80, statements: 80,
        },
        'src/services/**/*.ts': {
          lines: 80, branches: 70, functions: 80, statements: 80,
        },
        'src/components/ResultCard.vue': {
          lines: 70, branches: 60, functions: 70, statements: 70,
        },
        'src/components/PreviewTable.vue': {
          lines: 70, branches: 60, functions: 70, statements: 70,
        },
        'src/components/MainDropZone.vue': {
          lines: 70, branches: 60, functions: 70, statements: 70,
        },
      },
    },
  },
})
```

**设计决策**：
- 阈值采用「按文件 glob 设阈值」而非全局阈值——本轮不测的文件（`App.vue`、`ConfigPanel.vue`）默认不进门槛，避免出现「假装在保护一切」的伪门槛。
- 阈值定为「比预期实测略低 ~5 个百分点」的位置——本地跑一遍 `npm run test:coverage` 看实数后微调，目标是「现在 PASS、以后回退会红」。
- 纯函数档高于组件档：组件断言更脆（mock DOM/事件/props），高阈值会逼出过度断言。

### 6. CI 集成

`.github/workflows/ci.yml` 改动：

`frontend-test` job 中：
```yaml
- name: Run tests with coverage
  working-directory: frontend
  run: npm run test:coverage
```

替换原来的 `npm test`。

`Frontend Test` job 仍与 Go 三件套（Lint / Test / Build Check）并行，整体 CI 拓扑不变。

### 7. 错误处理与边界

- **Wails runtime 动态 import**：`MainDropZone` 在 `onMounted` 里 `await import("../../wailsjs/runtime/runtime")`，仅当 `api.isDesktop === true` 时执行。所有测试默认 `isDesktop: false`，绕开此路径。**不测此分支**写入 spec 是有意识的范围控制。
- **`api.service.openFile` 失败路径**：`ResultCard` 用例 5 显式覆盖。
- **覆盖率首次设阈稳健性**：在 plan 中规定「先用 `coverage.thresholdAutoUpdate: true` 跑一次让 vitest 自动写回阈值，看实测，去掉 autoUpdate 字段，把数字手动调到比实测低 ~5 个百分点」——避免阈值定得过高导致首次合入就红。若 vitest 当前版本无该字段，则手动跑 `--coverage` 看输出再凿死阈值。

### 8. 文件大小预估

| 文件 | 预估行数 |
|---|---|
| `frontend/src/test-utils/mockApi.ts` | ~30 |
| `frontend/src/components/ResultCard.test.ts` | ~80 |
| `frontend/src/components/PreviewTable.test.ts` | ~90 |
| `frontend/src/components/MainDropZone.test.ts` | ~110 |
| `frontend/vitest.config.ts` 改动 | +25 行 |
| `frontend/package.json` / `package-lock.json` | npm install 自动 |
| `.github/workflows/ci.yml` 改动 | 1 行 |

总新增 ~310 行测试代码 + ~25 行配置。

## 验证标准（Done Criteria）

- [ ] `cd frontend && npm test` 全 PASS（原 12 + 新增 ~16 共 ~28 用例）
- [ ] `cd frontend && npm run test:coverage` PASS 且阈值表生效不红
- [ ] CI 的 `Frontend Test` job 跑 `npm run test:coverage` 且 PASS
- [ ] `vitest.config.ts` 含 5 条阈值规则（utils / services / 3 个组件各一条）
- [ ] `App.vue` 与 `ConfigPanel.vue` 明确不在阈值表
- [ ] 3 个组件测试文件、`mockApi.ts` helper 全部 colocate / 在合适目录
- [ ] Go 侧 4 项 CI（Lint / Test / Build Check / GitGuardian）全绿
- [ ] PR commit 与 PR 标题符合 `type: 中文描述` 规范

## 后续候选（明确不在本轮）

1. `App.vue` 拆分（独立一轮，先拆再测）
2. `ConfigPanel.vue` 拆分（同上，可与 1 合并或单独）
3. macOS / Linux OCR 兜底（产品价值最高，工作量最大）
4. 多文件批量上传与队列
5. 端到端集成测试（Playwright / Cypress）
6. 删除 `frontend/yarn.lock`（10 分钟收尾活）
7. 前端覆盖率上传 Codecov（与 1/2 并行可做）
