import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import LandingView from './LandingView.vue'

const runtimeMocks = vi.hoisted(() => ({
  OnFileDrop: vi.fn(),
  OnFileDropOff: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => runtimeMocks)

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    service: {
      selectFile: vi.fn().mockResolvedValue(''),
    },
  },
}))

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  runtimeMocks.OnFileDrop.mockClear()
  runtimeMocks.OnFileDropOff.mockClear()
  mockApi.service.selectFile.mockClear()
  mockApi.service.selectFile.mockResolvedValue('')
})

describe('LandingView', () => {
  it('渲染拖放区与格式提示', () => {
    const wrapper = mount(LandingView)
    expect(wrapper.text()).toContain('拖入文书，或点击选择')
    expect(wrapper.text()).toContain('.PDF · .DOCX · .JPG/.PNG')
  })

  it('Wails drop 合法路径时 emit file', async () => {
    const wrapper = mount(LandingView)
    await new Promise(resolve => setTimeout(resolve, 0))

    const callback = runtimeMocks.OnFileDrop.mock.calls[0][0]
    callback(0, 0, ['/tmp/证据.JPEG'])

    const events = wrapper.emitted('file')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('/tmp/证据.JPEG')
  })

  it('Wails drop 非法路径时 emit notify error 且不 emit file', async () => {
    const wrapper = mount(LandingView)
    await new Promise(resolve => setTimeout(resolve, 0))

    const callback = runtimeMocks.OnFileDrop.mock.calls[0][0]
    callback(0, 0, ['/tmp/notes.txt'])

    expect(wrapper.emitted('file')).toBeFalsy()
    const notif = wrapper.emitted('notify')
    expect(notif).toBeTruthy()
    expect(notif![0][1]).toBe('error')
  })

  it('点击选择文件成功时 emit file', async () => {
    mockApi.service.selectFile.mockResolvedValue('/tmp/case.pdf')
    const wrapper = mount(LandingView)

    await wrapper.find('.dropzone').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))

    const events = wrapper.emitted('file')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('/tmp/case.pdf')
  })

  it('取消选择（空路径）不 emit file', async () => {
    const wrapper = mount(LandingView)

    await wrapper.find('.dropzone').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(wrapper.emitted('file')).toBeFalsy()
  })
})
