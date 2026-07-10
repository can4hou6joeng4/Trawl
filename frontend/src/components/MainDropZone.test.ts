import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MainDropZone from './MainDropZone.vue'

const { mockApi, runtimeMocks } = vi.hoisted(() => ({
  mockApi: {
    service: {
      selectFile: vi.fn().mockResolvedValue(null),
    },
  },
  runtimeMocks: {
    OnFileDrop: vi.fn(),
    OnFileDropOff: vi.fn(),
  },
}))

vi.mock('../services', () => ({
  api: mockApi,
}))

vi.mock('../../wailsjs/runtime/runtime', () => runtimeMocks)

beforeEach(() => {
  mockApi.service.selectFile.mockClear()
  mockApi.service.selectFile.mockResolvedValue(null)
  runtimeMocks.OnFileDrop.mockClear()
  runtimeMocks.OnFileDropOff.mockClear()
})

describe('MainDropZone', () => {
  it('selectedFile 为 null 时显示选择文书入口文案', () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    expect(wrapper.text()).toContain('选择待处理文书')
    expect(wrapper.text()).toContain('点击选择或拖入 .docx / .pdf / .jpg / .jpeg / .png 文件')
  })

  it('selectedFile 是路径字符串时 file-path-text 显示该路径', () => {
    const wrapper = mount(MainDropZone, {
      props: {
        selectedFile: '/Users/foo/案件.docx',
        fileName: '案件.docx',
      },
    })
    expect(wrapper.find('.file-path-text').text()).toBe('/Users/foo/案件.docx')
    expect(wrapper.find('.file-name-display').text()).toBe('案件.docx')
  })

  it('点击容器调用 selectFile 且非空返回时 emit update:selectedFile', async () => {
    mockApi.service.selectFile.mockResolvedValueOnce('/tmp/picked.pdf')
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    await wrapper.find('.drop-zone').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(mockApi.service.selectFile).toHaveBeenCalledTimes(1)
    const events = wrapper.emitted('update:selectedFile')
    expect(events).toBeTruthy()
    expect(events![0][0]).toBe('/tmp/picked.pdf')
  })

  it('Wails drop 合法 .pdf 路径时 emit update:selectedFile 与 success notification', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    await new Promise(resolve => setTimeout(resolve, 0))

    const callback = runtimeMocks.OnFileDrop.mock.calls[0][0]
    callback(0, 0, ['/tmp/案件.pdf'])

    const updateEvents = wrapper.emitted('update:selectedFile')
    expect(updateEvents).toBeTruthy()
    expect(updateEvents![0][0]).toBe('/tmp/案件.pdf')
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('文件已加载')
    expect(notif![0][1]).toBe('success')
  })

  it('Wails drop 合法 .jpeg 路径时 emit update:selectedFile 与 success notification', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    await new Promise(resolve => setTimeout(resolve, 0))

    const callback = runtimeMocks.OnFileDrop.mock.calls[0][0]
    callback(0, 0, ['/tmp/证据.JPEG'])

    const updateEvents = wrapper.emitted('update:selectedFile')
    expect(updateEvents).toBeTruthy()
    expect(updateEvents![0][0]).toBe('/tmp/证据.JPEG')
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('文件已加载')
    expect(notif![0][1]).toBe('success')
  })

  it('Wails drop 非法 .txt 路径时 emit error notification 且不 emit update:selectedFile', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    await new Promise(resolve => setTimeout(resolve, 0))

    const callback = runtimeMocks.OnFileDrop.mock.calls[0][0]
    callback(0, 0, ['/tmp/note.txt'])

    expect(wrapper.emitted('update:selectedFile')).toBeFalsy()
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('不支持的文件格式')
    expect(notif![0][1]).toBe('error')
  })
})
