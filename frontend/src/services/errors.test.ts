import { describe, expect, it } from 'vitest';
import { friendlyErrorMessage, isCancelledError } from './errors';

describe('friendlyErrorMessage', () => {
  it('映射已知错误码为中文文案', () => {
    expect(friendlyErrorMessage('PDF_ENCRYPTED_OR_LOCKED')).toContain('PDF 已加密');
    expect(friendlyErrorMessage('BAIDU_TOKEN_MISSING')).toContain('百度 OCR Token');
    expect(friendlyErrorMessage('CANCELLED')).toBe('操作已取消');
  });

  it('UNSUPPORTED_FORMAT 前缀透出后端补充说明', () => {
    expect(friendlyErrorMessage('UNSUPPORTED_FORMAT: 不支持的文件格式: .txt')).toBe(
      '不支持的文件格式: .txt',
    );
    expect(friendlyErrorMessage('UNSUPPORTED_FORMAT')).toBe('不支持的文件格式');
  });

  it('未知文案原样透出（后端已人类可读）', () => {
    expect(friendlyErrorMessage('读取文件失败: permission denied')).toBe(
      '读取文件失败: permission denied',
    );
  });

  it('空值返回未知错误', () => {
    expect(friendlyErrorMessage(undefined)).toBe('未知错误');
    expect(friendlyErrorMessage('')).toBe('未知错误');
  });
});

describe('isCancelledError', () => {
  it('仅 CANCELLED 视为用户取消', () => {
    expect(isCancelledError('CANCELLED')).toBe(true);
    expect(isCancelledError('PDF_ENCRYPTED_OR_LOCKED')).toBe(false);
    expect(isCancelledError(undefined)).toBe(false);
  });
});
