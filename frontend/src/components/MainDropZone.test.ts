import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MainDropZone from './MainDropZone.vue'

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    isDesktop: false,
    service: {
      openFile: vi.fn().mockResolvedValue(undefined),
      selectFile: vi.fn().mockResolvedValue(null),
    },
  },
}))

vi.mock('../services', () => ({
  api: mockApi,
}))

beforeEach(() => {
  mockApi.service.selectFile.mockClear()
  mockApi.service.selectFile.mockResolvedValue(null)
})

describe('MainDropZone', () => {
  it('selectedFile 为 null 时显示「点击或拖拽上传文件」文案', () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    expect(wrapper.text()).toContain('点击或拖拽上传文件')
  })

  it('selectedFile 是字符串时 file-path-text 显示该字符串', () => {
    const wrapper = mount(MainDropZone, {
      props: {
        selectedFile: '/Users/foo/案件.docx',
        fileName: '案件.docx',
      },
    })
    expect(wrapper.find('.file-path-text').text()).toBe('/Users/foo/案件.docx')
    expect(wrapper.find('.file-name-display').text()).toBe('案件.docx')
  })

  it('selectedFile 是 500KB 的 File 时 displayPath 显示 "500.0 KB"', () => {
    const file = new File(['placeholder'], '案件.pdf', { type: 'application/pdf' })
    Object.defineProperty(file, 'size', { value: 500 * 1024 })
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: file, fileName: '案件.pdf' },
    })
    expect(wrapper.find('.file-path-text').text()).toBe('500.0 KB')
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

  it('Web drop 合法 .pdf 文件时 emit update:selectedFile 与 success notification', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    const file = new File(['data'], '案件.pdf', { type: 'application/pdf' })
    await wrapper.find('.drop-zone').trigger('drop', {
      dataTransfer: { files: [file] },
    })
    const updateEvents = wrapper.emitted('update:selectedFile')
    expect(updateEvents).toBeTruthy()
    expect(updateEvents![0][0]).toBe(file)
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('文件已加载')
    expect(notif![0][1]).toBe('success')
  })

  it('Web drop 非法 .txt 文件时 emit error notification 且不 emit update:selectedFile', async () => {
    const wrapper = mount(MainDropZone, {
      props: { selectedFile: null, fileName: '' },
    })
    const file = new File(['data'], 'note.txt', { type: 'text/plain' })
    await wrapper.find('.drop-zone').trigger('drop', {
      dataTransfer: { files: [file] },
    })
    expect(wrapper.emitted('update:selectedFile')).toBeFalsy()
    const notif = wrapper.emitted('notification')
    expect(notif).toBeTruthy()
    expect(notif![0][0]).toBe('不支持的文件格式')
    expect(notif![0][1]).toBe('error')
  })
})
