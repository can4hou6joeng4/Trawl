export interface Record {
  [key: string]: string;
}

/** 云端 OCR 识别的一行文字，坐标归一化到 [0,1]（相对页面尺寸） */
export interface OcrLine {
  text: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

/** 一页的 OCR 文字行；page 为全文档 1-based 页码 */
export interface OcrPageLines {
  page: number;
  lines: OcrLine[];
}

export interface ExtractResult {
  success: boolean;
  recordCount: number;
  outputPath?: string;
  errorMessage?: string;
  records?: Record[];
  fieldLabels?: { [key: string]: string };
  /** 扫描件/图片的逐页 OCR 坐标行（字段高亮数据源），文本层 PDF/DOCX 为空 */
  ocrPages?: OcrPageLines[];
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
  /** 读取最近一次解析的文书内容（base64），供原文页面渲染 */
  readCurrentDocument(): Promise<string>;
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

  async readCurrentDocument(): Promise<string> {
    const { ReadCurrentDocument } = await import('../../wailsjs/go/app/App');
    return ReadCurrentDocument();
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
