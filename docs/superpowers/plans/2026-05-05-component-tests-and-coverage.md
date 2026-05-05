# 组件级单测与覆盖率门槛 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 3 个中小 Vue 组件（`ResultCard` / `PreviewTable` / `MainDropZone`）补单测，引入 `@vue/test-utils`，并在 vitest 设按文件 glob 的覆盖率门槛，CI 阈值生效。

**Architecture:** 引入 `@vue/test-utils` 作为组件测试库；测试 colocate 于源文件目录；用 `vi.mock('../services')` + 共享 `makeMockApi()` helper 统一处理 `api` 依赖；覆盖率门槛仅对已测文件设定（避免「假装在保护一切」）。

**Tech Stack:** Vue 3 + TypeScript + Vitest + jsdom + `@vue/test-utils` + v8 coverage。

---

## 前置假设

- 已在 `feature/component-tests-and-coverage` 分支
- 工作树干净（spec 已 commit）
- 当前目录：仓库根 `/Users/bobochang/Documents/legal-extractor`
- Node 20、npm 已就绪（CI 同款）

执行规范：本仓库 commit 严格 `type: 中文描述`；不带 scope、不带 issue 号、不带工具来源。

---

## Task 1：装 `@vue/test-utils`

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`（npm 自动）

- [ ] **Step 1：装包**

Run:
```bash
cd frontend && npm install --save-dev @vue/test-utils@^2.4.0
```

Expected：`devDependencies` 多出 `"@vue/test-utils": "^2.4.x"`，`package-lock.json` 同步更新。

- [ ] **Step 2：验证现有测试不受影响**

Run:
```bash
cd frontend && npm test
```

Expected：原 12 用例全 PASS（9 eta + 3 api）。

- [ ] **Step 3：commit**

Run:
```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore: 引入 vue test utils 作为组件测试库"
```

Expected：commit 成功。

---

## Task 2：写 `makeMockApi()` helper

**Files:**
- Create: `frontend/src/test-utils/mockApi.ts`

> 这一步暂不写 helper 的单测——它是测试基础设施，会在 Task 3-5 的组件测试里被实际使用并验证。

- [ ] **Step 1：先确认 services 真实导出形态**

Run:
```bash
cd frontend && grep -n "export" src/services/index.ts src/services/api.ts 2>/dev/null | head -20
```

Expected：能看到 `export const api`、`export type ExtractResult`、`export type Record` 等导出（确切名称以输出为准；后续 mock 要重导出这些类型）。

- [ ] **Step 2：创建 helper 文件**

写入 `frontend/src/test-utils/mockApi.ts`：

```ts
import { vi } from 'vitest'

/**
 * 测试用的 api mock 工厂。
 *
 * 3 个组件级单测都需要 mock `../services` 模块的 `api`，集中到这里避免重复。
 * 各测试文件在 `vi.mock('../services', ...)` 工厂里调用 `makeMockApi()`，
 * 用 `overrides` 覆盖默认行为。
 *
 * 默认值：
 *   - isDesktop: false（Web 模式，绕开 Wails runtime 动态 import）
 *   - service.openFile / selectFile：返回成功的 noop
 *
 * 仅声明 3 个目标组件实际用到的方法。如未来组件用到新方法，
 * 在 MockService 里追加即可。
 */
export interface MockService {
  openFile: ReturnType<typeof vi.fn>
  selectFile: ReturnType<typeof vi.fn>
}

export interface MockApi {
  isDesktop: boolean
  service: MockService
}

export interface MockApiOverrides {
  isDesktop?: boolean
  service?: Partial<MockService>
}

export function makeMockApi(overrides: MockApiOverrides = {}): MockApi {
  return {
    isDesktop: overrides.isDesktop ?? false,
    service: {
      openFile: vi.fn().mockResolvedValue(undefined),
      selectFile: vi.fn().mockResolvedValue(null),
      ...overrides.service,
    },
  }
}
```

- [ ] **Step 3：验证 TypeScript 通过**

Run:
```bash
cd frontend && npx vue-tsc --noEmit
```

Expected：无编译错误。

- [ ] **Step 4：验证现有测试不受影响**

Run:
```bash
cd frontend && npm test
```

Expected：原 12 用例仍 PASS（新文件未被任何测试导入，应该零影响）。

- [ ] **Step 5：commit**

Run:
```bash
git add frontend/src/test-utils/mockApi.ts
git commit -m "test: 新增 api mock 工厂以便组件单测复用"
```

Expected：commit 成功。

---

## Task 3：`ResultCard.test.ts`（5 用例，最小先打样）

**Files:**
- Create: `frontend/src/components/ResultCard.test.ts`
- Reference (read-only): `frontend/src/components/ResultCard.vue`

**TDD 流程**：先把 5 个用例全写成会 fail 的状态（实现一开始就在），再跑、再 fix（这里实现已经存在，所以 fail 应该来自「测试本身写错」或「mock 没装好」——但保持先 fail 再 pass 的节奏）。

**实际策略**：因为被测代码已存在，TDD 在此场景退化为「写测试 → 跑 → 应该 PASS」。仍然要在写完每个用例后跑一次，确认通过；不要写完一堆再跑。

- [ ] **Step 1：创建测试文件骨架（先用 1 个用例验证 mount + mock 通路）**

写入 `frontend/src/components/ResultCard.test.ts`：

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { makeMockApi } from '../test-utils/mockApi'
import ResultCard from './ResultCard.vue'

const mockApi = makeMockApi()

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  mockApi.service.openFile.mockClear()
  mockApi.service.openFile.mockResolvedValue(undefined)
})

describe('ResultCard', () => {
  it('result === null 时不渲染主容器', () => {
    const wrapper = mount(ResultCard, { props: { result: null } })
    expect(wrapper.find('.result-card').exists()).toBe(false)
  })
})
```

- [ ] **Step 2：跑这 1 个用例确认通路**

Run:
```bash
cd frontend && npx vitest run src/components/ResultCard.test.ts
```

Expected：1 passed。如果 FAIL，先排查（mount/mock 通路问题——例如类型导入、`.vue` 解析）。

- [ ] **Step 3：补用例 2（成功结果渲染）**

在同文件 `describe` 块内追加：

```ts
  it('成功结果渲染「提取成功」标题与 recordCount', () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 7,
          outputPath: '/tmp/out.xlsx',
          errorMessage: '',
        },
      },
    })
    expect(wrapper.find('.result-card').exists()).toBe(true)
    expect(wrapper.find('.result-card').classes()).not.toContain('error')
    expect(wrapper.text()).toContain('提取成功')
    expect(wrapper.text()).toContain('7')
  })
```

注意：`ExtractResult` 的具体字段以 `frontend/src/services/index.ts` 里的类型定义为准；如果实际字段是 `record_count` 或 `output_path` 等下划线命名，按真实命名调整这里和后续用例。**写之前先 grep 一次**：

Run:
```bash
cd frontend && grep -n "ExtractResult" src/services/*.ts
```

如有出入，按实际字段名修正所有用例的 `props.result` 字面量。

- [ ] **Step 4：跑测试**

Run:
```bash
cd frontend && npx vitest run src/components/ResultCard.test.ts
```

Expected：2 passed。

- [ ] **Step 5：补用例 3（失败结果）**

追加：

```ts
  it('失败结果渲染 errorMessage 与 error class', () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: false,
          recordCount: 0,
          outputPath: '',
          errorMessage: '解析失败：未识别到当事人',
        },
      },
    })
    expect(wrapper.find('.result-card').classes()).toContain('error')
    expect(wrapper.text()).toContain('提取失败')
    expect(wrapper.text()).toContain('解析失败：未识别到当事人')
  })
```

- [ ] **Step 6：跑测试**

Run: `cd frontend && npx vitest run src/components/ResultCard.test.ts`
Expected：3 passed。

- [ ] **Step 7：补用例 4（点击 outputPath 调 openFile）**

追加：

```ts
  it('点击 outputPath 调用 api.service.openFile 一次', async () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 1,
          outputPath: '/tmp/result.xlsx',
          errorMessage: '',
        },
      },
    })
    await wrapper.find('.clickable-path').trigger('click')
    expect(mockApi.service.openFile).toHaveBeenCalledTimes(1)
    expect(mockApi.service.openFile).toHaveBeenCalledWith('/tmp/result.xlsx')
  })
```

- [ ] **Step 8：跑测试**

Run: `cd frontend && npx vitest run src/components/ResultCard.test.ts`
Expected：4 passed。

- [ ] **Step 9：补用例 5（openFile 抛错时 emit notification）**

追加：

```ts
  it('openFile 抛错时 emit notification 含「无法打开文件」', async () => {
    mockApi.service.openFile.mockRejectedValueOnce(new Error('boom'))
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 1,
          outputPath: '/tmp/x.xlsx',
          errorMessage: '',
        },
      },
    })
    await wrapper.find('.clickable-path').trigger('click')
    // 等微任务队列冲刷（async handleOpenFile 里的 catch）
    await new Promise(resolve => setTimeout(resolve, 0))
    const events = wrapper.emitted('notification')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('无法打开文件')
    expect(events![0][1]).toBe('error')
  })
})
```

- [ ] **Step 10：跑测试**

Run: `cd frontend && npx vitest run src/components/ResultCard.test.ts`
Expected：5 passed。

如果 #5 FAIL（`emitted('notification')` 是 undefined），通常是异步 catch 还没执行：把 `await new Promise(resolve => setTimeout(resolve, 0))` 改为 `await wrapper.vm.$nextTick(); await new Promise(resolve => setTimeout(resolve, 0))`，再跑。

- [ ] **Step 11：跑全量测试确认无回归**

Run: `cd frontend && npm test`
Expected：17 passed（原 12 + 新 5）。

- [ ] **Step 12：commit**

```bash
git add frontend/src/components/ResultCard.test.ts
git commit -m "test: 覆盖 ResultCard 渲染与文件打开交互"
```

---

## Task 4：`PreviewTable.test.ts`（5 用例）

**Files:**
- Create: `frontend/src/components/PreviewTable.test.ts`
- Reference: `frontend/src/components/PreviewTable.vue`

> `PreviewTable` 不依赖 `api`，但仍统一用 `vi.mock` 占位以保持测试文件结构一致。

- [ ] **Step 1：先 grep 确认 Record 类型形态**

Run:
```bash
cd frontend && grep -n "type Record\|interface Record" src/services/*.ts
```

Expected：能看到 Record 类型（如 `Record = { [key: string]: string }` 或类似）。如果实际形态不同，下方用例的 fixture 字面量需要按真实类型调整。

- [ ] **Step 2：写测试文件骨架 + 用例 1（空数据列为 0）**

写入 `frontend/src/components/PreviewTable.test.ts`：

```ts
import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { makeMockApi } from '../test-utils/mockApi'
import PreviewTable from './PreviewTable.vue'

vi.mock('../services', () => ({
  api: makeMockApi(),
}))

const fieldLabels = {
  defendant: '被告',
  idNumber: '身份证号',
  request: '诉讼请求',
  factsReason: '事实与理由',
}

describe('PreviewTable', () => {
  it('records 为空时表头列数为 0', () => {
    const wrapper = mount(PreviewTable, {
      props: { records: [], fieldLabels },
    })
    expect(wrapper.findAll('thead th').length).toBe(0)
  })
})
```

- [ ] **Step 3：跑测试**

Run: `cd frontend && npx vitest run src/components/PreviewTable.test.ts`
Expected：1 passed。

- [ ] **Step 4：用例 2（4 字段全有，渲染 4 列且 label 正确）**

追加：

```ts
  it('records 含全部 4 个字段时渲染 4 列表头并使用 fieldLabels', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [
          {
            defendant: '张三',
            idNumber: '110000000000000000',
            request: '请求A',
            factsReason: '事实A',
          },
        ],
        fieldLabels,
      },
    })
    const headers = wrapper.findAll('thead th').map(th => th.text())
    expect(headers).toEqual(['被告', '身份证号', '诉讼请求', '事实与理由'])
  })
```

- [ ] **Step 5：跑测试**

Run: `cd frontend && npx vitest run src/components/PreviewTable.test.ts`
Expected：2 passed。

- [ ] **Step 6：用例 3（只含 2 字段，按 orderedKeys 顺序过滤）**

追加：

```ts
  it('records 仅含 defendant 与 request 时按 orderedKeys 顺序仅渲染 2 列', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [{ defendant: '李四', request: '请求B' }],
        fieldLabels,
      },
    })
    const headers = wrapper.findAll('thead th').map(th => th.text())
    expect(headers).toEqual(['被告', '诉讼请求'])
  })
```

- [ ] **Step 7：跑测试**

Run: `cd frontend && npx vitest run src/components/PreviewTable.test.ts`
Expected：3 passed。

- [ ] **Step 8：用例 4（textarea / input 按列类型分发）**

追加：

```ts
  it('request 与 factsReason 列渲染 textarea，其它列渲染 input', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [
          {
            defendant: '王五',
            idNumber: '110000000000000001',
            request: '请求C',
            factsReason: '事实C',
          },
        ],
        fieldLabels,
      },
    })
    const cells = wrapper.findAll('tbody td')
    // 4 列，单行，按列顺序：defendant(input) idNumber(input) request(textarea) factsReason(textarea)
    expect(cells[0].find('input').exists()).toBe(true)
    expect(cells[0].find('textarea').exists()).toBe(false)
    expect(cells[1].find('input').exists()).toBe(true)
    expect(cells[2].find('textarea').exists()).toBe(true)
    expect(cells[2].find('input').exists()).toBe(false)
    expect(cells[3].find('textarea').exists()).toBe(true)
  })
```

- [ ] **Step 9：跑测试**

Run: `cd frontend && npx vitest run src/components/PreviewTable.test.ts`
Expected：4 passed。

- [ ] **Step 10：用例 5（v-model 双向绑定）**

追加：

```ts
  it('修改 input 的 value 后同步回 records[index][col.key]', async () => {
    const records = [
      {
        defendant: '初值',
        idNumber: '110000000000000002',
        request: 'r',
        factsReason: 'f',
      },
    ]
    const wrapper = mount(PreviewTable, {
      props: { records, fieldLabels },
    })
    const firstInput = wrapper.find('tbody td input')
    await firstInput.setValue('改后值')
    expect(records[0].defendant).toBe('改后值')
  })
})
```

- [ ] **Step 11：跑测试**

Run: `cd frontend && npx vitest run src/components/PreviewTable.test.ts`
Expected：5 passed。

- [ ] **Step 12：跑全量确认无回归**

Run: `cd frontend && npm test`
Expected：22 passed（原 12 + ResultCard 5 + PreviewTable 5）。

- [ ] **Step 13：commit**

```bash
git add frontend/src/components/PreviewTable.test.ts
git commit -m "test: 覆盖 PreviewTable 列生成与可编辑单元格"
```

---

## Task 5：`MainDropZone.test.ts`（6 用例）

**Files:**
- Create: `frontend/src/components/MainDropZone.test.ts`
- Reference: `frontend/src/components/MainDropZone.vue`

> 所有用例默认 `isDesktop: false`。**不测**桌面端 Wails runtime 动态 import 分支。

- [ ] **Step 1：写骨架 + 用例 1（默认文案）**

写入 `frontend/src/components/MainDropZone.test.ts`：

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { makeMockApi } from '../test-utils/mockApi'
import MainDropZone from './MainDropZone.vue'

const mockApi = makeMockApi() // isDesktop: false

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  mockApi.service.selectFile.mockClear()
  mockApi.service.selectFile.mockResolvedValue(null)
})

describe('MainDropZone', () => {
  it('selectedFile 为 null 时显示「点击或拖拽上传文件」文案', () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    expect(wrapper.text()).toContain('点击或拖拽上传文件')
  })
})
```

- [ ] **Step 2：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：1 passed。

- [ ] **Step 3：用例 2（string 路径直接显示）**

追加：

```ts
  it('selectedFile 是字符串时 file-path-text 显示该字符串', () => {
    const wrapper = mount(MainDropZone, {
      props: {
        selectedFile: '/Users/foo/案件.docx',
        fileName: '案件.docx',
      },
    })
    expect(wrapper.find('.file-path-text').text()).toBe('/Users/foo/案件.docx')
    expect(wrapper.find('.file-name-display').text()).toBe('案件.docx')
  })
```

- [ ] **Step 4：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：2 passed。

- [ ] **Step 5：用例 3（File 500KB 显示 "500.0 KB"）**

追加：

```ts
  it('selectedFile 是 500KB 的 File 时 displayPath 显示 "500.0 KB"', () => {
    // 注意：jsdom 的 File 构造器接受字节数组；用一个长度为 500*1024 的填充字符串。
    const blob = new Blob(['x'.repeat(500 * 1024)])
    const file = new File([blob], '案件.pdf', { type: 'application/pdf' })
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: file, fileName: '案件.pdf' },
    })
    expect(wrapper.find('.file-path-text').text()).toBe('500.0 KB')
  })
```

- [ ] **Step 6：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：3 passed。

如 FAIL 因 jsdom Blob/File 行为差异（如 `file.size` 不准），改用 `Object.defineProperty(file, 'size', { value: 500 * 1024 })` 强制设 size。

- [ ] **Step 7：用例 4（点击容器调 selectFile + emit）**

追加：

```ts
  it('点击容器调用 selectFile 且非空返回时 emit update:selectedFile', async () => {
    mockApi.service.selectFile.mockResolvedValueOnce('/tmp/picked.pdf')
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    await wrapper.find('.drop-zone').trigger('click')
    // 等 await api.service.selectFile() 与后续 emit 完成
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(mockApi.service.selectFile).toHaveBeenCalledTimes(1)
    const events = wrapper.emitted('update:selectedFile')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('/tmp/picked.pdf')
  })
```

- [ ] **Step 8：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：4 passed。

- [ ] **Step 9：用例 5（合法 .pdf drop → emit + success notification）**

追加：

```ts
  it('Web drop 合法 .pdf 文件时 emit update:selectedFile 与 success notification', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    const file = new File(['data'], '案件.pdf', { type: 'application/pdf' })
    await wrapper.find('.drop-zone').trigger('drop', {
      dataTransfer: { files: [file] },
    })
    const updateEvents = wrapper.emitted('update:selectedFile')
    expect(updateEvents).toBeTruthy()
    expect(updateEvents![0][0]).toBe(file)
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('文件已加载')
    expect(notif![0][1]).toBe('success')
  })
```

- [ ] **Step 10：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：5 passed。

注意：`@vue/test-utils` 的 `trigger('drop', { dataTransfer: ... })` 在某些版本里 `dataTransfer` 不会自动挂到 event 上。如 FAIL，改写为：

```ts
const dropEvent = new Event('drop', { bubbles: true, cancelable: true })
Object.defineProperty(dropEvent, 'dataTransfer', { value: { files: [file] } })
wrapper.find('.drop-zone').element.dispatchEvent(dropEvent)
await wrapper.vm.$nextTick()
```

- [ ] **Step 11：用例 6（非法 .txt drop → 仅 error notification）**

追加：

```ts
  it('Web drop 非法 .txt 文件时 emit error notification 且不 emit update:selectedFile', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    const file = new File(['data'], 'note.txt', { type: 'text/plain' })
    await wrapper.find('.drop-zone').trigger('drop', {
      dataTransfer: { files: [file] },
    })
    expect(wrapper.emitted('update:selectedFile')).toBeFalsy()
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('不支持的文件格式')
    expect(notif![0][1]).toBe('error')
  })
})
```

- [ ] **Step 12：跑测试**

Run: `cd frontend && npx vitest run src/components/MainDropZone.test.ts`
Expected：6 passed。

- [ ] **Step 13：跑全量确认无回归**

Run: `cd frontend && npm test`
Expected：28 passed（原 12 + 16 新增）。

- [ ] **Step 14：commit**

```bash
git add frontend/src/components/MainDropZone.test.ts
git commit -m "test: 覆盖 MainDropZone 显示路径与拖拽校验"
```

---

## Task 6：`vitest.config.ts` 设覆盖率门槛 + 本地校准

**Files:**
- Modify: `frontend/vitest.config.ts`

策略：先用一个**临时极低**的全局占位阈值跑出实测数字 → 看实数 → 把数字凿死（实测 -5pp，整数）。

- [ ] **Step 1：先看本地实测覆盖率**

Run:
```bash
cd frontend && npm run test:coverage
```

Expected：输出文件级覆盖率表（v8 reporter "text"）。**记录下** 5 个目标文件的 lines / branches / functions / statements 实测值：

- `src/utils/eta.ts`
- `src/services/api.ts`
- `src/components/ResultCard.vue`
- `src/components/PreviewTable.vue`
- `src/components/MainDropZone.vue`

- [ ] **Step 2：写 vitest 配置**

替换 `frontend/vitest.config.ts` 全文为：

```ts
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// vitest 配置：jsdom + v8 覆盖率 + 按文件 glob 的覆盖率门槛。
//
// 阈值仅设给已写测试的文件——本轮不测的 App.vue / ConfigPanel.vue
// 不在阈值表，避免「假装在保护一切」。后续轮次补测哪个文件，
// 再把它加进 thresholds。
//
// 阈值数字策略：取「实测值 -5pp 取整」，作为防回归的现实门槛。
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/**/*.ts', 'src/**/*.vue'],
      exclude: [
        'src/**/*.{test,spec}.ts',
        'src/main.ts',
        'src/vite-env.d.ts',
        'src/test-utils/**',
      ],
      thresholds: {
        'src/utils/**/*.ts': {
          lines: 80, branches: 70, functions: 80, statements: 80,
        },
        'src/services/**/*.ts': {
          lines: 70, branches: 60, functions: 70, statements: 70,
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

> 注：`services/api.ts` 阈值给得比 `utils/eta.ts` 低 10pp，因为上一轮 api 仅 3 用例覆盖核心路径，实测可能 60-70%。如果 Step 1 实测数字与默认值差距过大，按下表调整：

| 实测 | 设阈值 |
|---|---|
| ≥ 95% | 90 |
| 85-95% | 80 |
| 75-85% | 70 |
| 65-75% | 60 |
| < 65% | 实测 -5pp 向下取整到 5 的倍数 |

- [ ] **Step 3：跑覆盖率验证阈值通过**

Run:
```bash
cd frontend && npm run test:coverage
```

Expected：所有 5 条 thresholds rule 都 PASS。如果某一条 FAIL（写着 `ERROR: Coverage for lines (XX%) does not meet threshold (YY%) ...`），把对应阈值下调 5pp 再跑。

- [ ] **Step 4：跑普通 `npm test` 确认非覆盖率运行不变**

Run: `cd frontend && npm test`
Expected：28 passed（与 Task 5 末尾一致；阈值不影响非 coverage 模式）。

- [ ] **Step 5：vue-tsc 编译验证**

Run: `cd frontend && npx vue-tsc --noEmit`
Expected：无编译错误。

- [ ] **Step 6：commit**

```bash
git add frontend/vitest.config.ts
git commit -m "test: 为已覆盖文件设定按文件覆盖率门槛"
```

---

## Task 7：CI 改用 `npm run test:coverage`

**Files:**
- Modify: `.github/workflows/ci.yml:103-105`

- [ ] **Step 1：编辑 CI**

把 `.github/workflows/ci.yml` 中 `frontend-test` job 的最后一步：

```yaml
      - name: Run tests
        working-directory: frontend
        run: npm test
```

改为：

```yaml
      - name: Run tests with coverage
        working-directory: frontend
        run: npm run test:coverage
```

- [ ] **Step 2：本地预演 CI 命令**

Run:
```bash
cd frontend && npm ci && npm run test:coverage
```

Expected：`npm ci` 干净安装、`test:coverage` 阈值 PASS。如失败，调整 `vitest.config.ts` 阈值。

> 用 `npm ci` 而非 `npm install` 是为了模拟 CI 环境——它要求 `package-lock.json` 与 `package.json` 严格一致。如 `npm ci` 报 `EUSAGE`，说明 lock 有偏差，回到 Task 1/2 检查包安装是否完整。

- [ ] **Step 3：commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: 前端测试改用覆盖率模式以执行阈值"
```

---

## Task 8：开 PR + CI + Squash Merge

> 这一步需要远端推送与 GitHub 操作。如果用户不希望此 agent 创建 PR，停下来询问；否则按下面执行。

- [ ] **Step 1：查看本分支总览**

Run:
```bash
git log --oneline main..HEAD
```

Expected：~7 个 commit（spec + Task 1-7 各一），按时间顺序。

- [ ] **Step 2：推送分支**

Run:
```bash
git push -u origin feature/component-tests-and-coverage
```

Expected：远端创建分支并跟踪。

- [ ] **Step 3：开 PR**

Run:
```bash
gh pr create --base main --head feature/component-tests-and-coverage \
  --title "test: 引入组件级单测与覆盖率门槛" \
  --body "$(cat <<'EOF'
## 概述

把前端测试从「零组件覆盖」推进到「3 个中小组件有回归网 + 覆盖率门槛防回归」。引入 \`@vue/test-utils\`，给 \`ResultCard\` / \`PreviewTable\` / \`MainDropZone\` 共 16 个组件级用例，并在 vitest 配置按文件 glob 设阈值。CI 的 \`Frontend Test\` job 改用 \`npm run test:coverage\`，阈值不达标会让 CI 红。

\`App.vue\`（1067 行）与 \`ConfigPanel.vue\`（684 行）作为巨石本轮明确不动——留给独立一轮先做拆分再覆盖。

## 改动清单

| 文件 | 改动 |
|---|---|
| \`frontend/package.json\` / \`package-lock.json\` | 加 \`@vue/test-utils\` devDep |
| \`frontend/src/test-utils/mockApi.ts\` | 新建：\`makeMockApi()\` 工厂集中 mock services 模块 |
| \`frontend/src/components/ResultCard.test.ts\` | 新建：5 用例（渲染/失败态/打开文件/异常） |
| \`frontend/src/components/PreviewTable.test.ts\` | 新建：5 用例（列生成/v-model/表单类型分发） |
| \`frontend/src/components/MainDropZone.test.ts\` | 新建：6 用例（显示路径/点击选择/拖拽合法非法分支） |
| \`frontend/vitest.config.ts\` | 加 \`coverage.thresholds\`（5 条 rule）；\`include\` 加 \`.vue\`；exclude 修 \`env.d.ts\` → \`vite-env.d.ts\`、加 \`test-utils/\` |
| \`.github/workflows/ci.yml\` | \`frontend-test\` job 跑 \`npm run test:coverage\` |

## 设计抉择

- **测 3 个不测 5 个**：\`App.vue\` / \`ConfigPanel.vue\` 体量过大，硬测会变成「测 mock」而非「测行为」。
- **\`@vue/test-utils\` 而非 \`@testing-library/vue\`**：与现有 vitest 风格一致，本项目 markup 没有 ARIA-role 优先，testing-library 红利打折。
- **按文件 glob 阈值而非全局**：未测文件不进门槛，避免「假装在保护一切」。本轮覆盖的 5 个文件凿死阈值。
- **\`mockApi\` helper 集中 mock**：3 个组件都依赖 \`api\`，集中工厂避免重复样板。
- **不测桌面端 Wails drop 分支**：动态 import + Wails runtime 全局，性价比低；Web 模式分支已能覆盖文件校验逻辑。

## 验证

- \`cd frontend && npm test\`：28 passed（原 12 + 新 16）
- \`cd frontend && npm run test:coverage\`：5 条 thresholds 全 PASS
- \`cd frontend && npx vue-tsc --noEmit\`：clean
- CI 5 项（Lint / Test / Build Check / GitGuardian / Frontend Test）应全绿

## 后续候选（明确不在本 PR）

1. \`App.vue\` 拆分（独立一轮，先拆再测）
2. \`ConfigPanel.vue\` 拆分
3. macOS / Linux OCR 兜底
4. 多文件批量上传与队列
5. E2E 集成测试
6. 删除 \`frontend/yarn.lock\`
7. 前端覆盖率上传 Codecov
EOF
)"
```

Expected：输出 PR URL。

- [ ] **Step 4：等 CI**

Run: `gh pr checks --watch`
Expected：5 项全 `pass`：Lint / Test / Build Check / GitGuardian Security / Frontend Test。

如 `Frontend Test` FAIL：拉日志看是 npm ci 失败（lock 偏差）还是阈值不达标；按错误信息回到 Task 6 调阈值。

- [ ] **Step 5：Squash merge**

Run（把 `<NUM>` 替换为上一步的 PR 编号）：
```bash
gh pr merge <NUM> --squash --delete-branch \
  --subject "test: 引入组件级单测与覆盖率门槛" \
  --body "把前端测试从零组件覆盖推进到 3 个中小组件有回归网。引入 @vue/test-utils 与 16 个组件级用例，覆盖 ResultCard / PreviewTable / MainDropZone。vitest 接入按文件 glob 的覆盖率门槛（5 条 rule，已覆盖文件凿死阈值），CI frontend-test 跑覆盖率模式。App.vue / ConfigPanel.vue 巨石作为独立一轮的候选。"
```

Expected：PR 合入、远端分支自动删除、本地 main fast-forward 后能拉到。

- [ ] **Step 6：同步本地**

Run:
```bash
git checkout main
git pull --ff-only
git fetch --prune
```

Expected：本地 main 与 origin/main 一致；本地 \`feature/component-tests-and-coverage\` 自动消失（因为 \`--delete-branch\` + \`--prune\`）。如果本地分支仍在，手动删：\`git branch -D feature/component-tests-and-coverage\`。

---

## Done Criteria

- [ ] PR 已通过 CI（5 项全绿）并合入 main
- [ ] `git log --oneline -3` 头部显示 squash commit `test: 引入组件级单测与覆盖率门槛`
- [ ] `cd frontend && npm test` 输出 `28 passed`
- [ ] `cd frontend && npm run test:coverage` PASS 且 5 条 thresholds rule 全过
- [ ] `vitest.config.ts` 含 5 条阈值规则；`App.vue` / `ConfigPanel.vue` 不在阈值表
- [ ] CI `frontend-test` 步骤跑 `npm run test:coverage`
- [ ] Go 侧 4 项 CI 全干净（前端改动不影响后端）
- [ ] 全部 commit 与 PR 标题符合 `type: 中文描述` 规范

---

## 异常处置预案

| 症状 | 排查 | 处置 |
|---|---|---|
| `vi.mock('../services')` 后类型缺失（如 `ExtractResult`） | mock 工厂没重导出类型 | 在 `vi.mock` 工厂里加 `__esModule: true` 与类型 stub；或测试文件中改 import 为 `import type` |
| `mount(Component)` 报 SFC 解析失败 | `vitest.config.ts` 没装 `@vitejs/plugin-vue` | 已装，确认 `plugins: [vue()]` 还在 |
| `@vue/test-utils` 类型与 Vue 3 不匹配 | 装的是 1.x（Vue 2 版） | 确认 `package.json` 是 `@vue/test-utils@^2.4.0` |
| `npm run test:coverage` 阈值红但本地无回归 | 实测低于设定阈值 | 按 Task 6 表格下调阈值；首次合入优先「先 PASS 再调高」 |
| `dataTransfer` 未挂到 drop 事件 | `@vue/test-utils` 版本差异 | 改用 `dispatchEvent` + `Object.defineProperty` 方式（见 Task 5 Step 10 注） |
| File `size` 与字节数组长度不一致 | jsdom 实现差异 | `Object.defineProperty(file, 'size', { value: ... })` 强设 |
