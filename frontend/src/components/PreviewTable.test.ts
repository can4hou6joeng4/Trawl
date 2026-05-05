import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PreviewTable from './PreviewTable.vue'

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

const fieldLabels = {
  defendant: '被告',
  idNumber: '身份证号',
  request: '诉讼请求',
  factsReason: '事实与理由',
}

describe('PreviewTable', () => {
  it('records 为空时表头列数为 0', () => {
    const wrapper = mount(PreviewTable, {
      props: { records: [], fieldLabels },
    })
    expect(wrapper.findAll('thead th').length).toBe(0)
  })

  it('records 含全部 4 个字段时渲染 4 列表头并使用 fieldLabels', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [
          {
            defendant: '张三',
            idNumber: '110000000000000000',
            request: '请求A',
            factsReason: '事实A',
          },
        ],
        fieldLabels,
      },
    })
    const headers = wrapper.findAll('thead th').map(th => th.text())
    expect(headers).toEqual(['被告', '身份证号', '诉讼请求', '事实与理由'])
  })

  it('records 仅含 defendant 与 request 时按 orderedKeys 顺序仅渲染 2 列', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [{ defendant: '李四', request: '请求B' }],
        fieldLabels,
      },
    })
    const headers = wrapper.findAll('thead th').map(th => th.text())
    expect(headers).toEqual(['被告', '诉讼请求'])
  })

  it('request 与 factsReason 列渲染 textarea，其它列渲染 input', () => {
    const wrapper = mount(PreviewTable, {
      props: {
        records: [
          {
            defendant: '王五',
            idNumber: '110000000000000001',
            request: '请求C',
            factsReason: '事实C',
          },
        ],
        fieldLabels,
      },
    })
    const cells = wrapper.findAll('tbody td')
    expect(cells[0].find('input').exists()).toBe(true)
    expect(cells[0].find('textarea').exists()).toBe(false)
    expect(cells[1].find('input').exists()).toBe(true)
    expect(cells[2].find('textarea').exists()).toBe(true)
    expect(cells[2].find('input').exists()).toBe(false)
    expect(cells[3].find('textarea').exists()).toBe(true)
  })

  it('修改 input 的 value 后同步回 records[index][col.key]', async () => {
    const records = [
      {
        defendant: '初值',
        idNumber: '110000000000000002',
        request: 'r',
        factsReason: 'f',
      },
    ]
    const wrapper = mount(PreviewTable, {
      props: { records, fieldLabels },
    })
    const firstInput = wrapper.find('tbody td input')
    await firstInput.setValue('改后值')
    expect(records[0].defendant).toBe('改后值')
  })
})
