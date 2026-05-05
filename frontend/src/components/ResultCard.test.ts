import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ResultCard from './ResultCard.vue'

// vi.mock 被 hoist 到文件顶，所以 api 对象必须通过 vi.hoisted 同级提升构造，
// 否则 mock 工厂里引用的变量还未初始化。
const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    isDesktop: false,
    service: {
      openFile: vi.fn().mockResolvedValue(undefined),
      selectFile: vi.fn().mockResolvedValue(null),
    },
  },
}))

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  mockApi.service.openFile.mockClear()
  mockApi.service.openFile.mockResolvedValue(undefined)
})

describe('ResultCard', () => {
  it('result === null 时不渲染主容器', () => {
    const wrapper = mount(ResultCard, { props: { result: null } })
    expect(wrapper.find('.result-card').exists()).toBe(false)
  })

  it('成功结果渲染「提取成功」标题与 recordCount', () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 7,
          outputPath: '/tmp/out.xlsx',
          errorMessage: '',
        },
      },
    })
    expect(wrapper.find('.result-card').exists()).toBe(true)
    expect(wrapper.find('.result-card').classes()).not.toContain('error')
    expect(wrapper.text()).toContain('提取成功')
    expect(wrapper.text()).toContain('7')
  })

  it('失败结果渲染 errorMessage 与 error class', () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: false,
          recordCount: 0,
          outputPath: '',
          errorMessage: '解析失败：未识别到当事人',
        },
      },
    })
    expect(wrapper.find('.result-card').classes()).toContain('error')
    expect(wrapper.text()).toContain('提取失败')
    expect(wrapper.text()).toContain('解析失败：未识别到当事人')
  })

  it('点击 outputPath 调用 api.service.openFile 一次', async () => {
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 1,
          outputPath: '/tmp/result.xlsx',
          errorMessage: '',
        },
      },
    })
    await wrapper.find('.clickable-path').trigger('click')
    expect(mockApi.service.openFile).toHaveBeenCalledTimes(1)
    expect(mockApi.service.openFile).toHaveBeenCalledWith('/tmp/result.xlsx')
  })

  it('openFile 抛错时 emit notification 含「无法打开文件」', async () => {
    mockApi.service.openFile.mockRejectedValueOnce(new Error('boom'))
    const wrapper = mount(ResultCard, {
      props: {
        result: {
          success: true,
          recordCount: 1,
          outputPath: '/tmp/x.xlsx',
          errorMessage: '',
        },
      },
    })
    await wrapper.find('.clickable-path').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))
    const events = wrapper.emitted('notification')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('无法打开文件')
    expect(events![0][1]).toBe('error')
  })
})
