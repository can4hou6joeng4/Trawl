export interface Record {
  [key: string]: string;
}

export interface ExtractResult {
  success: boolean;
  recordCount: number;
  outputPath?: string;
  errorMessage?: string;
  records?: Record[];
  fieldLabels?: { [key: string]: string };
}

export interface FieldOption {
  key: string;
  label: string;
}

/** 试用/授权状态，对应后端 config.TrialStatus。 */
export interface TrialStatus {
  isActivated: boolean;
  isExpired: boolean;
  remaining: number;
  days: number;
  hours: number;
}

export interface IApiService {
  selectFile(): Promise<string>;
  previewData(filePath: string, fields: string[]): Promise<ExtractResult>;
  extractToPath(filePath: string, outputPath: string, fields: string[]): Promise<ExtractResult>;
  exportData(records: Record[], outputPath: string): Promise<ExtractResult>;
  selectOutputPath(defaultName: string): Promise<string>;
  scanFields(filePath: string): Promise<FieldOption[]>;
  openFile(path: string): Promise<void>;
  getTrialStatus(): Promise<TrialStatus>;
  getMachineID(): Promise<string>;
  activate(licenseKey: string): Promise<boolean>;
  cancelExtraction(): Promise<void>;
}

class DesktopAdapter implements IApiService {
  async selectFile(): Promise<string> {
    const { SelectFile } = await import('../../wailsjs/go/app/App');
    return SelectFile();
  }

  async previewData(filePath: string, fields: string[]): Promise<ExtractResult> {
    const { PreviewData } = await import('../../wailsjs/go/app/App');
    return PreviewData(filePath, fields);
  }

  async extractToPath(filePath: string, outputPath: string, fields: string[]): Promise<ExtractResult> {
    const { ExtractToPath } = await import('../../wailsjs/go/app/App');
    return ExtractToPath(filePath, outputPath, fields);
  }

  async exportData(records: Record[], outputPath: string): Promise<ExtractResult> {
    const { ExportData } = await import('../../wailsjs/go/app/App');
    return ExportData(records, outputPath);
  }

  async selectOutputPath(defaultName: string): Promise<string> {
    const { SelectOutputPath } = await import('../../wailsjs/go/app/App');
    return SelectOutputPath(defaultName);
  }

  async scanFields(filePath: string): Promise<FieldOption[]> {
    const { ScanFields } = await import('../../wailsjs/go/app/App');
    return ScanFields(filePath);
  }

  async openFile(path: string): Promise<void> {
    const { OpenFile } = await import('../../wailsjs/go/app/App');
    return OpenFile(path);
  }

  async getTrialStatus(): Promise<TrialStatus> {
    const { GetTrialStatus } = await import('../../wailsjs/go/app/App');
    return GetTrialStatus();
  }

  async getMachineID(): Promise<string> {
    const { GetMachineID } = await import('../../wailsjs/go/app/App');
    return GetMachineID();
  }

  async activate(licenseKey: string): Promise<boolean> {
    const { Activate } = await import('../../wailsjs/go/app/App');
    return Activate(licenseKey);
  }

  async cancelExtraction(): Promise<void> {
    const { CancelExtraction } = await import('../../wailsjs/go/app/App');
    return CancelExtraction();
  }
}

let apiServiceInstance: IApiService | null = null;

export function getApiService(): IApiService {
  if (!apiServiceInstance) {
    apiServiceInstance = new DesktopAdapter();
  }
  return apiServiceInstance;
}

export const api = {
  get isDesktop() {
    return true;
  },
  get service() {
    return getApiService();
  },
};
