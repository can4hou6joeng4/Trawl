# 扫描件/图片 OCR 坐标高亮设计（取证台阶段 3）

日期：2026-07-14 ｜ 前置：阶段 2b 字段坐标高亮回链（#53，v4.1.0）

## 背景与目标

阶段 2b 的字段高亮只覆盖有文本层的 PDF（pdf.js `getTextContent` 前端匹配）。
扫描件 PDF 与图片文书走百度 PaddleOCR-VL（Layout Parsing）云端识别，此前只取
`markdown.text`，坐标信息被丢弃，前端只能降级为 REC 角标。

目标：把云端 OCR 返回的**文字行坐标**带回前端，在扫描件/图片上呈现与文本层 PDF
一致的字段高亮与双向回链体验。

## 核心决策

1. **匹配逻辑零重复**：后端只透传「文字行 + 归一化坐标」，不做字段匹配。
   前端把 OCR 行转换成 `PositionedText[]`（归一化坐标 × 显示尺寸），
   直接喂给阶段 2b 的 `computeFieldHighlights`——NFKC 折叠、上下文前缀、
   长字段锚定、向后认领、行聚类全部复用，文本层与 OCR 两条路径行为一致。
2. **坐标归一化在后端**：`rec_boxes` 是服务端栅格像素坐标，前端不知道栅格尺寸。
   后端归一化到 `[0,1]`（PDF 页尺寸取响应 `dataInfo.pages[i]`；图片优先
   `dataInfo.width/height`，缺失时本地 `image.DecodeConfig` 兜底），
   前端乘以显示尺寸即可，分辨率无关。
3. **防御性解析，宁缺毋错**：`prunedResult`/`dataInfo` 用 `RawMessage` 接住后
   逐页 best-effort 解析——字段缺失、形状随部署版本变化（评审确认强类型解码
   会击穿主流程）、数量不齐、页尺寸不可得时，该页不产出坐标行，前端自然回退
   REC 角标；`rec_boxes` 缺失时尝试 `rec_polys`（四点多边形取包围盒）。
   图片尺寸本地兜底仅对 PNG 开放：JPEG 可能带 EXIF 旋转，本地读到的是
   旋转前尺寸，与云端坐标空间可能不一致。分块 PDF 返回页数与切片页数不符时
   （云端缺页会使全局页序漂移），抹掉该块坐标行防止高亮画到物理错页。
4. **响应瘦身**：请求加 `visualize: false`，禁止云端回传 base64 可视化图
   （PaddleX 服务化标准参数，部署不支持时忽略，无副作用）。

## 数据契约

```
extractor 包：
  OcrLine      { text string; x, y, w, h float64 }   // 归一化 [0,1]
  OcrPageLines { page int; lines []OcrLine }          // page 全文档 1-based
  ExtractOutput{ records []Record; ocrPages []OcrPageLines }

签名变化（波及 6 文件 9 调用点）：
  BaiduClient.ParseDocument → (ExtractOutput, error)
  Extractor.ExtractData     → (ExtractOutput, error)
  RecordCache               → 缓存 ExtractOutput（坐标随记录同缓存，命中即回放）
  app.ExtractResult         + OcrPages []extractor.OcrPageLines `json:"ocrPages,omitempty"`

非 OCR 路径（本地文本层 / DOCX / WinOCR）ocrPages 恒为 nil：
  文本层 PDF 前端已有更精确的 pdf.js 坐标；WinOCR 桥不吐坐标，维持角标。
```

分块 PDF：`parseChunkWithRetry` 逐块返回块内行，`ParseDocument` 聚合时
按全局页序号（与 markdown 页对齐）编号。

## 前端渲染

- `DocPane` 模板统一 `.canvas-wrap`：内含 canvas（pdf）或 img（图片）+
  共用高亮覆盖层（按钮/角标/闪烁样式不变）。
- `refreshHighlights` 优先级：pdf 文本层非空 → 用文本层（现状）；
  文本层为空且有 `ocrLines` → OCR 行 ×（canvas clientWidth/Height）；
  `kind === 'image'` → OCR 行 ×（img clientWidth/Height），img `@load` 触发。
- `App.vue` 保存 `ocrPages`，按 `curPage` 计算 `ocrLinesOnPage` 下发；
  换文件/关闭时清空。

## 已知限制（记录为后续增强）

- OCR 行是**行级**框：字段值在行内的横向偏移按匹配器的视觉宽度加权均摊，
  精度低于文本层路径（字符级 transform），但可接受。
- markdown（VLM 重排后）与 rec_texts（原始行）文字可能不完全一致；
  匹配不上即无高亮 → 角标兜底，不会画错位置。
- 窗口 resize 后图片高亮与图片同步过期（与 pdf 路径同一限制）。

## 验证

- Go：httptest 夹具驱动（dataInfo 缺失、增强字段形状异常降级、polys 回退、
  PNG 尺寸兜底、JPEG 不兜底、分块页数守卫 sanitizeChunkPages、越界钳制），
  缓存回放断言；`-race` 全套。
- 前端：DocPane 三分支单测（优先级、几何换算、image 覆盖层、渲染在途守卫）。
- E2E：mock 桥返回 ocrPages，图片与「无文本层 PDF」（sips 由 PNG 转制）
  两条真实渲染路径：4 字段命中、行几何零偏差、双向回链、编辑联动，零控制台错误。
- 线上验证项（待真实 Token 回归）：AI Studio 生产端点对 `visualize:false`
  的兼容性（PaddleX 3.4 契约已确认支持）；EXIF 旋转图片的云端坐标空间。
