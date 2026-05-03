package extractor

import (
	"regexp"
	"strings"
)

// parseCases 现有的本地正则解析逻辑 (用于 DOCX)
func (e *Extractor) parseCases(text string, fields []string) []Record {
	parts := DefaultPatterns.Split.Split(text, -1)
	var data []Record

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		record := make(Record)
		fieldSet := make(map[string]bool)
		for _, f := range fields {
			fieldSet[f] = true
		}

		// 1. 提取被告
		if fieldSet["defendant"] {
			loc := DefaultPatterns.DefStart.FindStringIndex(part)
			if loc != nil {
				startIdx := loc[1]
				remaining := part[startIdx:]
				cleanRemaining := strings.ReplaceAll(remaining, "\n", "")
				locEnd := DefaultPatterns.DefEnd.FindStringIndex(cleanRemaining)

				var name string
				if locEnd != nil {
					name = cleanRemaining[:locEnd[0]]
				} else {
					if len(cleanRemaining) > 50 {
						name = cleanRemaining[:50]
					} else {
						name = cleanRemaining
					}
				}
				record["defendant"] = strings.TrimSpace(name)
			}
		}

		// 2. 提取身份证
		if fieldSet["idNumber"] {
			matchID := DefaultPatterns.ID.FindStringSubmatch(part)
			if len(matchID) > 1 {
				record["idNumber"] = strings.TrimSpace(matchID[1])
			}
		}

		// 3. 提取请求
		if fieldSet["request"] {
			matchReq := DefaultPatterns.Request.FindStringSubmatch(part)
			if len(matchReq) > 1 {
				record["request"] = smartMerge(matchReq[1])
			}
		}

		// 4. 提取事实
		if fieldSet["factsReason"] {
			matchFact := DefaultPatterns.Facts.FindStringSubmatch(part)
			if len(matchFact) > 1 {
				record["factsReason"] = smartMerge(matchFact[1])
			}
		}

		if len(record) > 0 {
			data = append(data, record)
		}
	}
	return data
}

// smartMerge 预编译正则
var (
	reMultipleNL     = regexp.MustCompile(`\n+`)
	rePreserveAfter  = regexp.MustCompile(`([。；？！])\n`)
	rePreserveBefore = regexp.MustCompile(`\n(\s*(?:[一二三四五六七八九十\d]+[、．]|[(（][一二三四五六七八九十\d]+[)）]))`)
)

// smartMerge 智能合并换行符
// 逻辑：保留句号、分号、冒号后的换行，或者新条目序号（如"二、"）之前的换行，其他的换行符视作布局造成的干扰并予以合并。
func smartMerge(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// 1. 标准化换行符
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = reMultipleNL.ReplaceAllString(s, "\n")

	// 2. 标记需要保留的"逻辑断点"
	s = rePreserveAfter.ReplaceAllString(s, "$1[LOGICAL_NL]")
	s = rePreserveBefore.ReplaceAllString(s, "[LOGICAL_NL]$1")

	// 3. 合并 OCR 碎行：将剩余的非逻辑换行符替换为一个小空格，防止文字粘连
	s = strings.ReplaceAll(s, "\n", " ")

	// 4. 将占位符还原为真正的换行符
	s = strings.ReplaceAll(s, "[LOGICAL_NL]", "\n")

	// 5. 深度清理：合并每行内部的多余空格
	lines := strings.Split(s, "\n")
	var resultLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		resultLines = append(resultLines, strings.Join(fields, " "))
	}

	return strings.Join(resultLines, "\n")
}
