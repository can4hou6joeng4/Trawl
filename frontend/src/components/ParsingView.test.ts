import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ParsingView from './ParsingView.vue'

const baseProps = {
  fileName: '起诉状_张三借款纠纷.pdf',
  percent: 42,
  message: '正在按页匹配法律实体...',
  etaLabel: '约 8 秒',
  isCancelling: false,
}

describe('ParsingView', () => {
  it('渲染文件名、进度与提示信息', () => {
    const wrapper = mount(ParsingView, { props: baseProps })

    expect(wrapper.text()).toContain('起诉状_张三借款纠纷.pdf')
    expect(wrapper.text()).toContain('42%')
    expect(wrapper.text()).toContain('约 8 秒')
    expect(wrapper.text()).toContain('正在按页匹配法律实体...')
    expect(wrapper.find('.parse-fill').attributes('style')).toContain('width: 42%')
  })

  it('点击停止解析 emit cancel；取消中禁用按钮', async () => {
    const wrapper = mount(ParsingView, { props: baseProps })
    await wrapper.find('.btn-cancel').trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()

    const cancelling = mount(ParsingView, { props: { ...baseProps, isCancelling: true } })
    expect(cancelling.find('.btn-cancel').attributes('disabled')).toBeDefined()
    expect(cancelling.text()).toContain('取消中')
  })
})
