/**
 * 字段高亮匹配：在 pdf.js 文本层（PositionedText 序列）中定位字段值，
 * 输出可直接绝对定位的高亮矩形（CSS 像素，与渲染画布同坐标系）。
 *
 * 纯函数、不依赖 pdf.js —— 匹配逻辑可完整单测。
 *
 * 匹配策略：
 * 1. 将文本层拼接并去除全部空白，同时做 NFKC 折叠（部分 PDF 的 ToUnicode 映射
 *    会把汉字吐成康熙部首形码位，如 ⾝→身、⼀→一，全角数字/标点同理），
 *    维护 归一化下标 → (文本项, 项内偏移) 映射；折叠按码点进行，
 *    组合序列（e + 组合符 é）不做跨码点组合，属已知不支持；
 * 2. 短字段可带上下文前缀（如 被告：张三）避免误中同名；命中后裁掉前缀；
 * 3. 长字段整串未命中时退化为 前缀锚定 + 后缀扩展；
 * 4. 同页多条记录逐字段「向后认领」：后一条记录从上一条命中区间之后继续找，
 *    避免同值/同开头字段全部叠在首个出现位置；向后无果再回退共享首处
 *    （同一段原文对应多条记录的场景）；
 * 5. 命中的字符按行聚类，每行合并为一个矩形；项内偏移按字符视觉宽度加权均摊
 *    （全宽字符按半角两倍计，混排行的数字/字母框不至于漂移）。
 */
import type { PositionedText } from "./pdfDoc";

export interface HighlightRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

/** 短字段的上下文前缀：优先带前缀匹配，避免值本身在页内多处出现时误中 */
const CONTEXT_PREFIXES: { [fieldKey: string]: string[] } = {
  defendant: ["被告：", "被告:", "被 告：", "被 告:"],
};

const LONG_FIELD_PREFIX = 20; // 长字段前缀锚定长度（归一化字符数）
const LONG_FIELD_SUFFIX = 10; // 后缀扩展长度
const LINE_GAP_RATIO = 0.6; // 行聚类阈值（相对行高）

function normalize(s: string): string {
  return s.normalize("NFKC").replace(/[\s　]+/g, "");
}

/** 视觉宽度权重：全宽字符（汉字/部首形/假名/全角标点数字）约为半角两倍 */
function charWeight(ch: string): number {
  // U+2E80-9FFF 部首/假名/CJK 标点/统一汉字；U+F900-FAFF 兼容汉字；U+FF00-FF60/FFE0-FFE6 全角形式
  return /[\u2E80-\u9FFF\uF900-\uFAFF\uFF00-\uFF60\uFFE0-\uFFE6]/.test(ch) ? 2 : 1;
}

interface CharRef {
  item: number;
  offset: number; // 项内码点起始偏移（UTF-16 码元下标）
  span: number; // 该码点占用的码元数（1 或 2）
}

interface PageLayout {
  items: PositionedText[];
  hay: string;
  refs: CharRef[];
}

/** 归一化拼接 + 下标映射：整页构建一次，供多记录多字段复用 */
function buildLayout(items: PositionedText[]): PageLayout {
  const refs: CharRef[] = [];
  let hay = "";
  items.forEach((it, idx) => {
    for (let i = 0; i < it.str.length; ) {
      const cp = it.str.codePointAt(i) as number;
      const span = cp > 0xffff ? 2 : 1;
      // 按码点折叠（代理对整体处理）；折叠出的每个码元都指回同一源码点，
      // 保证 hay 码元下标与 refs 数组下标恒等
      const folded = it.str.slice(i, i + span).normalize("NFKC");
      for (let j = 0; j < folded.length; j++) {
        const ch = folded[j];
        if (!/[\s　]/.test(ch)) {
          hay += ch;
          refs.push({ item: idx, offset: i, span });
        }
      }
      i += span;
    }
  });
  return { items, hay, refs };
}

/** 从归一化下标 from 起定位 value，返回命中区间 [start, end)；找不到返回 null */
function findRange(
  layout: PageLayout,
  value: string,
  fieldKey: string | undefined,
  from: number,
): { start: number; end: number } | null {
  const needle = normalize(value);
  if (!needle) return null;
  const { hay } = layout;
  if (!hay) return null;

  for (const prefix of (fieldKey && CONTEXT_PREFIXES[fieldKey]) || []) {
    const p = normalize(prefix);
    const idx = hay.indexOf(p + needle, from);
    if (idx >= 0) {
      const start = idx + p.length;
      return { start, end: start + needle.length };
    }
  }
  const idx = hay.indexOf(needle, from);
  if (idx >= 0) return { start: idx, end: idx + needle.length };

  if (needle.length > LONG_FIELD_PREFIX + 4) {
    // 长字段退化：前缀锚定 + 后缀扩展
    const prefix = needle.slice(0, LONG_FIELD_PREFIX);
    const aIdx = hay.indexOf(prefix, from);
    if (aIdx >= 0) {
      let end = aIdx + prefix.length;
      const suffix = needle.slice(-LONG_FIELD_SUFFIX);
      const sIdx = hay.indexOf(suffix, end);
      // 后缀离锚点过远说明跨页/断裂，只保留锚定段
      if (sIdx >= 0 && sIdx - aIdx <= needle.length * 1.5) {
        end = sIdx + suffix.length;
      }
      return { start: aIdx, end };
    }
  }
  return null;
}

/** 命中区间 → 逐字符矩形（项内按视觉宽度加权均摊）→ 按行聚类合并 */
function rectsForRange(layout: PageLayout, start: number, end: number): HighlightRect[] {
  const { items, refs } = layout;
  const pieces: HighlightRect[] = [];
  let cachedItem = -1;
  let prefixW: number[] = []; // prefixW[i] = 项内前 i 个码元的权重和
  let unitW = 0;
  for (let k = start; k < end && k < refs.length; k++) {
    const ref = refs[k];
    const it = items[ref.item];
    if (ref.item !== cachedItem) {
      cachedItem = ref.item;
      prefixW = [0];
      for (let i = 0; i < it.str.length; i++) prefixW.push(prefixW[i] + charWeight(it.str[i]));
      unitW = prefixW[it.str.length] > 0 ? it.width / prefixW[it.str.length] : 0;
    }
    pieces.push({
      left: it.left + unitW * prefixW[ref.offset],
      top: it.top,
      width: unitW * (prefixW[ref.offset + ref.span] - prefixW[ref.offset]),
      height: it.height,
    });
  }
  if (pieces.length === 0) return [];

  pieces.sort((a, b) => a.top - b.top || a.left - b.left);
  const rects: HighlightRect[] = [];
  let line: HighlightRect[] = [];
  let lineTop = pieces[0].top;

  const flush = () => {
    if (line.length === 0) return;
    const left = Math.min(...line.map((p) => p.left));
    const right = Math.max(...line.map((p) => p.left + p.width));
    const top = Math.min(...line.map((p) => p.top));
    const bottom = Math.max(...line.map((p) => p.top + p.height));
    rects.push({ left, top, width: right - left, height: bottom - top });
    line = [];
  };

  for (const p of pieces) {
    const threshold = Math.max(4, p.height * LINE_GAP_RATIO);
    if (line.length > 0 && p.top - lineTop > threshold) {
      flush();
      lineTop = p.top;
    }
    if (line.length === 0) lineTop = p.top;
    line.push(p);
  }
  flush();
  return rects;
}

/** 在文本层中查找 value，返回按行合并后的高亮矩形；找不到返回空数组。 */
export function matchValueRects(items: PositionedText[], value: string, fieldKey?: string): HighlightRect[] {
  const layout = buildLayout(items);
  const r = findRange(layout, value, fieldKey, 0);
  return r ? rectsForRange(layout, r.start, r.end) : [];
}

export interface FieldHighlight {
  key: string;
  label: string;
  recIndex: number;
  rects: HighlightRect[];
}

/** 为一页上的多条记录的可见字段批量计算高亮。 */
export function computeFieldHighlights(
  items: PositionedText[],
  targets: { recIndex: number; fields: { key: string; label: string; value: string }[] }[],
): FieldHighlight[] {
  const layout = buildLayout(items);
  const out: FieldHighlight[] = [];
  // 同字段向后认领：记录该字段下一次搜索的起点
  const nextFrom: { [fieldKey: string]: number } = {};
  for (const t of targets) {
    for (const f of t.fields) {
      if (!f.value || !f.value.trim()) continue;
      const from = nextFrom[f.key] || 0;
      let r = findRange(layout, f.value, f.key, from);
      // 向后无果时回退从头找：同一段原文可能对应多条记录（共享首处）
      if (!r && from > 0) r = findRange(layout, f.value, f.key, 0);
      if (!r) continue;
      nextFrom[f.key] = Math.max(from, r.end);
      const rects = rectsForRange(layout, r.start, r.end);
      if (rects.length > 0) {
        out.push({ key: f.key, label: f.label, recIndex: t.recIndex, rects });
      }
    }
  }
  return out;
}
