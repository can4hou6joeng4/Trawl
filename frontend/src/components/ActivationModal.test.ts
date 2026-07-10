import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ActivationModal from './ActivationModal.vue'

describe('ActivationModal', () => {
  it('show 为 false 时不渲染', () => {
    const wrapper = mount(ActivationModal, {
      props: { show: false, machineID: 'ABCD1234' },
    })
    expect(wrapper.find('.modal-overlay').exists()).toBe(false)
  })

  it('显示设备特征码，点击关闭按钮 emit close', async () => {
    const wrapper = mount(ActivationModal, {
      props: { show: true, machineID: 'ABCD1234' },
    })
    expect(wrapper.text()).toContain('ABCD1234')

    await wrapper.find('.close-btn').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('授权码不足 16 位时激活按钮禁用，达标后 emit activate 并带上授权码', async () => {
    const wrapper = mount(ActivationModal, {
      props: { show: true, machineID: 'ABCD1234' },
    })

    const activateBtn = wrapper.find('.btn-primary')
    expect(activateBtn.attributes('disabled')).toBeDefined()

    await wrapper.find('.license-input').setValue('AAAA-BBBB-CCCC-DDDD')
    expect(activateBtn.attributes('disabled')).toBeUndefined()

    await activateBtn.trigger('click')
    const events = wrapper.emitted('activate')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('AAAA-BBBB-CCCC-DDDD')
  })

  it('关闭弹窗后重新打开时输入框被清空', async () => {
    const wrapper = mount(ActivationModal, {
      props: { show: true, machineID: 'ABCD1234' },
    })

    await wrapper.find('.license-input').setValue('AAAA-BBBB-CCCC-DDDD')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    const input = wrapper.find('.license-input').element as HTMLInputElement
    expect(input.value).toBe('')
  })
})
