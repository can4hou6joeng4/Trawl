import { beforeEach, describe, expect, it, vi } from 'vitest';

const wailsMethods = vi.hoisted(() => ({
  SelectFile: vi.fn(),
  PreviewData: vi.fn(),
  ExtractToPath: vi.fn(),
  ExportData: vi.fn(),
  SelectOutputPath: vi.fn(),
  ScanFields: vi.fn(),
  OpenFile: vi.fn(),
  GetTrialStatus: vi.fn(),
  GetMachineID: vi.fn(),
  Activate: vi.fn(),
  CancelExtraction: vi.fn(),
}));

vi.mock('../../wailsjs/go/app/App', () => wailsMethods);

describe('desktop api service', () => {
  beforeEach(() => {
    vi.resetModules();
    Object.values(wailsMethods).forEach(method => method.mockReset());
  });

  it('always reports desktop mode', async () => {
    const { api } = await import('./api');
    expect(api.isDesktop).toBe(true);
  });

  it('delegates file selection to Wails', async () => {
    wailsMethods.SelectFile.mockResolvedValue('/tmp/case.pdf');
    const { getApiService } = await import('./api');
    await expect(getApiService().selectFile()).resolves.toBe('/tmp/case.pdf');
    expect(wailsMethods.SelectFile).toHaveBeenCalledTimes(1);
  });

  it('delegates cancellation to Wails', async () => {
    wailsMethods.CancelExtraction.mockResolvedValue(undefined);
    const { getApiService } = await import('./api');
    await getApiService().cancelExtraction();
    expect(wailsMethods.CancelExtraction).toHaveBeenCalledTimes(1);
  });
});
