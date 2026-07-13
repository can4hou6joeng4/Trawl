<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { loadPdf, type LoadedPdf, type PositionedText } from "../utils/pdfDoc";
import { computeFieldHighlights, type FieldHighlight } from "../utils/pdfHighlights";
import type { OcrLine } from "../services";

export interface HighlightTarget {
  recIndex: number;
  fields: { key: string; label: string; value: string }[];
}

const props = withDefaults(
  defineProps<{
    fileName: string;
    /** 文书类型：pdf 走 pdf.js 渲染，image 直接显示，docx 显示占位页 */
    kind: "pdf" | "image" | "docx";
    /** 文书内容（base64），null 表示未加载或读取失败 */
    docData: string | null;
    currentPage: number;
    totalPages: number;
    /** 产生了记录的页码集合 */
    recordPages: number[];
    /** 当前页上的记录（用于页面上的定位入口） */
    recordsOnPage: { index: number; label: string }[];
    /** 当前页记录的可见字段值，用于文本层高亮匹配 */
    highlightTargets?: HighlightTarget[];
    /** 当前页的云端 OCR 坐标行（归一化 [0,1]），扫描件/图片高亮数据源 */
    ocrLines?: OcrLine[];
    /** 需要在页面高亮上闪烁的记录下标（「定位原文」回链） */
    flashRecIndex?: number | null;
  }>(),
  {
    highlightTargets: () => [],
    ocrLines: () => [],
    flashRecIndex: null,
  },
);

const emit = defineEmits<{
  (e: "update:currentPage", page: number): void;
  (e: "locate", recordIndex: number): void;
  (e: "close"): void;
  /** pdf 文档解析完成，上报真实总页数 */
  (e: "loaded", numPages: number): void;
}>();

const mockLines = [
  { w: "88%" }, { w: "72%" }, { w: "84%", mb: 22 },
  { w: "40%" }, { w: "91%" }, { w: "86%", mb: 22 },
  { w: "34%" }, { w: "89%" }, { w: "93%" }, { w: "58%" },
];

const canvasEl = ref<HTMLCanvasElement | null>(null);
const imgEl = ref<HTMLImageElement | null>(null);
const pageCardEl = ref<HTMLElement | null>(null);
const rendering = ref(false);
const renderFailed = ref(false);
const highlights = ref<FieldHighlight[]>([]);

// 已有字段高亮的记录不再显示 REC 角标；未匹配到的记录保留角标作为定位入口
const unmatchedRecs = computed(() =>
  props.recordsOnPage.filter((r) => !highlights.value.some((h) => h.recIndex === r.index)),
);

let pdf: LoadedPdf | null = null;
let generation = 0; // 防止过期的异步加载/渲染写入新状态

const imageSrc = computed(() => {
  if (props.kind !== "image" || !props.docData) return "";
  const mime = /\.png$/i.test(props.fileName) ? "image/png" : "image/jpeg";
  return `data:${mime};base64,${props.docData}`;
});

/** pdf 已就绪且未失败时显示画布 */
const showCanvas = computed(() => props.kind === "pdf" && props.docData !== null && !renderFailed.value);

async function renderCurrent() {
  if (!pdf || !canvasEl.value || !pageCardEl.value) return;
  const gen = generation;
  const page = props.currentPage;
  rendering.value = true;
  try {
    const cssWidth = pageCardEl.value.clientWidth || 340;
    await pdf.renderPage(page, canvasEl.value, cssWidth);
    // 渲染成功即自愈：此前偶发的失败不应让画布在本文档内永久退化
    if (gen === generation) renderFailed.value = false;
    // 仅当本次渲染仍对应当前页时刷新高亮，过期渲染交给最新一次处理
    if (gen === generation && page === props.currentPage) void refreshHighlights();
  } catch (e) {
    if (gen !== generation) return; // 换文档/卸载引发的中断属预期，静默
    console.error("PDF 页面渲染失败:", e);
    renderFailed.value = true;
    highlights.value = [];
  } finally {
    if (gen === generation) rendering.value = false;
  }
}

// 在文本层中匹配当前页记录的字段值，生成高亮矩形。
// 数据源优先级：pdf.js 文本层（字符级坐标，最精确）→ 云端 OCR 坐标行
// （扫描件/图片，行级坐标）→ 都没有则无高亮（保留 REC 角标作为定位入口）。
async function refreshHighlights() {
  if (props.kind === "image") {
    refreshImageHighlights();
    return;
  }
  if (!pdf || props.kind !== "pdf") {
    highlights.value = [];
    return;
  }
  const gen = generation;
  const page = props.currentPage;
  try {
    // 与画布共用同一宽度基准（renderPage 设置的 CSS 宽），
    // 容器出现滚动条/窗口调整后重测容器会导致坐标与画布错位
    const cssWidth = canvasEl.value?.clientWidth || pageCardEl.value?.clientWidth || 340;
    let items = await pdf.getTextLayout(page, cssWidth);
    if (gen !== generation || page !== props.currentPage) return; // 已换文档或已翻页
    if (items.length === 0 && props.ocrLines.length > 0) {
      // 渲染在途时画布尺寸仍属旧页，等渲染完成后的那次刷新（renderCurrent
      // 在置 rendering=false 之前发起刷新，本守卫不拦截该路径）
      if (rendering.value) return;
      // 扫描件：无文本层，用云端 OCR 坐标行（归一化 × 画布显示尺寸）
      items = ocrToPositioned(props.ocrLines, cssWidth, canvasEl.value?.clientHeight || 0);
    }
    highlights.value = computeFieldHighlights(items, props.highlightTargets);
  } catch (e) {
    if (gen !== generation) return; // 换文档/卸载使在途请求 reject 属预期，静默
    console.error("字段高亮计算失败:", e);
    highlights.value = [];
  }
}

// 图片文书：OCR 坐标行 × 图片显示尺寸，同步计算无异步竞态
function refreshImageHighlights() {
  const el = imgEl.value;
  if (!el || !el.clientWidth || !el.clientHeight || props.ocrLines.length === 0) {
    highlights.value = [];
    return;
  }
  const items = ocrToPositioned(props.ocrLines, el.clientWidth, el.clientHeight);
  highlights.value = computeFieldHighlights(items, props.highlightTargets);
}

/** 归一化 OCR 行 → 显示坐标系的文本项（与 pdf.js 文本层同构，复用同一匹配器） */
function ocrToPositioned(lines: OcrLine[], displayW: number, displayH: number): PositionedText[] {
  if (displayW <= 0 || displayH <= 0) return [];
  return lines.map((l) => ({
    str: l.text,
    left: l.x * displayW,
    top: l.y * displayH,
    width: l.w * displayW,
    height: l.h * displayH,
  }));
}

// 字段显隐、记录值编辑或 OCR 行到位后重新匹配
// （人工修正后原文不再匹配属预期：高亮消失）
watch(
  () => JSON.stringify([props.highlightTargets, props.ocrLines]),
  () => {
    // 渲染在途时跳过：renderCurrent 完成后的刷新在 await 后读取 props，
    // 必然携带最新 targets/ocrLines，这里的重复往返注定被丢弃
    if (rendering.value) return;
    if (pdf || props.kind === "image") void refreshHighlights();
  },
);

watch(
  () => [props.kind, props.docData] as const,
  async ([kind, data]) => {
    generation += 1;
    const gen = generation;
    pdf?.destroy();
    pdf = null;
    renderFailed.value = false;
    highlights.value = [];

    if (kind !== "pdf" || !data) return;
    rendering.value = true;
    try {
      const loaded = await loadPdf(data);
      if (gen !== generation) {
        loaded.destroy();
        return;
      }
      pdf = loaded;
      emit("loaded", loaded.numPages);
      await renderCurrent();
    } catch (e) {
      console.error("PDF 解析失败:", e);
      if (gen === generation) {
        renderFailed.value = true;
        rendering.value = false;
      }
    }
  },
  { immediate: true },
);

watch(
  () => props.currentPage,
  () => {
    if (pdf) {
      highlights.value = []; // 翻页即清，避免旧页高亮残留在新页画布上
      void renderCurrent();
    }
  },
);

onUnmounted(() => {
  generation += 1;
  pdf?.destroy();
  pdf = null;
});

function go(delta: number) {
  const next = props.currentPage + delta;
  if (next >= 1 && next <= props.totalPages) {
    emit("update:currentPage", next);
  }
}
</script>

<template>
  <div class="docpane">
    <div class="doc-name">
      <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
      <span class="p" :title="fileName">{{ fileName }}</span>
      <button class="back" type="button" @click="emit('close')">✕ 关闭</button>
    </div>

    <div ref="pageCardEl" class="pagecard" :class="{ real: showCanvas || (kind === 'image' && imageSrc) }">
      <!-- 真实原文（PDF 画布 / 图片）+ 字段高亮覆盖层 -->
      <div
        v-show="showCanvas || (kind === 'image' && imageSrc)"
        class="canvas-wrap"
        :class="{ 'image-wrap': kind === 'image' }"
      >
        <canvas v-show="showCanvas" ref="canvasEl" class="pdf-canvas"></canvas>
        <img
          v-if="kind === 'image' && imageSrc"
          ref="imgEl"
          class="doc-image"
          :src="imageSrc"
          :alt="fileName"
          @load="refreshHighlights"
        />
        <template v-for="(h, hi) in highlights" :key="hi">
          <button
            v-for="(r, ri) in h.rects"
            :key="ri"
            class="field-hl"
            :class="{ flash: flashRecIndex === h.recIndex }"
            type="button"
            :style="{ left: r.left - 2 + 'px', top: r.top - 2 + 'px', width: r.width + 4 + 'px', height: r.height + 4 + 'px' }"
            :title="h.label + ' · 点击定位记录'"
            @click="emit('locate', h.recIndex)"
          >
            <span v-if="ri === 0" class="tag" :class="{ below: r.top < 15 }">{{ h.label }}</span>
          </button>
        </template>
      </div>
      <div v-if="showCanvas && rendering" class="page-loading">
        <span class="spinner"></span>
      </div>

      <!-- DOCX / 数据未就绪 / 渲染失败：占位页 -->
      <template v-if="!showCanvas && !(kind === 'image' && imageSrc)">
        <div class="pl t"></div>
        <div
          v-for="(l, i) in mockLines"
          :key="i"
          class="pl"
          :style="{ width: l.w, marginBottom: l.mb ? l.mb + 'px' : undefined }"
        ></div>
        <div v-if="renderFailed" class="page-note">原文渲染失败，已显示占位页</div>
      </template>

      <div v-if="unmatchedRecs.length > 0" class="page-recs">
        <button
          v-for="r in unmatchedRecs"
          :key="r.index"
          class="page-rec"
          type="button"
          :title="'定位到 ' + r.label"
          @click="emit('locate', r.index)"
        >{{ r.label }}</button>
      </div>
      <div v-else-if="!showCanvas && kind !== 'image'" class="page-empty">本页无匹配记录</div>
    </div>

    <div class="pagenav">
      <button class="nav" type="button" :disabled="currentPage <= 1" @click="go(-1)">◂</button>
      <span class="cur">第 {{ currentPage }} / {{ totalPages }} 页</span>
      <button class="nav" type="button" :disabled="currentPage >= totalPages" @click="go(1)">▸</button>
      <span class="dotrow">
        <button
          v-for="i in totalPages"
          :key="i"
          class="pdot"
          type="button"
          :class="{ has: recordPages.includes(i), cur: currentPage === i }"
          :title="'第 ' + i + ' 页'"
          @click="emit('update:currentPage', i)"
        ></button>
      </span>
    </div>
  </div>
</template>

<style scoped>
.docpane {
  border-right: 1px solid var(--surface-border);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}
.doc-name {
  font-size: 11.5px;
  color: var(--accent-soft);
  font-family: var(--font-mono);
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}
.doc-name .p {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.doc-name .back {
  margin-left: auto;
  color: var(--text-muted);
  font-size: 10px;
  flex-shrink: 0;
  background: none;
  border: none;
  cursor: pointer;
  font-family: inherit;
}
.doc-name .back:hover {
  color: var(--error);
}
.pagecard {
  flex: 1;
  min-height: 0;
  background: #f5f2ea;
  border-radius: 6px;
  position: relative;
  overflow: hidden;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
  padding: 26px 24px;
}
/* 真实内容（pdf 画布/图片）模式下取消占位页的内边距 */
.pagecard.real {
  padding: 0;
  background: #e8e4d8;
  overflow-y: auto;
  scrollbar-width: thin;
}
.canvas-wrap {
  position: relative;
  width: fit-content;
  margin: 0 auto;
}
/* 图片模式撑满页卡宽度（与原直铺行为一致），高亮坐标随图片显示尺寸换算 */
.canvas-wrap.image-wrap {
  width: 100%;
}
.pdf-canvas {
  display: block;
}
.field-hl {
  position: absolute;
  border: 1.5px solid rgba(24, 178, 93, 0.85);
  background: rgba(52, 216, 121, 0.16);
  border-radius: 3px;
  padding: 0;
  cursor: pointer;
  transition: background 0.15s;
}
.field-hl:hover {
  background: rgba(52, 216, 121, 0.32);
}
.field-hl.flash {
  animation: hlflash 0.55s ease-in-out 2;
}
@keyframes hlflash {
  50% {
    background: rgba(52, 216, 121, 0.5);
    box-shadow: 0 0 0 5px rgba(52, 216, 121, 0.3);
  }
}
.field-hl .tag {
  position: absolute;
  top: -14px;
  left: -1.5px;
  font-family: var(--font-mono);
  font-size: 9px;
  background: var(--accent-deep);
  color: #04170c;
  padding: 1px 6px;
  border-radius: 2px;
  font-weight: 700;
  letter-spacing: 0.5px;
  line-height: 1.5;
  white-space: nowrap;
}
/* 页首行的矩形：角标翻到矩形下方，避免被页卡上缘裁掉（上方溢出不可滚达） */
.field-hl .tag.below {
  top: calc(100% + 1px);
}
.doc-image {
  display: block;
  width: 100%;
  height: auto;
}
.page-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(232, 228, 216, 0.55);
}
.spinner {
  width: 22px;
  height: 22px;
  border: 2.5px solid rgba(24, 178, 93, 0.25);
  border-top-color: var(--accent-deep);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.pl {
  height: 7px;
  border-radius: 2px;
  background: #c9c3b4;
  margin-bottom: 9px;
}
.pl.t {
  background: #8f887a;
  height: 10px;
  width: 46%;
  margin: 0 auto 20px;
}
.page-note {
  position: absolute;
  bottom: 40px;
  left: 0;
  right: 0;
  text-align: center;
  font-family: var(--font-mono);
  font-size: 9.5px;
  color: #b0552e;
  letter-spacing: 1px;
}
.page-recs {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.page-rec {
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: #04170c;
  background: var(--accent-deep);
  border: none;
  border-radius: 3px;
  padding: 3px 8px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(24, 178, 93, 0.4);
  transition: transform 0.12s;
}
.page-rec:hover {
  transform: scale(1.06);
}
.page-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding-bottom: 18px;
  font-family: var(--font-mono);
  font-size: 9.5px;
  color: #a49d8d;
  letter-spacing: 1px;
  pointer-events: none;
}
.pagenav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--accent-soft);
}
.pagenav .cur {
  white-space: nowrap;
}
.pagenav .nav {
  width: 24px;
  height: 24px;
  border: 1px solid var(--surface-border-strong);
  background: transparent;
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.13s;
  font-family: inherit;
}
.pagenav .nav:hover:not(:disabled) {
  color: var(--accent-primary);
  border-color: var(--accent-primary);
}
.pagenav .nav:disabled {
  opacity: 0.35;
  cursor: default;
}
.pagenav .dotrow {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  max-width: 120px;
}
.pagenav .pdot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(84, 140, 105, 0.35);
  border: none;
  padding: 0;
  cursor: pointer;
}
.pagenav .pdot.has {
  background: var(--accent-deep);
}
.pagenav .pdot.cur {
  outline: 1.5px solid var(--accent-primary);
  outline-offset: 1.5px;
  background: var(--accent-primary);
}
</style>
