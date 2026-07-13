import { describe, it, expect } from 'vitest'
import { matchValueRects, computeFieldHighlights } from './pdfHighlights'
import type { PositionedText } from './pdfDoc'

// 便捷构造：一行文本项（每字符宽 10px、高 14px）
function item(str: string, left: number, top: number): PositionedText {
  return { str, left, top, width: str.length * 10, height: 14 }
}

describe('matchValueRects', () => {
  it('单项内精确匹配，按字符宽度均摊偏移', () => {
    const items = [item('被告：张三，男', 50, 100)]
    const rects = matchValueRects(items, '张三', 'defendant')

    expect(rects).toHaveLength(1)
    // "被告：" 占 3 字符 → 张三 从 50+30 开始，宽 2 字符
    expect(rects[0]).toEqual({ left: 80, top: 100, width: 20, height: 14 })
  })

  it('值跨越多个文本项时合并为一行矩形（混排行按视觉宽度加权）', () => {
    // 混排项按真实字形宽度构造：全宽 20px、半角 10px
    const a = { str: '身份证号码：1101', left: 0, top: 40, width: 6 * 20 + 4 * 10, height: 14 }
    const b = { str: '0119900101', left: 160, top: 40, width: 100, height: 14 }
    const c = { str: '1234，住址', left: 260, top: 40, width: 4 * 10 + 3 * 20, height: 14 }
    const rects = matchValueRects([a, b, c], '110101199001011234')

    expect(rects).toHaveLength(1)
    expect(rects[0].left).toBe(120) // "身份证号码："6 个全宽字符 = 120px 后
    expect(rects[0].left + rects[0].width).toBe(300) // 到第三项的 "1234"（4 个半角）结束
  })

  it('空白差异不影响匹配（提取值带空格，PDF 无空格）', () => {
    const items = [item('偿还借款10000元。', 0, 0)]
    const rects = matchValueRects(items, '偿还借款 10000 元。')
    expect(rects).toHaveLength(1)
    expect(rects[0].width).toBe(110) // 11 个非空白字符（偿还借款10000元。）
  })

  it('跨行值按行聚类为多个矩形', () => {
    const items = [item('判令被告偿还借款', 20, 100), item('并承担诉讼费用。', 20, 130)]
    const rects = matchValueRects(items, '判令被告偿还借款并承担诉讼费用。')

    expect(rects).toHaveLength(2)
    expect(rects[0].top).toBe(100)
    expect(rects[1].top).toBe(130)
  })

  it('defendant 优先带上下文前缀匹配，避免同名误中', () => {
    const items = [item('原告：张三丰委托代理', 0, 0), item('被告：张三，男', 0, 30)]
    const rects = matchValueRects(items, '张三', 'defendant')

    expect(rects).toHaveLength(1)
    expect(rects[0].top).toBe(30) // 命中的是"被告："后面那个
    expect(rects[0].left).toBe(30)
  })

  it('长字段整串不在时退化为前缀锚定+后缀扩展', () => {
    // 页面文字与值的中段不一致（OCR 归并差异），但前 20 字与后 10 字都在
    const pageText = '2024年1月1日，被告向原告借款10000元，约定六月一日前归还。到期后经多次催告仍未偿还。'
    const value = '2024年1月1日，被告向原告借款10000元，约定2024年6月1日前归还。到期后经多次催告仍未偿还。'
    const items = [item(pageText, 0, 0)]
    const rects = matchValueRects(items, value)

    expect(rects).toHaveLength(1)
    expect(rects[0].left).toBe(0)
    // 至少覆盖到后缀"到期后经多次催告仍未偿还。"的结尾
    expect(rects[0].left + rects[0].width).toBe(pageText.length * 10)
  })

  it('页面无匹配（扫描件无文本层）返回空数组', () => {
    expect(matchValueRects([], '张三')).toEqual([])
    expect(matchValueRects([item('完全无关的内容', 0, 0)], '张三')).toEqual([])
  })

  it('NFKC 折叠：文本层的康熙部首形字符可与常规汉字匹配', () => {
    // 浏览器生成的 PDF（及部分真实 PDF）ToUnicode 常吐部首形码位：⾝(U+2FBD)/⼀(U+2F00)/⽉(U+2F49)
    const items = [item('⾝份证号码：110101199001011234', 0, 0)]
    const rects = matchValueRects(items, '身份证号码：110101199001011234')

    expect(rects).toHaveLength(1)
    expect(rects[0].left).toBe(0)
    expect(rects[0].width).toBe(240) // 24 个字符全部命中

    const long = [item('⼀、判令被告偿还借款10000元；', 0, 0), item('⼆、判令被告承担诉讼费⽤。', 0, 30)]
    expect(matchValueRects(long, '一、判令被告偿还借款10000元；二、判令被告承担诉讼费用。')).toHaveLength(2)
  })

  it('NFKC 折叠：全角数字/标点可与半角提取值匹配', () => {
    const items = [item('借款１００００元', 0, 0)]
    expect(matchValueRects(items, '借款10000元')).toHaveLength(1)
  })

  it('增补平面字符按码点折叠与匹配（代理对不拆散）', () => {
    // 𝟏 (U+1D7CF 数学粗体数字) NFKC 折叠为 1；𠮷 (U+20BB7) 无分解、原样匹配
    expect(matchValueRects([item('借款𝟏0000元', 0, 0)], '借款10000元')).toHaveLength(1)
    expect(matchValueRects([item('被告：吕𠮷', 0, 0)], '吕𠮷', 'defendant')).toHaveLength(1)
  })

  it('空值返回空数组', () => {
    expect(matchValueRects([item('被告：张三', 0, 0)], '')).toEqual([])
    expect(matchValueRects([item('被告：张三', 0, 0)], '   ')).toEqual([])
  })
})

describe('computeFieldHighlights', () => {
  it('批量计算多记录多字段，跳过无值与未命中字段', () => {
    const items = [item('被告：张三，男', 0, 0), item('身份证号码：110101199001011234', 0, 30)]
    const result = computeFieldHighlights(items, [
      {
        recIndex: 0,
        fields: [
          { key: 'defendant', label: '被告', value: '张三' },
          { key: 'idNumber', label: '身份证号码', value: '110101199001011234' },
          { key: 'request', label: '诉讼请求', value: '' },
          { key: 'factsReason', label: '事实与理由', value: '本页不存在的内容xyz' },
        ],
      },
    ])

    expect(result.map(h => h.key)).toEqual(['defendant', 'idNumber'])
    expect(result[0].recIndex).toBe(0)
    expect(result[0].label).toBe('被告')
    expect(result[1].rects[0].top).toBe(30)
  })

  it('同页两条同名被告向后认领：各自高亮各自出现处', () => {
    const items = [item('被告：张三，男', 0, 0), item('被告：张三，女', 0, 200)]
    const result = computeFieldHighlights(items, [
      { recIndex: 0, fields: [{ key: 'defendant', label: '被告', value: '张三' }] },
      { recIndex: 1, fields: [{ key: 'defendant', label: '被告', value: '张三' }] },
    ])

    expect(result).toHaveLength(2)
    expect(result[0].rects[0].top).toBe(0)
    expect(result[1].rects[0].top).toBe(200) // 不再叠在第一处
  })

  it('长字段前缀锚定不落入前一条记录已认领的区间', () => {
    // 两条 request 前 20 归一化字符相同；记录 2 的值与页面有 OCR 差异（元 vs 元整），走锚定路径
    const rec1Text = '一、判令被告立即向原告支付拖欠货款人民币50000元；二、诉讼费用由被告承担。'
    const rec2Text = '一、判令被告立即向原告支付拖欠货款人民币80000元；二、诉讼费用由被告承担。'
    const items = [item(rec1Text, 0, 0), item(rec2Text, 0, 100)]
    const result = computeFieldHighlights(items, [
      { recIndex: 0, fields: [{ key: 'request', label: '诉讼请求', value: rec1Text }] },
      {
        recIndex: 1,
        fields: [
          { key: 'request', label: '诉讼请求', value: '一、判令被告立即向原告支付拖欠货款人民币80000元整；二、诉讼费用由被告承担。' },
        ],
      },
    ])

    expect(result).toHaveLength(2)
    expect(result[0].rects[0].top).toBe(0)
    expect(result[1].rects[0].top).toBe(100) // 锚定在记录 2 自己的文字上
  })

  it('页面只出现一次时后续记录回退共享首处', () => {
    const items = [item('被告：张三，男', 0, 0)]
    const result = computeFieldHighlights(items, [
      { recIndex: 0, fields: [{ key: 'defendant', label: '被告', value: '张三' }] },
      { recIndex: 1, fields: [{ key: 'defendant', label: '被告', value: '张三' }] },
    ])

    expect(result).toHaveLength(2)
    expect(result[0].rects[0].top).toBe(0)
    expect(result[1].rects[0].top).toBe(0) // 原文确实只有一处，两条记录共享
  })
})
