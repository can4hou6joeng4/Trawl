# 工程化收尾：删除 yarn.lock 与 Codecov 双 flag 接入 — 设计

- 状态：草案
- 日期：2026-05-06
- 作者：bobochang
- 目标版本：v3.5.0

## 背景

v3.4.0（2026-05-05）落地了组件级单测与覆盖率门槛，前端测试基建步入「有回归网」状态。本轮收尾两件长期遗留小事：

1. **删除 `frontend/yarn.lock`**：v2.x 时期短暂用过 yarn，自 v3.x 起仓库完全切到 npm（`Dockerfile` / `wails.json` / `.github/workflows/ci.yml` 均用 `npm`，`package-lock.json` 是唯一权威 lock）。`yarn.lock` 自此沦为孤儿——但 `npm install` 在某些版本下会顺手更新它，导致每次本地装包都产生无意义的 git diff（v3.4.0 会话期间反复出现）。
2. **前端覆盖率上传 Codecov**：Go 链路自 v3.0 起通过 `codecov/codecov-action@v5` 上传至 codecov.io，README 已挂 codecov badge。前端在 v3.4.0 拿到了覆盖率数字但未接入，dashboard 半暗。

本轮目标：把这两个工程化债务一次性清掉，得到「单 lock 文件 + 双轨覆盖率 dashboard」的干净状态。

## 范围

### 本轮做

1. `git rm frontend/yarn.lock`（15KB 死文件）。
2. `frontend/vitest.config.ts` 的 `coverage.reporter` 加 `'lcov'`，让 `npm run test:coverage` 同时产出 `frontend/coverage/lcov.info`。
3. `.github/workflows/ci.yml` 的 `frontend-test` job 末尾加 codecov upload step（push 到 main 才触发，与 Go 链路条件一致）。
4. 给 Go test job 与 frontend-test job 的两个 codecov upload step 各加 `flags`（分别 `backend` / `frontend`）。
5. 新建 `codecov.yml`，定义两个 flag 的 carryforward 行为，避免某次单边 upload 让另一边数据"消失"。

### 本轮不做

- 不改 `coverage.thresholds`（vitest 阈值已在 v3.4.0 凿死，是防回归的一层；Codecov 是观测用，不再加 PR 阻塞策略）。
- 不动 README badge（现有总分 badge 仍有效；分 flag 数据通过 dashboard 看，不需要在 README 显示两个 badge）。
- 不接入 codecov 的 PR comment / status check（额外配置量大，价值低；vitest 阈值已经在 PR 层把控回归）。
- 不上传 PR 触发的 coverage（与 Go 链路保持一致：仅 `push && refs/heads/main` 时 upload，避免 PR 草稿数据污染主线趋势）。

## 设计

### 1. `vitest.config.ts` 改动

`coverage.reporter` 由 `['text', 'html']` 改为 `['text', 'html', 'lcov']`。

```ts
coverage: {
  provider: 'v8',
  reporter: ['text', 'html', 'lcov'],
  // 其余不变
}
```

效果：
- 终端继续打印 text 表（开发体验不变）
- 仍生成 `coverage/index.html`（本地浏览器查看用）
- 新生成 `coverage/lcov.info`（标准 lcov 格式，Codecov 直接吃）

`coverage/` 目录已在 `.gitignore`（v3.4.0 加），lcov.info 也在内。

### 2. `frontend/yarn.lock` 删除

`git rm frontend/yarn.lock` 即可。无需在 `.gitignore` 列入，因为：
- 项目方针是 npm 单源，CI/Dockerfile/wails.json 都不调 yarn
- 现代 npm（≥7）默认不会创建 yarn.lock；旧 npm 才会"保留"既存 yarn.lock
- 删后再 `npm install` 不会重生（v3.4.0 会话验证：lock 之所以被改动是它"已经存在"才被 sync；首次安装时 npm 不会主动建）

### 3. `codecov.yml` 新建（仓库根）

```yaml
coverage:
  status:
    project:
      default:
        target: auto
        threshold: 1%
      backend:
        flags:
          - backend
        target: auto
        threshold: 1%
      frontend:
        flags:
          - frontend
        target: auto
        threshold: 1%

flags:
  backend:
    paths:
      - internal/
      - cmd/
    carryforward: true
  frontend:
    paths:
      - frontend/src/
    carryforward: true
```

**设计决策**：

- **`carryforward: true`**：当某次 push 只上传单边覆盖率（实际不会发生，但保险起见），dashboard 用上次同 flag 数据填充缺失值，避免另一侧数据"归零"假象。
- **`status.project.{backend,frontend}`**：让 codecov 在 PR 评论/状态里分别显示两条覆盖率线，target=auto 即与 base 比对，不强制阈值（差异 1pp 容忍）。
- **paths 限定**：让 codecov 知道某 flag 只覆盖某些目录，避免覆盖率被"非该 flag 的 0%"目录稀释。
- **不写 `comment:` 段**：用 codecov 默认行为（PR 自动评论，不爆炸性输出），避免与项目方针冲突。

### 4. `.github/workflows/ci.yml` 改动

#### 4.1 `test` job（Go）— 加 flags

```yaml
- name: Upload coverage to Codecov
  if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  uses: codecov/codecov-action@v5
  with:
    files: coverage.out
    flags: backend
    fail_ci_if_error: false
```

仅加 1 行 `flags: backend`。其它字段不动。

#### 4.2 `frontend-test` job — 新增 upload step

在 `Run tests with coverage` 之后：

```yaml
- name: Upload frontend coverage to Codecov
  if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  uses: codecov/codecov-action@v5
  with:
    files: frontend/coverage/lcov.info
    flags: frontend
    fail_ci_if_error: false
```

触发条件、`fail_ci_if_error` 与 Go 链路保持一致。

### 5. 数据流

```
Push to main
    │
    ├─ test job (Go)
    │   └─ go test -coverprofile=coverage.out
    │   └─ codecov/codecov-action@v5 (flags: backend)
    │
    └─ frontend-test job
        └─ npm run test:coverage  → frontend/coverage/lcov.info
        └─ codecov/codecov-action@v5 (flags: frontend)

Codecov.io
    ├─ flag: backend  → internal/ + cmd/
    ├─ flag: frontend → frontend/src/
    └─ project status comment on PR (auto target, ±1pp threshold)
```

### 6. 错误处理与边界

- **CI 中 codecov upload 失败不挂掉 CI**：两处 upload step 都设 `fail_ci_if_error: false`，与 Go 链路现有方针一致。Codecov 是观测，不应阻塞合入。
- **PR 不上传**：`if` 条件保证仅 push to main 触发；PR 评论由 codecov 在 base/head 比对时生成（依赖 main 历史，PR 本身无需上传）。
- **token 配置**：仓库已配置 `CODECOV_TOKEN` secret（v3.0 时期 Go 上传开始用），`codecov-action@v5` 自动读取，无需新增 secret。**验证方式**：现有 Go upload 成功即说明 token 已就位。
- **首次 dashboard 展示**：合入后第一次 push 触发上传，约 1-2 分钟后在 codecov.io 看到 `frontend` flag 与对应 covered files 列表。

### 7. 文件大小预估

| 文件 | 改动 |
|---|---|
| `frontend/yarn.lock` | 删除（-407 行）|
| `frontend/vitest.config.ts` | +1 字符（reporter 数组多 `'lcov'`）|
| `codecov.yml` | 新建（~22 行）|
| `.github/workflows/ci.yml` | +1 行（Go flags）+8 行（frontend upload）|

总计 ~30 行新增，~407 行删除，净减小。

## 验证标准（Done Criteria）

- [ ] `frontend/yarn.lock` 不再存在于工作树或 git index
- [ ] `cd frontend && npm run test:coverage` 产出 `frontend/coverage/lcov.info`（>0 字节）
- [ ] `codecov.yml` 通过 `https://codecov.io/validate` 校验（合入后可用 `curl -X POST --data-binary @codecov.yml https://codecov.io/validate` 验证）
- [ ] CI 4 项 Go + 1 项 frontend-test 全绿（codecov upload step 失败不算红，但合入后应在 dashboard 看到双 flag）
- [ ] dashboard 在合入后 5 分钟内显示 `backend` 与 `frontend` 两个 flag 的覆盖率数字
- [ ] PR commit 与 PR 标题符合 `type: 中文描述` 规范

## 后续候选（明确不在本轮）

1. `App.vue` 拆分（独立一轮，先拆再测）
2. `ConfigPanel.vue` 拆分
3. macOS / Linux OCR 兜底
4. 多文件批量上传与队列
5. E2E 集成测试
6. 给 codecov.yml 加 PR comment 自定义 + status check 阻塞策略（需要再观察 1-2 周看现有"自动 target"是否够用）
