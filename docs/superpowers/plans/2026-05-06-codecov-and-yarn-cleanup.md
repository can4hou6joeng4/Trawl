# 工程化收尾：删除 yarn.lock 与 Codecov 双 flag 接入 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除已无用的 `frontend/yarn.lock`；前端 vitest 覆盖率额外产出 lcov；CI 把前后端覆盖率以双 flag 推送到 Codecov。

**Architecture:** 三个独立小改动各一 commit。CI 改动复用现有 `codecov/codecov-action@v5` 与 `CODECOV_TOKEN` secret，仅加 `flags` 字段；新建仓库根 `codecov.yml` 定义 flag/path 映射；vitest reporter 数组多加 `'lcov'` 一项。

**Tech Stack:** vitest v8 coverage、`codecov/codecov-action@v5`、Codecov flag system。

---

## 前置假设

- 已在 `feature/codecov-and-yarn-cleanup` 分支
- 工作树干净（spec 已 commit）
- 当前目录：仓库根 `/Users/bobochang/Documents/legal-extractor`
- 仓库已配置 `CODECOV_TOKEN` secret（v3.0 起 Go 上传一直用，无需新增）

执行规范：本仓库 commit 严格 `type: 中文描述`；不带 scope、不带 issue 号、不带工具来源标识。

---

## Task 1：删除 `frontend/yarn.lock`

**Files:**
- Delete: `frontend/yarn.lock`

- [ ] **Step 1：确认无 yarn 引用**

Run:
```bash
grep -rn "yarn" /Users/bobochang/Documents/legal-extractor/.github/ /Users/bobochang/Documents/legal-extractor/Dockerfile /Users/bobochang/Documents/legal-extractor/wails.json /Users/bobochang/Documents/legal-extractor/build.sh 2>/dev/null
```

Expected：无任何输出（或仅出现在测试 fixture / spec 文档里的字符串，不在脚本里）。如果发现脚本里有 `yarn install` 等真实调用，停下来报告，不要继续。

- [ ] **Step 2：删除文件**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor && git rm frontend/yarn.lock
```

Expected：`rm 'frontend/yarn.lock'`。

- [ ] **Step 3：验证 npm 链路不受影响**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor/frontend && npm ci 2>&1 | tail -5
```

Expected：`added N packages` 之类正常输出，不报 lock 缺失。

- [ ] **Step 4：跑测试确认零回归**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor/frontend && npm test 2>&1 | tail -8
```

Expected：`Tests  28 passed (28)`。

- [ ] **Step 5：commit**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor && git status --short
```

Expected：只有 `D  frontend/yarn.lock` 一项 staged（npm ci 不应该新建任何 dirty 文件）。

```bash
git commit -m "chore: 删除已废弃的 frontend/yarn.lock"
git log -1 --stat
```

Expected：1 file changed, 0 insertions(+), 407 deletions(-)（行数以实际为准）。

---

## Task 2：vitest 生成 lcov reporter

**Files:**
- Modify: `frontend/vitest.config.ts`（reporter 数组）

- [ ] **Step 1：编辑配置**

把 `frontend/vitest.config.ts` 第 18 行附近的：

```ts
      reporter: ['text', 'html'],
```

改为：

```ts
      reporter: ['text', 'html', 'lcov'],
```

仅改这 1 处，不动 thresholds、include、exclude。

- [ ] **Step 2：跑覆盖率验证 lcov 生成**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor/frontend && npm run test:coverage 2>&1 | tail -5 && ls -la coverage/lcov.info 2>&1 | head -2
```

Expected：
- 测试 28 passed、阈值不红
- `coverage/lcov.info` 存在且 size > 0（一般几 KB）

- [ ] **Step 3：抽查 lcov 内容格式**

Run:
```bash
head -10 /Users/bobochang/Documents/legal-extractor/frontend/coverage/lcov.info
```

Expected：第一行是 `TN:` 开头（test name 段，lcov 标准前缀），随后 `SF:` 段记录被测文件路径。

- [ ] **Step 4：确认 .gitignore 已忽略 coverage/**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor && git status --short | grep -E "coverage|lcov" || echo "no coverage artifacts in git status (correct)"
```

Expected：输出 "no coverage artifacts in git status (correct)"，证明 `frontend/coverage/` 已被 .gitignore 拦下。

- [ ] **Step 5：commit**

```bash
cd /Users/bobochang/Documents/legal-extractor
git add frontend/vitest.config.ts
git commit -m "test: 前端覆盖率额外输出 lcov 以便接入 codecov"
git log -1 --stat
```

Expected：`1 file changed, 1 insertion(+), 1 deletion(-)`。

---

## Task 3：新建 codecov.yml + CI 双 flag upload

**Files:**
- Create: `codecov.yml`（仓库根）
- Modify: `.github/workflows/ci.yml`（Go test job 加 flag；frontend-test job 末尾加 upload step）

- [ ] **Step 1：创建 `codecov.yml`**

写入仓库根 `codecov.yml`：

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

- [ ] **Step 2：本地 lint codecov.yml 语法**

Run:
```bash
cat /Users/bobochang/Documents/legal-extractor/codecov.yml | curl --silent --data-binary @- https://codecov.io/validate
```

Expected：响应里包含 `"valid": true` 或 "Valid!" 字样。

> 如果网络不通或 codecov 服务暂时不可达，跳过此步骤；正确性由 CI 上传时由 codecov 自身校验。

- [ ] **Step 3：给 Go test job 加 flag**

编辑 `.github/workflows/ci.yml`。在原有 Go upload step（约第 58-63 行）：

```yaml
      - name: Upload coverage to Codecov
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        uses: codecov/codecov-action@v5
        with:
          files: coverage.out
          fail_ci_if_error: false
```

改为：

```yaml
      - name: Upload coverage to Codecov
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        uses: codecov/codecov-action@v5
        with:
          files: coverage.out
          flags: backend
          fail_ci_if_error: false
```

仅插入一行 `flags: backend`。

- [ ] **Step 4：给 frontend-test job 加 upload step**

`.github/workflows/ci.yml` 中 `frontend-test` job 现有最后一步：

```yaml
      - name: Run tests with coverage
        working-directory: frontend
        run: npm run test:coverage
```

在其后追加新 step（保持与 Go 同样缩进、同样字段集）：

```yaml
      - name: Upload frontend coverage to Codecov
        if: github.event_name == 'push' && github.ref == 'refs/heads/main'
        uses: codecov/codecov-action@v5
        with:
          files: frontend/coverage/lcov.info
          flags: frontend
          fail_ci_if_error: false
```

- [ ] **Step 5：YAML 语法本地校验**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); yaml.safe_load(open('codecov.yml')); print('YAML OK')"
```

Expected：`YAML OK`。

如果 python yaml 模块不可用，改用：
```bash
which yq && yq eval '.' .github/workflows/ci.yml > /dev/null && yq eval '.' codecov.yml > /dev/null && echo "YAML OK"
```

如果两者都不可用，跳过此步——CI 推送后会立即报错。

- [ ] **Step 6：grep 自检改动一致性**

Run:
```bash
grep -n "flags:" /Users/bobochang/Documents/legal-extractor/.github/workflows/ci.yml
```

Expected：恰好两行命中——一个 `flags: backend`、一个 `flags: frontend`。

```bash
grep -c "codecov-action@v5" /Users/bobochang/Documents/legal-extractor/.github/workflows/ci.yml
```

Expected：`2`（一个 Go、一个 frontend）。

- [ ] **Step 7：commit**

```bash
cd /Users/bobochang/Documents/legal-extractor
git add codecov.yml .github/workflows/ci.yml
git commit -m "ci: 前后端覆盖率分 flag 上传 codecov"
git log -1 --stat
```

Expected：2 files changed，新增 codecov.yml + 修改 ci.yml；commit message 完全一致。

---

## Task 4：开 PR + CI + Squash Merge

> 这一步推送远端与开 PR。如果用户不希望此 agent 创建 PR，停下来询问；否则按下面执行。

- [ ] **Step 1：分支总览**

Run:
```bash
cd /Users/bobochang/Documents/legal-extractor && git log --oneline main..HEAD
```

Expected：4 个 commit——
- `docs: 新增 codecov 双 flag 与 yarn lock 清理设计`
- `chore: 删除已废弃的 frontend/yarn.lock`
- `test: 前端覆盖率额外输出 lcov 以便接入 codecov`
- `ci: 前后端覆盖率分 flag 上传 codecov`

- [ ] **Step 2：推送分支**

Run:
```bash
git push -u origin feature/codecov-and-yarn-cleanup
```

Expected：远端创建分支并跟踪。

- [ ] **Step 3：开 PR**

Run:
```bash
gh pr create --base main --head feature/codecov-and-yarn-cleanup \
  --title "chore: 删除 yarn lock 并接入 codecov 双 flag" \
  --body "$(cat <<'EOF'
## 概述

清理两件长期遗留小事：
1. 删除自 v2 时代起就无人调用、却还会被 \`npm install\` 偶发改动的 \`frontend/yarn.lock\`
2. 把前端覆盖率接入 Codecov，与 Go 链路并轨为双 flag dashboard（\`backend\` + \`frontend\`）

## 改动清单

| 文件 | 改动 |
|---|---|
| \`frontend/yarn.lock\` | 删除（仓库完全使用 npm，yarn.lock 无人调用却产生 git noise） |
| \`frontend/vitest.config.ts\` | \`coverage.reporter\` 增加 \`'lcov'\` 项，让 \`npm run test:coverage\` 同时产出 \`coverage/lcov.info\` |
| \`.github/workflows/ci.yml\` | Go upload step 加 \`flags: backend\`；frontend-test job 新增 upload step（\`flags: frontend\`），触发条件与 Go 一致（仅 push to main） |
| \`codecov.yml\` | 新建：定义 backend / frontend 两个 flag，绑定路径 \`internal+cmd\` / \`frontend/src\`，开启 carryforward，状态阈值 ±1pp |

## 设计抉择

- **双 flag 而非合并上传**：dashboard 上前后端分轨展示，互不污染。前端实测约 50% 量级（受 App.vue / ConfigPanel.vue 巨石未测拖累），不应拉低 Go 链路真实覆盖率视图。
- **paths 限定 + carryforward**：让 Codecov 知道每个 flag 各管哪些目录；某次单边上传时另一边数据不会"消失"。
- **仅 push to main 上传**：与 v3.0 起 Go 链路一致；PR 不上传，避免草稿数据污染主线趋势。
- **vitest 阈值层不变**：防回归继续在 vitest \`coverage.thresholds\` 层做（PR 阻塞用），Codecov 仅作观测——不重复设防。
- **不动 README badge**：现有总分 badge 仍有效；分 flag 数据通过 dashboard 查看。
- **\`fail_ci_if_error: false\`**：upload 失败不挂 CI（沿用 Go 链路方针，Codecov 是观测不应阻塞）。

## 验证

- \`grep -rn "yarn" .github/ Dockerfile wails.json build.sh\` 无任何脚本引用
- \`cd frontend && npm test\` 28 passed（无回归）
- \`cd frontend && npm run test:coverage\` 阈值 PASS、产出 \`coverage/lcov.info\`
- \`python3 -c "import yaml; yaml.safe_load(...)"\` 验证 YAML 语法
- 合入 main 后约 2 分钟，codecov.io dashboard 应同时显示 \`backend\` 与 \`frontend\` 两个 flag 的数字

## 后续候选（明确不在本 PR）

1. \`App.vue\` 拆分（独立一轮，先拆再测）
2. \`ConfigPanel.vue\` 拆分
3. macOS / Linux OCR 兜底
4. 多文件批量上传与队列
5. E2E 集成测试
6. codecov.yml 加 PR comment 自定义 / status check 阻塞策略（先观察 1-2 周自动 target 是否够用）
EOF
)"
```

Expected：输出 PR URL。

- [ ] **Step 4：等 CI**

Run:
```bash
gh pr checks --watch
```

Expected：5 项全 `pass`：Lint / Test / Build Check / GitGuardian Security / Frontend Test。

注意：codecov upload step 由于 `if: push && main` 条件，**在 PR run 里不会执行**——所以 PR CI 不验证上传链路。这是预期行为。上传链路要在 squash merge 后由 main 的 push 触发验证。

- [ ] **Step 5：Squash merge**

Run（把 `<NUM>` 替换为 PR 编号）：
```bash
gh pr merge <NUM> --squash --delete-branch \
  --subject "chore: 删除 yarn lock 并接入 codecov 双 flag" \
  --body "删除自 v2 时代起遗留无用的 frontend/yarn.lock；vitest 输出 lcov；CI 把前后端覆盖率以 backend / frontend 双 flag 推送 Codecov。dashboard 从此两轨独立展示。"
```

Expected：PR 合入；远端分支自动删除；本地 main fast-forward。

- [ ] **Step 6：同步本地 + 验证 codecov 上传**

Run:
```bash
git checkout main
git pull --ff-only
git fetch --prune
```

Expected：本地 main 与 origin/main 一致；本地 `feature/codecov-and-yarn-cleanup` 自动消失。

随后等约 2-3 分钟（main 的 push 会触发 Go 与 frontend 两个 job，各跑一次 codecov upload），用浏览器访问：

```
https://codecov.io/gh/can4hou6joeng4/legal-extractor
```

Expected：Flags 区域应显示 `backend` 与 `frontend` 两条覆盖率线。如果只显示 `backend`（前端那条没出来），用 `gh run view --log <main-run-id>` 拉 main 那次 run 的日志，看 frontend 的 upload step 是否成功。

---

## Done Criteria

- [ ] PR 已通过 CI（5 项全绿）并合入 main
- [ ] `git log --oneline -3` 头部显示 squash commit `chore: 删除 yarn lock 并接入 codecov 双 flag`
- [ ] `frontend/yarn.lock` 不在工作树或 git index
- [ ] `cd frontend && npm run test:coverage` 产出 `coverage/lcov.info`
- [ ] `codecov.yml` 在仓库根，YAML 语法合法
- [ ] CI `frontend-test` job 末尾含 codecov upload step（带 `flags: frontend`）
- [ ] CI `test` job 的 codecov upload step 含 `flags: backend`
- [ ] 合入后 codecov.io dashboard 同时显示 backend 与 frontend 两个 flag
- [ ] 全部 commit 与 PR 标题符合 `type: 中文描述` 规范

---

## 异常处置预案

| 症状 | 排查 | 处置 |
|---|---|---|
| `npm ci` 在删 yarn.lock 后报错 | npm 版本太旧（< 7） | 升级 Node 到 20；CI 已用 Node 20，本地若 < 20 也无妨，CI 是权威 |
| `coverage/lcov.info` 没生成 | reporter 数组写错（如 `lcov-info` 而非 `lcov`） | 校验 vitest 文档：v8 provider 支持 reporter 名是 `'lcov'` |
| codecov dashboard 看不到 frontend flag | upload step 失败或 path 不匹配 | `gh run view --log` 看 step；确认 `files:` 路径用 `frontend/coverage/lcov.info` 不要漏前缀 |
| codecov 把 frontend 数据归到 backend flag | flag 字段写错或 codecov.yml 没生效 | 验证 codecov.yml 在**仓库根**；flag 字段是 `flags`（复数）不是 `flag` |
| YAML lint 报错 indent | tab 与空格混用 | 用 `cat -A` 看，全部统一为 2 空格 |
| `codecov.yml` 在 codecov.io 验证 invalid | flag 名包含非法字符或 paths 写成绝对路径 | flag 名必须 `[a-z][a-z0-9_-]*`；paths 必须仓库相对路径 |
| Go 那边 dashboard 数据"突然降低" | 加 flag 后旧数据没 carryforward | codecov.yml 已设 `carryforward: true`，等下一次 main push 自然恢复 |
