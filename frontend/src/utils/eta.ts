/**
 * Estimated-time-remaining helper for the extraction progress overlay.
 *
 * Inputs are the most recent (timestamp, current) samples plus the total
 * unit count from the backend's progress callback. We compute throughput
 * over a sliding window so an early stall doesn't permanently skew the
 * estimate.
 */

export interface ProgressSample {
  /** Wall-clock ms (Date.now()) when the sample was taken. */
  t: number;
  /** Backend's reported `current` for this sample. */
  current: number;
}

/**
 * Compute the ETA label.
 *
 * Returns:
 *   - '估算中...' when too few samples or rate is too low to be meaningful
 *   - '约 N 秒'   when ETA < 60s
 *   - '约 N 分 M 秒' otherwise
 */
export function formatEta(samples: ProgressSample[], total: number): string {
  if (total <= 0 || samples.length < 2) {
    return '估算中...';
  }

  // Use a sliding window: only consider samples from the last 8 seconds.
  const now = samples[samples.length - 1].t;
  const window = samples.filter(s => now - s.t <= 8000);
  if (window.length < 2) {
    return '估算中...';
  }

  const first = window[0];
  const last = window[window.length - 1];
  const dt = (last.t - first.t) / 1000; // seconds
  const dCur = last.current - first.current;

  if (dt <= 0 || dCur <= 0) {
    return '估算中...';
  }

  const ratePerSec = dCur / dt;
  const remaining = Math.max(0, total - last.current);
  const etaSec = Math.round(remaining / ratePerSec);

  if (etaSec <= 0) {
    return '即将完成';
  }
  if (etaSec < 60) {
    return `约 ${etaSec} 秒`;
  }
  const min = Math.floor(etaSec / 60);
  const sec = etaSec % 60;
  return sec === 0 ? `约 ${min} 分` : `约 ${min} 分 ${sec} 秒`;
}
