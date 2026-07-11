import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DocPane from './DocPane.vue'

const baseProps = {
  fileName: '起诉状_张三借款纠纷.pdf',
  currentPage: 1,
  totalPages: 12,
  recordPages: [1, 3],
  recordsOnPage: [{ index: 0, label: 'REC 01' }],
}

describe('DocPane', () => {
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

  it('无记录页显示提示', () => {
    const wrapper = mount(DocPane, {
      props: { ...baseProps, currentPage: 2, recordsOnPage: [] },
    })
    expect(wrapper.text()).toContain('本页无匹配记录')
  })

  it('点击关闭 emit close', async () => {
    const wrapper = mount(DocPane, { props: baseProps })
    await wrapper.find('.back').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
