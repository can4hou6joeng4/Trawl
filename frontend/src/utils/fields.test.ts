import { describe, expect, it } from 'vitest'
import { FIELD_ORDER, fieldKind } from './fields'

describe('fields', () => {
  it('fieldKind 按字段映射输入控件类型', () => {
    expect(fieldKind('idNumber')).toBe('num')
    expect(fieldKind('request')).toBe('long')
    expect(fieldKind('factsReason')).toBe('long')
    expect(fieldKind('defendant')).toBe('text')
    expect(fieldKind('未知字段')).toBe('text')
  })

  it('FIELD_ORDER 为四个语义字段的固定顺序（不含 page）', () => {
    expect(FIELD_ORDER).toEqual(['defendant', 'idNumber', 'request', 'factsReason'])
    expect(FIELD_ORDER).not.toContain('page')
  })
})
