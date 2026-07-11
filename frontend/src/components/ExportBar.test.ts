import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ExportBar from './ExportBar.vue'

const baseProps = {
  outputPath: '~/Desktop/提取结果.xlsx',
  format: 'xlsx' as const,
  exporting: false,
  done: null,
}

describe('ExportBar', () => {
  it('显示保存路径，点击导出 emit export', async () => {
    const wrapper = mount(ExportBar, { props: baseProps })

    expect(wrapper.text()).toContain('~/Desktop/提取结果.xlsx')

    await wrapper.find('.btn-export').trigger('click')
    expect(wrapper.emitted('export')).toBeTruthy()
  })

  it('未选择路径时提示选择，点击 emit choose-path', async () => {
    const wrapper = mount(ExportBar, { props: { ...baseProps, outputPath: '' } })

    expect(wrapper.text()).toContain('尚未选择保存位置')
    await wrapper.find('.chg').trigger('click')
    expect(wrapper.emitted('choose-path')).toBeTruthy()
  })

  it('点击格式角标弹出格式菜单并 emit update:format，不触发 export', async () => {
    const wrapper = mount(ExportBar, { props: baseProps })

    await wrapper.find('.fmt').trigger('click')
    expect(wrapper.emitted('export')).toBeFalsy()
    expect(wrapper.find('.fmt-pop').exists()).toBe(true)

    const items = wrapper.findAll('.fmt-pop button')
    expect(items).toHaveLength(3)
    await items[1].trigger('click')

    const events = wrapper.emitted('update:format')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('csv')
  })

  it('导出中禁用按钮', () => {
    const wrapper = mount(ExportBar, { props: { ...baseProps, exporting: true } })
    expect(wrapper.find('.btn-export').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('导出中')
  })

  it('导出成功条渲染并支持打开文件与关闭', async () => {
    const wrapper = mount(ExportBar, {
      props: { ...baseProps, done: { path: '/tmp/out.xlsx', count: 2, edited: true } },
    })

    expect(wrapper.text()).toContain('已导出 2 条记录（含人工修正）')
    expect(wrapper.text()).toContain('/tmp/out.xlsx')

    await wrapper.find('.open').trigger('click')
    expect(wrapper.emitted('open-file')).toBeTruthy()

    await wrapper.find('.export-done .x').trigger('click')
    expect(wrapper.emitted('dismiss-done')).toBeTruthy()
  })
})
