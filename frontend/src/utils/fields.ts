/**
 * 取证台字段与导出格式的共享定义。
 * 字段顺序与后端 exportColumnOrder 的语义列一致（page 是元信息，单独处理）。
 */

export interface RecordField {
  key: string;
  label: string;
  kind: "text" | "num" | "long";
}

export type ExportFormat = "xlsx" | "csv" | "json";

/** 语义字段的固定展示顺序（不含 page） */
export const FIELD_ORDER = ["defendant", "idNumber", "request", "factsReason"];

/** 字段的输入控件类型，未列出的默认单行文本 */
const FIELD_KINDS: { [key: string]: RecordField["kind"] } = {
  idNumber: "num",
  request: "long",
  factsReason: "long",
};

export function fieldKind(key: string): RecordField["kind"] {
  return FIELD_KINDS[key] || "text";
}
