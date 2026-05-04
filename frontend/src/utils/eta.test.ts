import { describe, it, expect } from 'vitest';
import { formatEta, type ProgressSample } from './eta';

describe('formatEta', () => {
  it('returns "估算中..." when total is zero', () => {
    expect(formatEta([{ t: 0, current: 0 }, { t: 1000, current: 5 }], 0)).toBe('估算中...');
  });

  it('returns "估算中..." when samples array is empty', () => {
    expect(formatEta([], 100)).toBe('估算中...');
  });

  it('returns "估算中..." when only one sample is available', () => {
    expect(formatEta([{ t: 0, current: 0 }], 100)).toBe('估算中...');
  });

  it('returns "估算中..." when window has zero progress (dCur = 0)', () => {
    const samples: ProgressSample[] = [
      { t: 0, current: 5 },
      { t: 2000, current: 5 },
    ];
    expect(formatEta(samples, 100)).toBe('估算中...');
  });

  it('returns "即将完成" when current already reached total', () => {
    const samples: ProgressSample[] = [
      { t: 0, current: 50 },
      { t: 2000, current: 100 },
    ];
    expect(formatEta(samples, 100)).toBe('即将完成');
  });

  it('formats short ETAs in seconds', () => {
    // rate = (10 - 0)/(1000ms - 0ms) = 10/sec; remaining = 100 - 10 = 90 → 9 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 10 },
    ];
    expect(formatEta(samples, 100)).toBe('约 9 秒');
  });

  it('formats whole-minute ETAs without seconds', () => {
    // rate = 1/sec; remaining = 120 → 120 秒 = 2 分整
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 1 },
    ];
    expect(formatEta(samples, 121)).toBe('约 2 分');
  });

  it('formats minute+second ETAs', () => {
    // rate = 1/sec; remaining = 150 → 2 分 30 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 1000, current: 1 },
    ];
    expect(formatEta(samples, 151)).toBe('约 2 分 30 秒');
  });

  it('uses 8-second sliding window so old slow samples do not poison the rate', () => {
    // 早期 10 秒慢启动（0 → 10）；最近 0.5 秒高速（10 → 15）。
    // 滑窗截止 = 10500 - 8000 = 2500，丢弃 t=0 那条。
    // 窗内：first={t:10000, current:10}, last={t:10500, current:15}
    // dt = 0.5s, dCur = 5, rate = 10/sec
    // remaining = 100 - 15 = 85 → 9 秒
    const samples: ProgressSample[] = [
      { t: 0, current: 0 },
      { t: 10000, current: 10 },
      { t: 10500, current: 15 },
    ];
    expect(formatEta(samples, 100)).toBe('约 9 秒');
  });
});
