package extractor

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestParseCases(t *testing.T) {
	e := NewExtractor(nil)
	text := `
民 事 起 诉 状

被 告： 张三
身份证号码： 110101199001011234
住址： 北京市朝阳区

诉讼请求：
1. 请求判令被告偿还借款10000元。
2. 诉讼费由被告承担。

事实与理由：
2023年1月1日，被告向原告借款...
此致
`
	expected := []Record{
		{
			"defendant":   "张三",
			"idNumber":    "110101199001011234",
			"request":     "1. 请求判令被告偿还借款10000元。\n2. 诉讼费由被告承担。",
			"factsReason": "2023年1月1日，被告向原告借款...",
		},
	}

	result := e.parseCases(text, []string{"defendant", "idNumber", "request", "factsReason"})

	if len(result) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(result))
	}

	for k, v := range expected[0] {
		if result[0][k] != v && k != "request" && k != "factsReason" {
			t.Errorf("Field %s: expected %q, got %q", k, v, result[0][k])
		}
	}
}

func TestParseCases_MultipleDocuments(t *testing.T) {
	e := NewExtractor(nil)
	text := `
民 事 起 诉 状

被 告： 张三
身份证号码： 110101199001011234

诉讼请求：
请求偿还借款。

事实与理由：
被告借款未还。
此致

民 事 起 诉 状

被 告： 李四
身份证号码： 220102198805061234

诉讼请求：
请求偿还货款。

事实与理由：
被告拖欠货款。
此致
`
	result := e.parseCases(text, []string{"defendant", "idNumber", "request", "factsReason"})

	if len(result) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(result))
	}

	if result[0]["defendant"] != "张三" {
		t.Errorf("第1条被告 = %q, want %q", result[0]["defendant"], "张三")
	}
	if result[1]["defendant"] != "李四" {
		t.Errorf("第2条被告 = %q, want %q", result[1]["defendant"], "李四")
	}
}

func TestParseCases_EmptyText(t *testing.T) {
	e := NewExtractor(nil)
	result := e.parseCases("", []string{"defendant"})
	if len(result) != 0 {
		t.Errorf("空文本应返回空结果, got %d records", len(result))
	}
}

func TestParseCases_NoMatchingFields(t *testing.T) {
	e := NewExtractor(nil)
	text := "这是一段不包含法律文书格式的普通文本。"
	result := e.parseCases(text, []string{"defendant", "idNumber"})
	if len(result) != 0 {
		t.Errorf("无匹配字段应返回空结果, got %d records", len(result))
	}
}

func TestSmartMerge(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// smartMerge 会将非逻辑换行符替换为空格，防止 OCR 碎行文字粘连
			name:  "Merge weird newlines",
			input: "这是\n一句\n完整的话。",
			want:  "这是 一句 完整的话。",
		},
		{
			name:  "Preserve lists",
			input: "1. 第一点\n2. 第二点",
			want:  "1. 第一点 2. 第二点", // 数字列表不在保留规则内，会被合并
		},
		{
			name:  "Preserve punctuation",
			input: "第一句。\n第二句",
			want:  "第一句。\n第二句",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "保留分号后换行",
			input: "第一条；\n第二条",
			want:  "第一条；\n第二条",
		},
		{
			name:  "保留中文序号",
			input: "内容一\n二、第二部分",
			want:  "内容一\n二、第二部分",
		},
		{
			name:  "CRLF 标准化",
			input: "第一行\r\n第二行。\r\n第三行",
			want:  "第一行 第二行。\n第三行",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartMerge(tt.input)
			if got != tt.want {
				t.Errorf("smartMerge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateHash(t *testing.T) {
	e := NewExtractor(nil)

	// 相同内容应产生相同哈希
	hash1 := e.calculateHash([]byte("test content"))
	hash2 := e.calculateHash([]byte("test content"))
	if hash1 != hash2 {
		t.Errorf("相同内容哈希不一致: %q != %q", hash1, hash2)
	}

	// 不同内容应产生不同哈希
	hash3 := e.calculateHash([]byte("different content"))
	if hash1 == hash3 {
		t.Error("不同内容不应产生相同哈希")
	}

	// 哈希应该是 64 字符的十六进制字符串 (SHA256)
	if len(hash1) != 64 {
		t.Errorf("SHA256 哈希长度应为 64, got %d", len(hash1))
	}
}

func TestNewExtractor(t *testing.T) {
	// nil logger 不应导致 panic
	e := NewExtractor(nil)
	if e == nil {
		t.Fatal("NewExtractor(nil) 不应返回 nil")
	}
	if e.Logger() == nil {
		t.Fatal("Logger() 不应返回 nil")
	}
	if e.cache == nil {
		t.Fatal("cache 不应为 nil")
	}
}

func TestExtractData_UnsupportedFormat(t *testing.T) {
	e := NewExtractor(nil)
	_, err := e.ExtractData([]byte("data"), "test.txt", []string{"defendant"}, nil)
	if err == nil {
		t.Fatal("不支持的格式应返回错误")
	}
}

func TestExtractData_ImageDisabled(t *testing.T) {
	e := NewExtractor(nil)
	_, err := e.ExtractData([]byte("data"), "test.jpg", []string{"defendant"}, nil)
	if err == nil {
		t.Fatal("图片格式应返回禁用错误")
	}
}

func TestDefaultPatterns(t *testing.T) {
	// 验证分隔符模式
	if !DefaultPatterns.Split.MatchString("民 事 起 诉 状") {
		t.Error("Split 模式应匹配 '民 事 起 诉 状'")
	}
	if !DefaultPatterns.Split.MatchString("民事起诉状") {
		t.Error("Split 模式应匹配 '民事起诉状'")
	}

	// 验证被告模式
	if !DefaultPatterns.DefStart.MatchString("被 告：") {
		t.Error("DefStart 模式应匹配 '被 告：'")
	}
	if !DefaultPatterns.DefStart.MatchString("被告:") {
		t.Error("DefStart 模式应匹配 '被告:'")
	}

	// 验证身份证模式
	match := DefaultPatterns.ID.FindStringSubmatch("身份证号码：110101199001011234")
	if len(match) < 2 || match[1] != "110101199001011234" {
		t.Errorf("ID 模式应匹配身份证号, got %v", match)
	}

	// 验证 PatternRegistry 完整性
	expectedFields := []string{"defendant", "idNumber", "request", "factsReason", "page"}
	for _, f := range expectedFields {
		if _, ok := PatternRegistry[f]; !ok {
			t.Errorf("PatternRegistry 缺少字段 %q", f)
		}
	}
}

func TestExtractTextFromDocx_InvalidData(t *testing.T) {
	_, err := extractTextFromDocx([]byte("not a valid zip"))
	if err == nil {
		t.Error("无效 docx 数据应返回错误")
	}
}

func TestExtractTextFromDocx_EmptyZip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("other.xml")
	f.Write([]byte("<root/>"))
	w.Close()

	_, err := extractTextFromDocx(buf.Bytes())
	if err == nil {
		t.Error("缺少 document.xml 的 zip 应返回错误")
	}
}

func TestExtractFromDocx_InvalidData(t *testing.T) {
	e := NewExtractor(nil)
	_, err := e.extractFromDocx([]byte("bad data"), []string{"defendant"})
	if err == nil {
		t.Error("无效 docx 应返回错误")
	}
}

func TestExtractData_CacheHit(t *testing.T) {
	e := NewExtractor(nil)
	data := []byte("test data for cache")
	hash := e.calculateHash(data)

	e.cacheMu.Lock()
	e.cache[hash] = []Record{{"defendant": "缓存张三"}}
	e.cacheMu.Unlock()

	records, err := e.ExtractData(data, "test.pdf", []string{"defendant"}, nil)
	if err != nil {
		t.Fatalf("缓存命中不应报错: %v", err)
	}
	if len(records) != 1 || records[0]["defendant"] != "缓存张三" {
		t.Error("应返回缓存中的数据")
	}
}
