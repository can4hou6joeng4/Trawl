import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TrialBanner from './TrialBanner.vue'

const trialing = {
  isActivated: false,
  isExpired: false,
  remaining: 100,
  days: 5,
  hours: 3,
}

describe('TrialBanner', () => {
  it('trialStatus 为 null 时不渲染任何内容', () => {
    const wrapper = mount(TrialBanner, { props: { trialStatus: null } })
    expect(wrapper.find('.trial-banner').exists()).toBe(false)
    expect(wrapper.find('.active-badge-fixed').exists()).toBe(false)
  })

  it('试用中显示剩余天数并在点击时 emit activate', async () => {
    const wrapper = mount(TrialBanner, { props: { trialStatus: trialing } })
    expect(wrapper.text()).toContain('试用期剩余')
    expect(wrapper.text()).toContain('5')

    await wrapper.find('.trial-cta-btn').trigger('click')
    expect(wrapper.emitted('activate')).toBeTruthy()
  })

  it('试用过期显示锁定文案与紧急按钮', () => {
    const wrapper = mount(TrialBanner, {
      props: { trialStatus: { ...trialing, isExpired: true } },
    })
    expect(wrapper.text()).toContain('试用期已结束')
    expect(wrapper.find('.trial-cta-btn.urgent').exists()).toBe(true)
  })

  it('已激活时仅显示专业版徽章', () => {
    const wrapper = mount(TrialBanner, {
      props: { trialStatus: { ...trialing, isActivated: true } },
    })
    expect(wrapper.find('.trial-banner').exists()).toBe(false)
    expect(wrapper.text()).toContain('专业授权版')
  })
})
