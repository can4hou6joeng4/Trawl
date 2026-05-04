import { describe, it, expect, beforeEach, vi } from 'vitest';

// api.ts 含 module-level singleton（apiServiceInstance），所以每个测试前
// 都用 vi.resetModules() 让 dynamic import 拿到全新模块状态。

describe('mode detection (isDesktopMode / isWebMode)', () => {
  beforeEach(() => {
    vi.resetModules();
    delete (window as any).go;
  });

  it('returns desktop mode when window.go exists (Wails injected namespace)', async () => {
    (window as any).go = { app: { App: {} } };
    const { isDesktopMode, isWebMode } = await import('./api');
    expect(isDesktopMode()).toBe(true);
    expect(isWebMode()).toBe(false);
  });

  it('returns web mode when window.go is absent', async () => {
    const { isDesktopMode, isWebMode } = await import('./api');
    expect(isDesktopMode()).toBe(false);
    expect(isWebMode()).toBe(true);
  });
});

describe('WebAdapter cancellation flow', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    delete (window as any).go;
  });

  it('cancelExtraction() aborts the in-flight previewData fetch signal', async () => {
    // 捕获 fetch 拿到的 AbortSignal
    let captured: AbortSignal | undefined;
    const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
      captured = init?.signal ?? undefined;
      // 永远 pending —— 只能由 cancel 触发的 abort 中断
      return new Promise<Response>(() => undefined);
    });
    vi.stubGlobal('fetch', fetchMock);

    const { getApiService } = await import('./api');
    const service = getApiService();

    const file = new File([new Uint8Array([1, 2, 3])], 'sample.docx');
    // 不 await：让 fetch 进入 pending 状态
    const promise = service.previewData(file as any, ['defendant']);
    // 静默 reject（对 AbortError 在 WebAdapter 内已被吞为 success:false）
    promise.catch(() => undefined);

    // 让微任务跑完，确保 fetch 已被调用、signal 已被记录
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(captured).toBeDefined();
    expect(captured!.aborted).toBe(false);

    // 触发取消
    await service.cancelExtraction();

    expect(captured!.aborted).toBe(true);
  });
});
