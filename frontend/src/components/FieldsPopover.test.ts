import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FieldsPopover from './FieldsPopover.vue'

const options = [
  { key: 'defendant', label: '被告' },
  { key: 'idNumber', label: '身份证号码' },
  { key: 'request', label: '诉讼请求' },
  { key: 'factsReason', label: '事实与理由' },
]

describe('FieldsPopover', () => {
  it('按钮显示已选比例，点击展开弹层', async () => {
    const wrapper = mount(FieldsPopover, {
      props: { options, selected: ['defendant', 'idNumber'] },
    })

    expect(wrapper.find('.fields-btn').text()).toContain('2/4')
    expect(wrapper.find('.fields-pop').exists()).toBe(false)

    await wrapper.find('.fields-btn').trigger('click')
    expect(wrapper.find('.fields-pop').exists()).toBe(true)
    expect(wrapper.findAll('label')).toHaveLength(4)
  })

  it('取消勾选 emit 去除该字段的列表', async () => {
    const wrapper = mount(FieldsPopover, {
      props: { options, selected: ['defendant', 'idNumber'] },
    })
    await wrapper.find('.fields-btn').trigger('click')

    await wrapper.findAll('label')[0].trigger('click')

    const events = wrapper.emitted('update:selected')
    expect(events).toBeTruthy()
    expect(events![0][0]).toEqual(['idNumber'])
  })

  it('勾选新增字段 emit 包含新字段的列表', async () => {
    const wrapper = mount(FieldsPopover, {
      props: { options, selected: ['defendant'] },
    })
    await wrapper.find('.fields-btn').trigger('click')

    await wrapper.findAll('label')[2].trigger('click')

    const events = wrapper.emitted('update:selected')
    expect(events![0][0]).toEqual(['defendant', 'request'])
  })

  it('仅剩 1 个字段时不允许取消勾选', async () => {
    const wrapper = mount(FieldsPopover, {
      props: { options, selected: ['defendant'] },
    })
    await wrapper.find('.fields-btn').trigger('click')

    await wrapper.findAll('label')[0].trigger('click')

    expect(wrapper.emitted('update:selected')).toBeFalsy()
  })
})
