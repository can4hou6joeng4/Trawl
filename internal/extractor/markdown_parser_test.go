package extractor

import (
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "去除 div 标签",
			input: "<div>内容</div>",
			want:  "内容",
		},
		{
			name:  "去除嵌套标签",
			input: "<div><p>段落<img src='x'/></p></div>",
			want:  "段落",
		},
		{
			name:  "无标签原样返回",
			input: "纯文本内容",
			want:  "纯文本内容",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "混合 HTML 与文本",
			input: "被告：<b>张三</b>，身份证号：<span>110101199001011234</span>",
			want:  "被告：张三，身份证号：110101199001011234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "去除标题符号",
			input: "## 诉讼请求：请求偿还借款",
			want:  "诉讼请求：请求偿还借款",
		},
		{
			name:  "去除粗体符号",
			input: "**重要内容**普通内容",
			want:  "重要内容普通内容",
		},
		{
			name:  "去除表格线",
			input: "字段一|字段二|字段三",
			want:  "字段一 字段二 字段三",
		},
		{
			name:  "去除关键词头部_事实与理由",
			input: "事实与理由：被告于2023年借款",
			want:  "被告于2023年借款",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("cleanMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractField(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		keyword string
		want    string
	}{
		{
			name:    "冒号后直接取值",
			text:    "被告：张三\n性别：男",
			keyword: "被告",
			want:    "张三",
		},
		{
			name:    "英文冒号取值",
			text:    "被告:李四\n住址：北京",
			keyword: "被告",
			want:    "李四",
		},
		{
			name:    "值在下一行",
			text:    "被告：\n王五",
			keyword: "被告",
			want:    "王五",
		},
		{
			name:    "关键词不存在返回空",
			text:    "原告：赵六",
			keyword: "被告",
			want:    "",
		},
		{
			name:    "含 DefEnd 后缀自动清理",
			text:    "被告：张三，性别男",
			keyword: "被告",
			want:    "张三",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractField(tt.text, tt.keyword)
			if got != tt.want {
				t.Errorf("extractField(%q, %q) = %q, want %q", tt.text, tt.keyword, got, tt.want)
			}
		})
	}
}

func TestParseMarkdown(t *testing.T) {
	tests := []struct {
		name       string
		markdown   string
		wantLen    int
		wantFields map[string]string
	}{
		{
			name:    "空字符串返回 nil",
			markdown: "",
			wantLen: 0,
		},
		{
			name: "标准法律文书 Markdown",
			markdown: `## 民事起诉状

被告：张三
身份证号码：110101199001011234

## 诉讼请求
1. 请求判令被告偿还借款10000元。

## 事实与理由
2023年1月1日，被告向原告借款...

此致
北京市朝阳区人民法院`,
			wantLen: 1,
			wantFields: map[string]string{
				"defendant": "张三",
				"idNumber":  "110101199001011234",
			},
		},
		{
			name: "含 HTML 标签的 Markdown",
			markdown: `<div>被告：李四</div>
<p>身份证号码：220102198805061234</p>

## 诉讼请求
请求偿还欠款

## 事实与理由
被告拖欠货款

此致`,
			wantLen: 1,
			wantFields: map[string]string{
				"defendant": "李四",
				"idNumber":  "220102198805061234",
			},
		},
		{
			name:     "无有效信息返回 nil",
			markdown: "这是一段无关的文本，不包含任何法律文书信息。",
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMarkdown(tt.markdown)
			if len(got) != tt.wantLen {
				t.Fatalf("ParseMarkdown() returned %d records, want %d", len(got), tt.wantLen)
			}
			if tt.wantFields != nil && len(got) > 0 {
				for k, v := range tt.wantFields {
					if got[0][k] != v {
						t.Errorf("field %q = %q, want %q", k, got[0][k], v)
					}
				}
			}
		})
	}
}
