import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfigPanel from './ConfigPanel.vue'

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    service: {
      scanFields: vi.fn().mockResolvedValue([]),
      selectOutputPath: vi.fn().mockResolvedValue(''),
    },
  },
}))

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  mockApi.service.scanFields.mockClear()
  mockApi.service.scanFields.mockResolvedValue([])
  mockApi.service.selectOutputPath.mockClear()
  mockApi.service.selectOutputPath.mockResolvedValue('')
})

describe('ConfigPanel', () => {
  const baseProps = {
    selectedFile: '/tmp/case.pdf',
    fileName: 'case.pdf',
    selectedFormat: 'xlsx' as const,
    outputPath: '',
    isLoading: false,
    isDisabled: false,
    selectedFields: [] as string[],
  }

  it('字段扫描中显示加载提示，扫描完成后未选择字段时禁用预览与提取', async () => {
    vi.useFakeTimers()
    const wrapper = mount(ConfigPanel, { props: baseProps })

    expect(wrapper.text()).toContain('正在加载可提取字段')
    await vi.runAllTimersAsync()

    expect(wrapper.text()).toContain('请至少保留 1 个提取字段')
    expect(wrapper.find('.btn-secondary').attributes('disabled')).toBeDefined()
    expect(wrapper.find('.btn-primary').attributes('disabled')).toBeDefined()
    vi.useRealTimers()
  })

  it('字段与导出位置就绪时允许预览和提取', async () => {
    vi.useFakeTimers()
    const wrapper = mount(ConfigPanel, {
      props: {
        ...baseProps,
        selectedFields: ['defendant', 'idNumber'],
        outputPath: '/tmp/out.xlsx',
      },
    })

    await vi.runAllTimersAsync()

    expect(wrapper.text()).toContain('准备就绪')
    expect(wrapper.find('.btn-secondary').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('.btn-primary').attributes('disabled')).toBeUndefined()
    vi.useRealTimers()
  })
})
