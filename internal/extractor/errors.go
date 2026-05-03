// Package extractor: shared sentinel error values.
//
// Use errors.Is(err, ErrXxx) at API boundaries instead of string matching.
// Each sentinel carries a stable, user-facing Chinese message because both the
// desktop UI and the web JSON response surface .Error() directly.
package extractor

import "errors"

// ErrPDFEncrypted is returned when a PDF is password-protected or has
// permission flags that prevent text extraction.
var ErrPDFEncrypted = errors.New("PDF 文档已加密或受保护，无法解析")

// ErrUnsupportedFormat is returned for any extension the extractor cannot handle.
var ErrUnsupportedFormat = errors.New("不支持的文件格式")

// ErrEmptyFile is returned when the input byte slice is zero-length.
var ErrEmptyFile = errors.New("文件内容为空")

// ErrNoRecords is returned when extraction succeeds but no legal entities matched.
var ErrNoRecords = errors.New("未在文档中找到可提取的记录")

// ErrTokenMissing is returned when Baidu OCR is required but no token is configured.
var ErrTokenMissing = errors.New("百度 AI Studio Token 未配置")

// ErrCancelled is returned when ctx is cancelled mid-extraction.
var ErrCancelled = errors.New("提取已被取消")
