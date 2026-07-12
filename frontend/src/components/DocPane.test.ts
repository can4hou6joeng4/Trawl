import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DocPane from './DocPane.vue'

const pdfMocks = vi.hoisted(() => {
  const renderPage = vi.fn(async (_page: number, _canvas: HTMLCanvasElement, _width: number) => {})
  const destroy = vi.fn()
  return {
    renderPage,
    destroy,
    loadPdf: vi.fn(async (_data: string) => ({ numPages: 12, renderPage, destroy })),
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
  pdfMocks.loadPdf.mockImplementation(async () => ({
    numPages: 12,
    renderPage: pdfMocks.renderPage,
    destroy: pdfMocks.destroy,
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
