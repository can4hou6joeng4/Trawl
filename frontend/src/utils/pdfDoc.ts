/**
 * pdf.js 封装：把 pdfjs-dist 的加载与渲染细节收敛在此模块，
 * 组件只面向 loadPdf() 返回的极简接口（也便于单测 mock）。
 *
 * pdfjs 主库按需动态加载（不进首屏 chunk）；worker 以 ?url 方式由
 * Vite 发射为独立资源，加载失败时 pdf.js 自动回退主线程解析。
 */
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

export interface LoadedPdf {
  numPages: number;
  /** 将第 pageNum 页渲染到 canvas，按 cssWidth 适配宽度（自动处理 DPR）。 */
  renderPage(pageNum: number, canvas: HTMLCanvasElement, cssWidth: number): Promise<void>;
  /**
   * 返回第 pageNum 页的定位文本项（坐标为按 cssWidth 适配后的 CSS 像素，
   * 与 renderPage 的显示尺寸一致），供字段高亮匹配使用。
   * 扫描件没有文本层，返回空数组。
   */
  getTextLayout(pageNum: number, cssWidth: number): Promise<PositionedText[]>;
  destroy(): void;
}

export interface PositionedText {
  str: string;
  left: number;
  top: number;
  width: number;
  height: number;
}

export async function loadPdf(base64: string): Promise<LoadedPdf> {
  const pdfjs = await import("pdfjs-dist");
  pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;

  const raw = atob(base64);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) {
    bytes[i] = raw.charCodeAt(i);
  }

  const doc = await pdfjs.getDocument({
    data: bytes,
    // CJK 非内嵌字体所需的字符映射与标准字体（构建时拷入 pdfjs/ 目录）
    cMapUrl: new URL("pdfjs/cmaps/", document.baseURI).href,
    cMapPacked: true,
    standardFontDataUrl: new URL("pdfjs/standard_fonts/", document.baseURI).href,
  }).promise;

  // 渲染调度：串行链保证同一 canvas 永不并发渲染（pdf.js 对并发渲染同一
  // canvas 直接抛错）；renderSeq 让排队中的过期请求（快速翻页）直接跳过，
  // 画布最终必然停留在最后一次请求的页面；入口 cancel 让在途旧页尽快让位。
  let renderSeq = 0;
  let inflight: { cancel(): void } | null = null;
  let chain: Promise<void> = Promise.resolve();

  return {
    numPages: doc.numPages,
    renderPage(pageNum: number, canvas: HTMLCanvasElement, cssWidth: number): Promise<void> {
      const seq = ++renderSeq;
      if (inflight) {
        inflight.cancel();
        inflight = null;
      }

      const run = async () => {
        if (seq !== renderSeq) return; // 已有更新的翻页请求，跳过

        const page = await doc.getPage(pageNum);
        if (seq !== renderSeq) return; // getPage 异步空窗后复查

        const base = page.getViewport({ scale: 1 });
        const dpr = window.devicePixelRatio || 1;
        const scale = cssWidth / base.width;
        const viewport = page.getViewport({ scale: scale * dpr });

        canvas.width = Math.floor(viewport.width);
        canvas.height = Math.floor(viewport.height);
        canvas.style.width = cssWidth + "px";
        canvas.style.height = Math.floor(viewport.height / dpr) + "px";

        const ctx = canvas.getContext("2d");
        if (!ctx) throw new Error("无法创建画布上下文");

        const task = page.render({ canvasContext: ctx, viewport });
        inflight = task;
        try {
          await task.promise;
        } catch (e) {
          // 翻页导致的主动取消不算错误
          if ((e as { name?: string })?.name === "RenderingCancelledException") return;
          throw e;
        } finally {
          if (inflight === task) inflight = null;
        }
      };

      const result = chain.then(run, run);
      chain = result.catch(() => {}); // 渲染失败不打断后续排队
      return result;
    },
    async getTextLayout(pageNum: number, cssWidth: number): Promise<PositionedText[]> {
      const page = await doc.getPage(pageNum);
      const base = page.getViewport({ scale: 1 });
      const scale = cssWidth / base.width;
      const viewport = page.getViewport({ scale });
      const content = await page.getTextContent();

      const out: PositionedText[] = [];
      for (const item of content.items) {
        if (!("str" in item) || !item.str) continue;
        // item.transform 是 PDF 用户空间的文字矩阵；乘上视口矩阵得到
        // CSS 像素空间的基线位置与字形高度。
        const tx = pdfjs.Util.transform(viewport.transform, item.transform);
        const fontHeight = Math.hypot(tx[2], tx[3]);
        out.push({
          str: item.str,
          left: tx[4],
          top: tx[5] - fontHeight,
          width: item.width * scale,
          height: fontHeight || item.height * scale,
        });
      }
      return out;
    },
    destroy() {
      renderSeq += 1; // 作废所有排队中的渲染
      if (inflight) inflight.cancel();
      void doc.destroy();
    },
  };
}
