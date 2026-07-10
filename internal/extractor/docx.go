package extractor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// docxCtxCheckInterval 是 XML token 循环里检查 ctx.Err() 的间隔。
// 实测大 docx 单页 ~1500 token，256 给到约每页 5-6 次中断机会，
// 既保证 100ms 量级响应，又不让分支预测开销显著。
const docxCtxCheckInterval = 256

// extractFromDocx 保留原有的本地 DOCX 提取逻辑。
//
// ctx 在三处生效：本函数入口、extractTextFromDocx 解析 zip 之后，以及
// XML token 循环内每 docxCtxCheckInterval 次迭代一次。任意一处命中
// 取消都会返回 ErrCancelled，与 PDF / 百度 / WinOCR 路径保持一致。
func (e *Extractor) extractFromDocx(ctx context.Context, fileData []byte, fields []string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, ErrCancelled
	}

	text, err := extractTextFromDocx(ctx, fileData, e.logger)
	if err != nil {
		return nil, err
	}

	// fields 为空时 parseCases 默认提取全部已注册字段
	return e.parseCases(text, fields), nil
}

// extractTextFromDocx 核心 DOCX 文本提取逻辑。
func extractTextFromDocx(ctx context.Context, fileData []byte, logger *slog.Logger) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", ErrCancelled
	}

	var documentXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			documentXML, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if documentXML == nil {
		return "", fmt.Errorf("word/document.xml not found")
	}
	defer func() { _ = documentXML.Close() }()

	decoder := xml.NewDecoder(documentXML)
	var sb strings.Builder

	tokenCount := 0
	for {
		t, tokenErr := decoder.Token()
		if t == nil {
			// 损坏的 XML 中途断流时 Token 返回非 EOF 错误：
			// 记录日志避免文本被静默截断而无从排查。
			if tokenErr != nil && !errors.Is(tokenErr, io.EOF) && logger != nil {
				logger.Warn("DOCX XML 解析中途终止，提取文本可能不完整", "error", tokenErr)
			}
			break
		}
		tokenCount++
		if tokenCount%docxCtxCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return "", ErrCancelled
			}
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "t" {
				var s string
				if err := decoder.DecodeElement(&s, &se); err == nil {
					sb.WriteString(s)
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "p", "tr":
				sb.WriteString("\n")
			case "tc":
				sb.WriteString(" ")
			}
		}
	}

	return sb.String(), nil
}
