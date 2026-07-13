import { beforeEach, describe, expect, it, vi } from 'vitest';

const wailsMethods = vi.hoisted(() => ({
  SelectFile: vi.fn(),
  PreviewData: vi.fn(),
  ExtractToPath: vi.fn(),
  ExportData: vi.fn(),
  SelectOutputPath: vi.fn(),
  ScanFields: vi.fn(),
  OpenFile: vi.fn(),
  ReadCurrentDocument: vi.fn(),
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

  it('delegates current document read to Wails', async () => {
    wailsMethods.ReadCurrentDocument.mockResolvedValue('QUJD');
    const { getApiService } = await import('./api');
    await expect(getApiService().readCurrentDocument()).resolves.toBe('QUJD');
    expect(wailsMethods.ReadCurrentDocument).toHaveBeenCalledTimes(1);
  });

  it('delegates preview and extraction with pass-through args', async () => {
    wailsMethods.PreviewData.mockResolvedValue({ success: true, recordCount: 1 });
    wailsMethods.ExtractToPath.mockResolvedValue({ success: true, recordCount: 2 });
    const { getApiService } = await import('./api');

    await expect(getApiService().previewData('/f.pdf', ['defendant'])).resolves.toMatchObject({ recordCount: 1 });
    expect(wailsMethods.PreviewData).toHaveBeenCalledWith('/f.pdf', ['defendant']);

    await expect(getApiService().extractToPath('/f.pdf', '/o.xlsx', ['idNumber'])).resolves.toMatchObject({ recordCount: 2 });
    expect(wailsMethods.ExtractToPath).toHaveBeenCalledWith('/f.pdf', '/o.xlsx', ['idNumber']);
  });

  it('delegates export and output path selection', async () => {
    wailsMethods.ExportData.mockResolvedValue({ success: true, recordCount: 3 });
    wailsMethods.SelectOutputPath.mockResolvedValue('/tmp/out.xlsx');
    const { getApiService } = await import('./api');

    const records = [{ defendant: '张三' }];
    await expect(getApiService().exportData(records, '/o.csv')).resolves.toMatchObject({ recordCount: 3 });
    expect(wailsMethods.ExportData).toHaveBeenCalledWith(records, '/o.csv');

    await expect(getApiService().selectOutputPath('提取结果.xlsx')).resolves.toBe('/tmp/out.xlsx');
    expect(wailsMethods.SelectOutputPath).toHaveBeenCalledWith('提取结果.xlsx');
  });

  it('delegates field scan and file open', async () => {
    wailsMethods.ScanFields.mockResolvedValue([{ key: 'defendant', label: '被告' }]);
    wailsMethods.OpenFile.mockResolvedValue(undefined);
    const { getApiService } = await import('./api');

    await expect(getApiService().scanFields('/f.pdf')).resolves.toEqual([{ key: 'defendant', label: '被告' }]);
    expect(wailsMethods.ScanFields).toHaveBeenCalledWith('/f.pdf');

    await getApiService().openFile('/o.xlsx');
    expect(wailsMethods.OpenFile).toHaveBeenCalledWith('/o.xlsx');
  });

  it('delegates licensing methods', async () => {
    wailsMethods.GetTrialStatus.mockResolvedValue({ isActivated: true, isExpired: false, remaining: 1, days: 9, hours: 9 });
    wailsMethods.GetMachineID.mockResolvedValue('MACHINE-ID');
    wailsMethods.Activate.mockResolvedValue(true);
    const { getApiService } = await import('./api');

    await expect(getApiService().getTrialStatus()).resolves.toMatchObject({ isActivated: true, days: 9 });
    await expect(getApiService().getMachineID()).resolves.toBe('MACHINE-ID');
    await expect(getApiService().activate('LICENSE-KEY')).resolves.toBe(true);
    expect(wailsMethods.Activate).toHaveBeenCalledWith('LICENSE-KEY');
  });

  it('service is a per-module singleton', async () => {
    const { api, getApiService } = await import('./api');
    expect(api.service).toBe(getApiService());
    expect(getApiService()).toBe(getApiService());
  });
});
