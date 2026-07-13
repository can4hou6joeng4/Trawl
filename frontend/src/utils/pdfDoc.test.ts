import { describe, it, expect, vi, beforeEach } from 'vitest'

/**
 * pdfDoc 渲染调度测试：验证快速翻页下的串行化与过期请求跳过。
 * pdfjs-dist 与 worker 资源均被 mock，getPage 用可控 deferred 制造异步空窗。
 */

vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?url', () => ({ default: 'worker-url' }))

type Deferred<T> = { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void }
function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const state = vi.hoisted(() => ({
  renderCalls: [] as { page: number; cancelled: boolean }[],
  renderingNow: 0,
  maxConcurrent: 0,
  getPageGates: new Map<number, { promise: Promise<void>; resolve: (v: void) => void; reject: (e: unknown) => void }>(),
  renderShouldReject: false,
  textItems: [] as unknown[],
}))

function makePage(num: number) {
  return {
    getViewport: ({ scale }: { scale: number }) => ({ width: 595 * scale, height: 842 * scale, transform: [scale, 0, 0, scale, 0, 0] }),
    getTextContent: async () => ({ items: state.textItems }),
    render: () => {
      state.renderingNow += 1
      state.maxConcurrent = Math.max(state.maxConcurrent, state.renderingNow)
      const call = { page: num, cancelled: false }
      state.renderCalls.push(call)
      const done = new Promise<void>((resolve, reject) => {
        setTimeout(() => {
          state.renderingNow -= 1
          if (state.renderShouldReject) reject(new Error('render boom'))
          else resolve()
        }, 0)
      })
      return { promise: done, cancel: () => { call.cancelled = true } }
    },
  }
}

vi.mock('pdfjs-dist', () => ({
  GlobalWorkerOptions: {} as { workerSrc?: string },
  Util: { transform: (_a: number[], b: number[]) => b },
  getDocument: () => ({
    promise: Promise.resolve({
      numPages: 5,
      getPage: (n: number) => {
        const gate = state.getPageGates.get(n)
        if (gate) return gate.promise.then(() => makePage(n))
        return Promise.resolve(makePage(n))
      },
      destroy: async () => {},
    }),
  }),
}))

import { loadPdf } from './pdfDoc'

function makeCanvas(): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  Object.defineProperty(canvas, 'getContext', { value: () => ({}) })
  return canvas
}

const flush = () => new Promise(resolve => setTimeout(resolve, 5))

beforeEach(() => {
  state.renderCalls.length = 0
  state.renderingNow = 0
  state.maxConcurrent = 0
  state.getPageGates.clear()
  state.renderShouldReject = false
  state.textItems = []
})

describe('pdfDoc 渲染调度', () => {
  it('顺序翻页逐页渲染且同一 canvas 永不并发', async () => {
    const doc = await loadPdf('AAAA')
    const canvas = makeCanvas()

    await doc.renderPage(1, canvas, 340)
    await doc.renderPage(2, canvas, 340)

    expect(state.renderCalls.map(c => c.page)).toEqual([1, 2])
    expect(state.maxConcurrent).toBe(1)
    doc.destroy()
  })

  it('getPage 空窗内的快速翻页：过期请求被跳过，只渲染最后一页', async () => {
    const doc = await loadPdf('AAAA')
    const canvas = makeCanvas()

    // 卡住第 2 页的 getPage，制造异步空窗
    const gate2 = deferred<void>()
    state.getPageGates.set(2, gate2)

    const p2 = doc.renderPage(2, canvas, 340)
    await flush() // 让请求 2 进入 getPage 等待
    const p3 = doc.renderPage(3, canvas, 340)

    gate2.resolve() // 第 2 页的 getPage 此时才返回
    await Promise.all([p2, p3])
    await flush()

    // 第 2 页请求已过期：不渲染；只有第 3 页真正落到 canvas
    expect(state.renderCalls.map(c => c.page)).toEqual([3])
    expect(state.maxConcurrent).toBe(1)
    doc.destroy()
  })

  it('连续同步翻页只渲染最终页', async () => {
    const doc = await loadPdf('AAAA')
    const canvas = makeCanvas()

    const ps = [doc.renderPage(1, canvas, 340), doc.renderPage(2, canvas, 340), doc.renderPage(3, canvas, 340)]
    await Promise.all(ps)
    await flush()

    expect(state.renderCalls.map(c => c.page)).toEqual([3])
    doc.destroy()
  })

  it('destroy 作废排队中的渲染', async () => {
    const doc = await loadPdf('AAAA')
    const canvas = makeCanvas()

    const gate1 = deferred<void>()
    state.getPageGates.set(1, gate1)
    const p = doc.renderPage(1, canvas, 340)

    doc.destroy()
    gate1.resolve()
    await p
    await flush()

    expect(state.renderCalls).toHaveLength(0)
  })

  it('渲染失败向上抛出且不阻塞后续渲染', async () => {
    const doc = await loadPdf('AAAA')
    const canvas = makeCanvas()

    state.renderShouldReject = true
    await expect(doc.renderPage(1, canvas, 340)).rejects.toThrow('render boom')

    state.renderShouldReject = false
    await doc.renderPage(2, canvas, 340)
    expect(state.renderCalls.map(c => c.page)).toEqual([1, 2])
    doc.destroy()
  })
})

describe('pdfDoc 文本层', () => {
  it('getTextLayout 将文本项换算为 CSS 像素坐标，跳过无字项', async () => {
    state.textItems = [
      { str: '被告：张三', transform: [12, 0, 0, 12, 50, 700], width: 60, height: 12 },
      { type: 'beginMarkedContent' }, // 无 str 的标记项：跳过
      { str: '', transform: [12, 0, 0, 12, 0, 0], width: 0, height: 12 }, // 空串：跳过
    ]
    const doc = await loadPdf('AAAA')
    // cssWidth 297.5 → scale 0.5（基准页宽 595）
    const items = await doc.getTextLayout(1, 297.5)

    expect(items).toHaveLength(1)
    // mock 的 Util.transform 原样返回文字矩阵：字形高 12，基线 y=700 → top=688
    expect(items[0]).toEqual({ str: '被告：张三', left: 50, top: 688, width: 30, height: 12 })
    doc.destroy()
  })

  it('getTextLayout 字形高度为 0 时回退 item.height×scale', async () => {
    state.textItems = [{ str: 'X', transform: [12, 0, 0, 0, 10, 20], width: 10, height: 14 }]
    const doc = await loadPdf('AAAA')
    const items = await doc.getTextLayout(1, 595) // scale 1

    expect(items).toHaveLength(1)
    expect(items[0].height).toBe(14)
    expect(items[0].top).toBe(20)
    doc.destroy()
  })

  it('getTextLayout 空文本层（扫描件）返回空数组', async () => {
    state.textItems = []
    const doc = await loadPdf('AAAA')
    await expect(doc.getTextLayout(1, 340)).resolves.toEqual([])
    doc.destroy()
  })
})
