# PRD：OCR v2 作业式 API 迁移 与 中继网关授权模式

日期：2026-07-14 ｜ 状态：**已定稿（2026-07-14 产品负责人拍板：路线③ 直接上网关）** ｜ 前置：阶段 3 OCR 坐标高亮（#59）

## 1. 背景

两项新输入触发本设计：

1. **官方 API 形态变化**：项目当前对接的是 AI Studio 专属部署实例的**同步**
   layout-parsing 端点（`n1544et5uec1tbh9.aistudio-app.com/layout-parsing`）。
   官方现行推荐形态是 **v2 作业式（异步）API**：
   `https://paddleocr.aistudio-app.com/api/v2/ocr/jobs`，模型 `PaddleOCR-VL-1.6`，
   提交作业 → 轮询状态 → 下载 JSONL 结果。样例代码已由产品负责人提供。
2. **密钥分发模式变化**：现行政策是「云端凭据绝不随应用分发，用户自行配置
   token」（`release_policy_test.go` 强制断言）。产品负责人决定反转该政策：
   **密钥随应用内置，用户零配置开箱即用**，云 OCR 的使用资格由既有的
   试用期/授权机制控制。

## 2. 目标

- G1 OCR 客户端迁移到 v2 作业式 API，扫描件/图片识别在新端点上全功能可用
  （含阶段 3 的坐标高亮）。
- G2 解析进度从「客户端心跳假进度」升级为**服务端真实页级进度**。
- G3 大文档不再客户端物理分块 + 冷却等待，整文档单作业提交。
- G4 用户安装即用：不再要求配置 token；试用 7 天内全功能（含云 OCR），
  到期锁定，激活码解锁——**授权模型零改动，仅 token 来源变化**。
- G5 发布策略测试与文档同步反映新政策，公开仓库不泄露密钥明文。

## 3. 非目标

- 不引入任何服务端组件（激活服务器/代理网关）——desktop-only 范围不变
  （见 2026-05-14-desktop-only-scope-design.md）。
- 不做按机器用量配额（列入开放问题，视内置 token 用量数据再议）。
- 不保留旧同步端点的双协议兼容：一次性切换，`ocr.job_url` 可配置即是逃生门。
- WinOCR 兜底、DOCX、文本层 PDF 路径不受影响。

## 4. 现状与差距

| 维度 | 现状（同步 layout-parsing） | v2 作业式 API |
|---|---|---|
| 端点 | 专属部署实例 URL | `…/api/v2/ocr/jobs`（官方共享服务） |
| 鉴权 | `Authorization: token {T}` | `Authorization: bearer {T}` |
| 模型 | 部署实例固定 | 请求参数 `model: PaddleOCR-VL-1.6` |
| 提交 | JSON `{file: base64, fileType,…}` | **multipart 表单**（file + model + optionalPayload），免 base64 33% 膨胀 |
| 响应 | 同步返回全部结果（180s 超时风险） | `{data:{jobId}}` → 轮询 `GET /jobs/{id}` |
| 进度 | 无 → 客户端心跳假进度 | `extractProgress{totalPages, extractedPages}` 真实进度 |
| 大文档 | pdfcpu 物理分块（>20 页）+ 10s 冷却 | 服务端整文档处理（分块逻辑退役） |
| 结果 | 响应体内 | `resultUrl.jsonUrl` 下载 **JSONL**，逐行 `result.layoutParsingResults[]` |
| 页内 schema | markdown.text / prunedResult.**overall_ocr_res**（行级）/ dataInfo | markdown.text / prunedResult.**parsing_res_list**（块级）/ dataInfo（**已实测确认**，见 §9.1） |
| 失败 | HTTP 错误码 | `state: failed` + `errorMsg` |

## 5. 方案 A：v2 作业式客户端

### 5.1 生命周期

```
提交（multipart，本地文件模式）
  → 轮询 GET /jobs/{jobId}（间隔 5s，ctx 可取消）
      pending  → 进度事件「云端排队中…」
      running  → 进度事件 extractedPages/totalPages（真实页级进度）
      failed   → 透传 errorMsg，映射稳定错误码
      done     → 取 resultUrl.jsonUrl
  → 下载 JSONL → 逐行逐 result 聚合为页
  → 复用既有管线：ParseMarkdown 提取记录 + buildOcrLines 归一化坐标
```

- **取消**：轮询间检查 ctx，取消即停止轮询弃单（是否存在 `DELETE /jobs/{id}`
  为验证项 V3；无则仅弃单）。返回 `ErrCancelled` 哨兵，错误契约不变。
- **超时守卫**：作业总时长上限（默认 15 分钟，可配置），防 pending 永久卡死。
- **重试**：提交 5xx 沿用既有类型化重试；轮询容忍连续 N 次（默认 3）瞬时网络
  错误后再判失败；JSONL 下载失败退避重试 2 次。
- **JSONL 聚合**：每行 `result.layoutParsingResults[]`，跨行顺序累加为全局页序
  （与样例代码 page_num 语义一致）。`markdown.images`/`outputImages` 一律不下载。
- **坐标源切换**：VL-1.6 的 `prunedResult` 无 `overall_ocr_res`（行级），坐标改取
  `parsing_res_list[]`（块级：`block_content` + `block_bbox`，像素空间与
  `dataInfo.pages[i]`/`prunedResult.width/height` 一致）。后端把非空文本块转为
  `OcrLine` 后，归一化、`ExtractOutput`、缓存、前端匹配与渲染**全链路零改动**。
  已知限制：跨行段落块内的部分匹配按块框横向均摊，存在行内漂移
  （整块命中时框选完整无漂移）；表格块内容为 HTML，匹配自然失败无害。
- **退役清单**：`parseChunkWithRetry`、20 页分块与 10s 冷却、`sanitizeChunkPages`、
  心跳 ticker。`buildOcrLines`/`ExtractOutput`/缓存/前端全部无改动复用。

### 5.2 配置

新增 `ocr:` 配置节（旧 `baidu:` 节保留一个版本的迁移读取，仅 token 生效）：

```yaml
ocr:
  token: ""                                            # 留空则用内置密钥
  job_url: "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs"
  model: "PaddleOCR-VL-1.6"
```

环境变量：`LEGAL_EXTRACTOR_OCR_TOKEN` / `_OCR_JOB_URL` / `_OCR_MODEL`。

### 5.3 错误契约

- `BAIDU_TOKEN_MISSING` 稳定码**保留原名**（前端 errors.ts 契约，改名需两侧
  同步且无用户价值）；语义收窄为「内置密钥被禁用且未配置」，预期极少出现。
- 新增场景「作业失败」：`OCR_JOB_FAILED: {errorMsg}`，前端映射为友好文案；
  取消仍是 `CANCELLED`。

## 6. 方案 B：内置密钥与授权控制

### 6.1 产品行为

- 安装即用：全新安装无任何配置即可识别扫描件/图片。
- 试用期（7 天，BuildTime 起算，机制不变）内全功能含云 OCR；
  到期整体锁定（现状行为）；激活码（机器码绑定，机制不变）解锁。
- 覆盖链：**env > conf.yaml > 内置密钥**。高级用户可换自己的 token
  （用自己的配额），也是内置密钥泄露被封后的用户侧自救通道。

### 6.2 密钥分发路线（关键决策，待产品负责人拍板）

真正的分界线不是「密钥写在哪个文件」，而是**密钥是否随应用分发**——
落到客户端（无论源码、二进制还是本地配置）就一定可被提取。全景：

| 路线 | 密钥落在哪 | 泄露后处置 | 用量控制 | 托管组件 | 运行时依赖 |
|---|---|---|---|---|---|
| ❌ 源码字面量 | 公开仓库 | 即刻泄露，GitGuardian 拦截 | 无 | 无 | 无 |
| ① 构建期注入（ldflags + Secrets） | 每份二进制（strings 可提取） | 轮换 + 重新发版 | 整体封禁 | 无 | 无 |
| ② 激活时下发 | 用户本地配置 | 轮换 + 重新下发（免发版） | 按下发批次 | 下发端点 | 仅激活时 |
| ③ **中继网关（目标架构）** | **仅服务端，永不落客户端** | 上游轮换零影响 | **按机器码/授权精确计量、限流、封禁** | 转发端点（~50 行） | OCR 全程 |

**推荐：分步走**——v4.2.0 先走路线 ①（零托管、最快兑现开箱即用），
客户端自第一天起按「网关就绪」设计（`job_url` 与鉴权头可配置，两条路线
仅是配置差异）；用量数据出来后 v4.3 切路线 ③，去掉内置密钥、发一版完成
迁移。泄露风险只存在于过渡期，且被 6.3 的轮换/监控兜住。

> **拍板结果（2026-07-14）**：不走过渡期，**直接采用路线 ③**。
> 应用任何构建都不携带云端密钥；B1a（构建期注入）取消，6.3 的轮换/监控
> 措施转为网关侧上游 token 的常规运维项。发布策略测试保持「禁止凭据进入
> 构建」的原始不变量，另增网关密钥（`OCR_GW_TOKEN`/`LICENSE_SECRET` 服务端
> 环境变量）永不进入应用构建工作流的断言。

原始诉求「硬编码在代码文件中」在任何路线下都落地为「不在公开源码留存明文」；
若坚持源码字面量需先将仓库转私有。

### 6.3 诚实的安全评估与运营对策

内置于二进制的密钥可被 `strings` 等工具提取，混淆（XOR/分片）只提高门槛、
不改变本质。真正的控制手段是运营侧：

1. **按版本轮换**：每次正式发版换新 token，旧版本随时间自然失效面收窄；
2. **用量监控**：AI Studio 控制台定期核对调用量与授权用户数的合理比例；
3. **泄露应急**：AI Studio 轮换 token → 紧急发版；存量用户经 conf.yaml
   配置自救（覆盖链保证）；
4. **试用/正式分离（推荐）**：试用构建与正式构建注入**不同 token**，
   用量可归因、可独立封禁。
5. **既有密钥卫生**：当前线下提供的 token 已在明文渠道传递过，
   实施前应在 AI Studio **先轮换一次**，新值只进 GitHub Secrets 与本地
   git-ignored 文件。

### 6.4 政策与守护测试变更

`release_policy_test.go` 从「工作流不得出现任何凭据引用」反转为三条新断言：

1. 源码/模板/文档中**不得出现 token 明文**（正则扫描仓库内容）；
2. 发布工作流**必须**经 `secrets.OCR_BUILTIN_TOKEN` 注入 ldflags
   （缺失则发布的是无内置密钥的残缺产物——测试红）；
3. conf.yaml 模板的 `ocr.token` 必须为空串。

（切换路线 ③ 后第 2 条改为「不得注入任何 token + 默认 job_url 指向网关」。）

CLAUDE.md、README（「需配置 Token」相关段落）、TROUBLESHOOTING 同步改写。

### 6.5 中继网关设计（路线 ③，目标架构）

一个无状态转发端点，职责单一：

```
客户端请求（Authorization: 机器码 + 授权码/试用标识）
  → 网关校验（授权有效 / 试用期内按机器码计量）
  → 换上真实 token 转发 AI Studio（提交/轮询两类请求）
  → 原样返回
```

- **结果下载不过网关**：JSONL 的 `resultUrl.jsonUrl` 是免鉴权签名 URL
  （2026-07-14 探针已实测），客户端直连下载。一次识别经网关的流量
  仅 1 次提交 + 约 10 次轮询，免费额度量级。
- **托管选型**：用户在境内，须选国内可达托管（腾讯云函数 / 阿里云 FC
  免费额度，或现成 VPS）；Cloudflare Workers 的 workers.dev 境内不稳定，排除。
- **desktop-only 范围说明**：应用本体仍为纯桌面（不新增 REST/Web 面）；
  网关属外部凭据设施，与「授权码验证」同类，不违背 2026-05-14 决策的初衷。
- **可用性对策**：网关故障时，conf.yaml 自填 token 的覆盖链直连 AI Studio
  自救；网关本身无状态，可多活部署。
- **演进空间**：按授权分级配额、滥用告警、token 池轮转，全部服务端实现，
  客户端零改动。

## 7. 兼容与迁移

- 老用户已配置的 `baidu.token`：迁移读取一版（等价 `ocr.token`），
  发布说明提示改用新键；旧 `baidu.api_url` 指向同步端点，**不再读取**
  （协议已换，指过去也无意义）。
- 提取缓存、坐标高亮前端、导出、错误码映射：零改动。
- 试用/授权机制：零改动。

## 8. 风险与对策

| 风险 | 等级 | 对策 |
|---|---|---|
| 跨行段落块内部分匹配的行内漂移（块级坐标固有） | 低 | 整块命中无漂移；漂移场景框仍落在正确块内；后续可评估按块高拆分行条带 |
| 内置 token 被提取滥用 → 配额烧穿 | 中 | 6.3 全套：轮换/监控/应急/试用正式分离 |
| 官方共享端点限流比专属实例严 | 中 | 验证项 V5；轮询/重试参数可配置 |
| 作业排队+识别时长（实测 1 页约 45s）→ 等待焦虑 | 中 | pending/running 真实进度文案 + 总时长上限 + 可取消；发布说明设预期 |
| GitGuardian 对 Secrets 引用误报 | 低 | 断言只针对明文；Secrets 引用是 GitHub 原生用法 |

## 9. 验证计划

### 9.1 已完成：真实端点实测（2026-07-14，1 页扫描件 PDF 探针作业）

- ✅ **V1 坐标数据源**：`dataInfo` 完好（`{"numPages":1,"pages":[{"width":1600,"height":2200}],"type":"pdf"}`）；
  `prunedResult` **无** `overall_ocr_res`，坐标源为 `parsing_res_list[]`
  （`block_label`/`block_content`/`block_bbox`，像素空间与页尺寸一致，
  另有冗余的 `prunedResult.width/height`）。块粒度：标题与常规行为单行块，
  换行段落为跨行块。方案 A 已按此设计。
- ✅ **V6 鉴权与参数**：`bearer` 头 + multipart + `model: PaddleOCR-VL-1.6`
  全部生效；`optionalPayload` 以 JSON 字符串随表单提交。
- ✅ 状态机实测：`running`（含 `extractProgress 0/1`）→ `done`
  （含 startTime/endTime）；`resultUrl` 仅含 `jsonUrl`。
- ⏱ 时延数据点：1 页约 45 秒（排队+识别），轮询 UX 按此设预期。

### 9.2 待验证（实施前/中补齐）

- **V2** 大文档（>20 页）单作业受理情况与页数上限；
- **V3** 取消语义：有无 DELETE 端点、弃单是否计费；
- **V4** failed 态 errorMsg 形态与常见失败类别（配额/格式/超限）；
- **V5** 限流、并发与配额规则。

## 10. 实施分期

| 期 | 内容 | 交付 |
|---|---|---|
| A1 | v2 客户端（提交/轮询/下载/聚合）+ httptest 全链路测试；网关/直连双模式 | PR，功能等价切换 |
| A2 | 真实进度事件、取消打磨、分块/心跳退役清理 | 随 A1 |
| B1 | **中继网关**（Go 单二进制：机器码/授权 HMAC 校验、双档配额、作业归属、透传转发）+ 部署指引 | 随 A1 同一 PR |
| B2 | 产品负责人部署网关（国内 serverless/VPS）→ 回填默认 gateway_url → 直连模式真实回归 → 发布 **v4.2.0** | Release |

## 11. 开放问题

1. ~~6.2 路线拍板~~ → **已定：路线③ 直接上网关**。
2. ~~试用/正式分离 token~~ → 网关按机器码+授权分档计量，天然解决。
3. 试用档/授权档每日配额初值（暂定 20/200 次提交，可调）。
4. `BAIDU_TOKEN_MISSING` 错误码是否借大版本更名为 `OCR_TOKEN_MISSING`？
5. 作业轮询间隔 5s 与总超时 15 分钟的默认值是否合适（视 V2/V5 数据调整）。
6. 网关多实例部署时配额存储外置（Redis 等）——单实例内存版先行，量起再议。
