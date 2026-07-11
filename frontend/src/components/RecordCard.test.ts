import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import RecordCard from './RecordCard.vue'

const fields = [
  { key: 'defendant', label: '被告', kind: 'text' as const },
  { key: 'idNumber', label: '身份证号码', kind: 'num' as const },
  { key: 'request', label: '诉讼请求', kind: 'long' as const },
]

const record = {
  page: '1',
  defendant: '张三',
  idNumber: '110101199001011234',
  request: '一、判令被告偿还借款 10000 元。',
}

const baseProps = {
  index: 0,
  record,
  fields,
  open: true,
  edited: false,
  flash: false,
}

describe('RecordCard', () => {
  it('展开时按字段渲染可编辑输入', () => {
    const wrapper = mount(RecordCard, { props: baseProps })

    expect(wrapper.text()).toContain('REC 01')
    expect(wrapper.text()).toContain('P.1')
    const inputs = wrapper.findAll('input')
    expect(inputs).toHaveLength(2) // text + num
    expect(wrapper.findAll('textarea')).toHaveLength(1) // long
    expect((inputs[0].element as HTMLInputElement).value).toBe('张三')
  })

  it('编辑输入 emit edit(key, value)', async () => {
    const wrapper = mount(RecordCard, { props: baseProps })

    await wrapper.find('input').setValue('张三（更正）')

    const events = wrapper.emitted('edit')
    expect(events).toBeTruthy()
    expect(events![0]).toEqual(['defendant', '张三（更正）'])
  })

  it('折叠时显示摘要，点击头部 emit toggle', async () => {
    const wrapper = mount(RecordCard, { props: { ...baseProps, open: false } })

    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('.sum').text()).toBe('张三')

    await wrapper.find('.rec-head').trigger('click')
    expect(wrapper.emitted('toggle')).toBeTruthy()
  })

  it('已修正标记按 edited 属性显示', () => {
    const clean = mount(RecordCard, { props: baseProps })
    expect(clean.find('.edited').exists()).toBe(false)

    const dirty = mount(RecordCard, { props: { ...baseProps, edited: true } })
    expect(dirty.find('.edited').text()).toContain('已修正')
  })

  it('点击定位原文 emit locate 且不触发 toggle', async () => {
    const wrapper = mount(RecordCard, { props: baseProps })

    await wrapper.find('.loc').trigger('click')

    expect(wrapper.emitted('locate')).toBeTruthy()
    expect(wrapper.emitted('toggle')).toBeFalsy()
  })
})
