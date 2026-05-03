package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractFromDocx 保留原有的本地 DOCX 提取逻辑
func (e *Extractor) extractFromDocx(fileData []byte, fields []string) ([]Record, error) {
	text, err := extractTextFromDocx(fileData)
	if err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		for k := range PatternRegistry {
			fields = append(fields, k)
		}
	}

	return e.parseCases(text, fields), nil
}

// extractTextFromDocx 核心 DOCX 文本提取逻辑
func extractTextFromDocx(fileData []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", err
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

	for {
		t, _ := decoder.Token()
		if t == nil {
			break
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
