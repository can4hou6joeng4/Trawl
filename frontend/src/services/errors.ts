/**
 * 后端稳定错误码 → 用户可读中文文案。
 *
 * 后端 `internal/app.friendlyExtractError` 会将已知哨兵错误转换为稳定
 * 错误码（PDF_ENCRYPTED_OR_LOCKED / BAIDU_TOKEN_MISSING / CANCELLED /
 * UNSUPPORTED_FORMAT: ...），本模块是该契约的前端半边。
 * 未识别的字符串视为后端已人类可读的中文文案，原样透出。
 */

const ERROR_CODE_MESSAGES: { [code: string]: string } = {
  PDF_ENCRYPTED_OR_LOCKED:
    "PDF 已加密或受保护，无法解析。请先解除文档限制后重试。",
  BAIDU_TOKEN_MISSING:
    "未配置百度 OCR Token：扫描件与图片识别需要云端引擎。请在 config/conf.yaml 或环境变量 LEGAL_EXTRACTOR_BAIDU_TOKEN 配置后重试。",
  CANCELLED: "操作已取消",
};

const UNSUPPORTED_PREFIX = "UNSUPPORTED_FORMAT";

/** 判断错误信息是否为用户主动取消（应显示中性提示而非错误）。 */
export function isCancelledError(raw?: string | null): boolean {
  return raw === "CANCELLED";
}

/** 将后端错误信息转换为用户可读文案。 */
export function friendlyErrorMessage(raw?: string | null): string {
  if (!raw) return "未知错误";

  const mapped = ERROR_CODE_MESSAGES[raw];
  if (mapped) return mapped;

  if (raw.startsWith(UNSUPPORTED_PREFIX)) {
    // 形如 "UNSUPPORTED_FORMAT: 不支持的文件格式: .txt"，透出后端补充说明
    const detail = raw.slice(UNSUPPORTED_PREFIX.length).replace(/^[:：\s]+/, "");
    return detail || "不支持的文件格式";
  }

  return raw;
}
