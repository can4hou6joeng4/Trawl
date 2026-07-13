import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DocPane from './DocPane.vue'

const pdfMocks = vi.hoisted(() => {
  const renderPage = vi.fn(async (_page: number, _canvas: HTMLCanvasElement, _width: number) => {})
  const destroy = vi.fn()
  const getTextLayout = vi.fn(async (_page: number, _width: number) => [] as { str: string; left: number; top: number; width: number; height: number }[])
  return {
    renderPage,
    destroy,
    getTextLayout,
    loadPdf: vi.fn(async (_data: string) => ({ numPages: 12, renderPage, destroy, getTextLayout })),
  }
})

vi.mock('../utils/pdfDoc', () => ({
  loadPdf: pdfMocks.loadPdf,
}))

const flush = () => new Promise(resolve => setTimeout(resolve, 0))

const baseProps = {
  fileName: '起诉状_张三借款纠纷.pdf',
  kind: 'docx' as const,
  docData: null,
  currentPage: 1,
  totalPages: 12,
  recordPages: [1, 3],
  recordsOnPage: [{ index: 0, label: 'REC 01' }],
}

beforeEach(() => {
  pdfMocks.loadPdf.mockClear()
  pdfMocks.renderPage.mockClear()
  pdfMocks.destroy.mockClear()
  pdfMocks.getTextLayout.mockClear()
  pdfMocks.getTextLayout.mockResolvedValue([])
  pdfMocks.loadPdf.mockImplementation(async () => ({
    numPages: 12,
    renderPage: pdfMocks.renderPage,
    destroy: pdfMocks.destroy,
    getTextLayout: pdfMocks.getTextLayout,
  }))
})

describe('DocPane 导航', () => {
  it('渲染页码与文件名', () => {
    const wrapper = mount(DocPane, { props: baseProps })
    expect(wrapper.text()).toContain('第 1 / 12 页')
    expect(wrapper.text()).toContain('起诉状_张三借款纠纷.pdf')
  })

  it('第一页时上一页按钮禁用，点击下一页 emit update:currentPage', async () => {
    const wrapper = mount(DocPane, { props: baseProps })
    const navs = wrapper.findAll('.nav')

    expect(navs[0].attributes('disabled')).toBeDefined()
    await navs[1].trigger('click')

    const events = wrapper.emitted('update:currentPage')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe(2)
  })

  it('末页时下一页按钮禁用', () => {
    const wrapper = mount(DocPane, { props: { ...baseProps, currentPage: 12 } })
    const navs = wrapper.findAll('.nav')
    expect(navs[1].attributes('disabled')).toBeDefined()
  })

  it('页点标记有记录的页，点击页点跳页', async () => {
    const wrapper = mount(DocPane, { props: baseProps })
    const dots = wrapper.findAll('.pdot')

    expect(dots).toHaveLength(12)
    expect(dots[0].classes()).toContain('has')
    expect(dots[2].classes()).toContain('has')
    expect(dots[1].classes()).not.toContain('has')

    await dots[4].trigger('click')
    const events = wrapper.emitted('update:currentPage')
    expect(events![0][0]).toBe(5)
  })

  it('点击页面上的记录角标 emit locate', async () => {
    const wrapper = mount(DocPane, { props: baseProps })
    await wrapper.find('.page-rec').trigger('click')

    const events = wrapper.emitted('locate')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe(0)
  })

  it('点击关闭 emit close', async () => {
    const wrapper = mount(DocPane, { props: baseProps })
    await wrapper.find('.back').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})

describe('DocPane 渲染模式', () => {
  it('docx 显示占位页与无记录提示', () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, currentPage: 2, recordsOnPage: [] },
    })
    expect(wrapper.find('.pl.t').exists()).toBe(true)
    expect(wrapper.text()).toContain('本页无匹配记录')
    expect(pdfMocks.loadPdf).not.toHaveBeenCalled()
  })

  it('pdf 模式加载文档：emit loaded 真实页数并渲染当前页', async () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA' },
    })
    await flush()

    expect(pdfMocks.loadPdf).toHaveBeenCalledWith('AAAA')
    const loaded = wrapper.emitted('loaded')
    expect(loaded).toBeTruthy()
    expect(loaded![0][0]).toBe(12)
    expect(pdfMocks.renderPage).toHaveBeenCalled()
    expect(pdfMocks.renderPage.mock.calls[0][0]).toBe(1)
  })

  it('pdf 翻页时重新渲染对应页', async () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA' },
    })
    await flush()
    pdfMocks.renderPage.mockClear()

    await wrapper.setProps({ currentPage: 3 })
    await flush()

    expect(pdfMocks.renderPage).toHaveBeenCalled()
    expect(pdfMocks.renderPage.mock.calls[0][0]).toBe(3)
  })

  it('pdf 解析失败回退占位页并提示', async () => {
    pdfMocks.loadPdf.mockRejectedValueOnce(new Error('bad pdf'))
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'BROKEN' },
    })
    await flush()

    expect(wrapper.emitted('loaded')).toBeFalsy()
    expect(wrapper.text()).toContain('原文渲染失败')
    expect(wrapper.find('.pl.t').exists()).toBe(true)
  })

  it('单页渲染失败后翻页成功即自愈', async () => {
    pdfMocks.renderPage.mockRejectedValueOnce(new Error('canvas boom'))
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA' },
    })
    await flush()
    expect(wrapper.text()).toContain('原文渲染失败')

    await wrapper.setProps({ currentPage: 2 })
    await flush()
    expect(wrapper.text()).not.toContain('原文渲染失败')
  })

  it('image 模式按扩展名生成 data URI 直接显示', () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, fileName: '证据.PNG', kind: 'image', docData: 'IMGDATA' },
    })
    const img = wrapper.find('.doc-image')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('data:image/png;base64,IMGDATA')
    expect(pdfMocks.loadPdf).not.toHaveBeenCalled()
  })

  it('卸载时销毁 pdf 文档', async () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA' },
    })
    await flush()

    wrapper.unmount()
    expect(pdfMocks.destroy).toHaveBeenCalled()
  })
})

describe('DocPane 字段高亮', () => {
  const targets = [
    {
      recIndex: 1,
      fields: [{ key: 'defendant', label: '被告', value: '张三' }],
    },
  ]

  it('文本层命中时渲染高亮框与字段标签，点击 emit locate', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([
      { str: '被告：张三，男', left: 50, top: 100, width: 70, height: 14 },
    ])
    const wrapper = mount(DocPane, {
      props: {
        ...baseProps,
        kind: 'pdf',
        docData: 'AAAA',
        highlightTargets: targets,
        recordsOnPage: [
          { index: 0, label: 'REC 01' },
          { index: 1, label: 'REC 02' },
        ],
      },
    })
    await flush()
    await flush()

    const hl = wrapper.find('.field-hl')
    expect(hl.exists()).toBe(true)
    expect(wrapper.find('.field-hl .tag').text()).toBe('被告')
    // 角标按记录粒度退位：已高亮的记录 1 隐去角标，未匹配的记录 0 保留定位入口
    const recLabels = wrapper.findAll('.page-rec').map(b => b.text())
    expect(recLabels).toEqual(['REC 01'])

    await hl.trigger('click')
    const events = wrapper.emitted('locate')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe(1)
  })

  it('flashRecIndex 命中记录时高亮框带闪烁类', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([
      { str: '被告：张三，男', left: 50, top: 100, width: 70, height: 14 },
    ])
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA', highlightTargets: targets, flashRecIndex: 1 },
    })
    await flush()
    await flush()

    expect(wrapper.find('.field-hl').classes()).toContain('flash')
  })

  it('无文本层（扫描件）时无高亮，REC 角标保留为定位入口', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([])
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA', highlightTargets: targets },
    })
    await flush()
    await flush()

    expect(wrapper.find('.field-hl').exists()).toBe(false)
    expect(wrapper.find('.page-rec').exists()).toBe(true)
  })

  it('翻页立即清空旧页高亮，不等新文本层返回', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([
      { str: '被告：张三，男', left: 50, top: 100, width: 70, height: 14 },
    ])
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA', highlightTargets: targets },
    })
    await flush()
    await flush()
    expect(wrapper.find('.field-hl').exists()).toBe(true)

    // 新页文本层悬而不决，旧高亮也必须先消失（避免残留在新页画布上）
    pdfMocks.getTextLayout.mockImplementation(() => new Promise(() => {}))
    await wrapper.setProps({ currentPage: 2 })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.field-hl').exists()).toBe(false)
  })

  it('渲染失败时清空高亮，REC 角标恢复为占位页的定位入口', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([
      { str: '被告：张三，男', left: 50, top: 100, width: 70, height: 14 },
    ])
    const wrapper = mount(DocPane, {
      props: {
        ...baseProps,
        kind: 'pdf',
        docData: 'AAAA',
        highlightTargets: targets,
        recordsOnPage: [{ index: 1, label: 'REC 02' }],
      },
    })
    await flush()
    await flush()
    expect(wrapper.find('.field-hl').exists()).toBe(true)
    expect(wrapper.find('.page-rec').exists()).toBe(false)

    pdfMocks.renderPage.mockRejectedValueOnce(new Error('boom'))
    await wrapper.setProps({ currentPage: 2 })
    await flush()
    await flush()

    expect(wrapper.find('.field-hl').exists()).toBe(false)
    expect(wrapper.find('.page-rec').exists()).toBe(true) // 占位页上兜底入口不被旧高亮抑制
  })

  it('页首行的高亮角标翻转到矩形下方，避免被页卡上缘裁掉', async () => {
    pdfMocks.getTextLayout.mockResolvedValue([
      { str: '被告：张三，男', left: 50, top: 6, width: 70, height: 14 },
    ])
    const wrapper = mount(DocPane, {
      props: { ...baseProps, kind: 'pdf', docData: 'AAAA', highlightTargets: targets },
    })
    await flush()
    await flush()

    expect(wrapper.find('.field-hl .tag').classes()).toContain('below')
  })
})
