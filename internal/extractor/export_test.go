package extractor

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExportCSV(t *testing.T) {
	records := []Record{
		{
			"defendant":   "张三",
			"idNumber":    "110101199001011234",
			"request":     "偿还借款10000元",
			"factsReason": "2023年1月借款",
		},
		{
			"defendant":   "李四",
			"idNumber":    "220102198805061234",
			"request":     "偿还货款50000元",
			"factsReason": "2023年6月拖欠",
		},
	}

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_output.csv")

	if err := ExportCSV(csvPath, records); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Fatal("CSV 文件未创建")
	}

	// 读取并验证内容
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("打开 CSV 失败: %v", err)
	}
	defer f.Close()

	// 跳过 BOM
	bom := make([]byte, 3)
	f.Read(bom)

	reader := csv.NewReader(f)
	allRows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("读取 CSV 失败: %v", err)
	}

	// 表头 + 2 行数据
	if len(allRows) != 3 {
		t.Fatalf("CSV 行数 = %d, want 3", len(allRows))
	}

	// 验证表头是中文
	if allRows[0][0] != "被告" {
		t.Errorf("表头第一列 = %q, want %q", allRows[0][0], "被告")
	}

	// 验证数据
	if allRows[1][0] != "张三" {
		t.Errorf("第一行被告 = %q, want %q", allRows[1][0], "张三")
	}
}

func TestExportCSV_EmptyRecords(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "empty.csv")

	if err := ExportCSV(csvPath, []Record{}); err != nil {
		t.Fatalf("ExportCSV() with empty records error = %v", err)
	}

	info, err := os.Stat(csvPath)
	if err != nil {
		t.Fatal("文件未创建")
	}
	// 空记录应该只有 BOM
	if info.Size() > 10 {
		t.Errorf("空记录 CSV 文件不应包含数据行, size = %d", info.Size())
	}
}

func TestExportJSON(t *testing.T) {
	records := []Record{
		{
			"defendant": "张三",
			"idNumber":  "110101199001011234",
		},
	}

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "test_output.json")

	if err := ExportJSON(jsonPath, records); err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	// 读取并验证 JSON 结构
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("读取 JSON 失败: %v", err)
	}

	var parsed []map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("JSON 记录数 = %d, want 1", len(parsed))
	}

	if parsed[0]["defendant"] != "张三" {
		t.Errorf("defendant = %q, want %q", parsed[0]["defendant"], "张三")
	}
}

func TestExportExcel(t *testing.T) {
	records := []Record{
		{
			"defendant":   "张三",
			"idNumber":    "110101199001011234",
			"request":     "偿还借款",
			"factsReason": "被告于2023年借款",
		},
	}

	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "test_output.xlsx")

	if err := ExportExcel(xlsxPath, records); err != nil {
		t.Fatalf("ExportExcel() error = %v", err)
	}

	// 读取 Excel 文件验证
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("打开 Excel 失败: %v", err)
	}
	defer f.Close()

	// 验证表头
	header, err := f.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("读取 A1 失败: %v", err)
	}
	if header != "被告" {
		t.Errorf("A1 = %q, want %q", header, "被告")
	}

	// 验证数据
	val, err := f.GetCellValue("Sheet1", "A2")
	if err != nil {
		t.Fatalf("读取 A2 失败: %v", err)
	}
	if val != "张三" {
		t.Errorf("A2 = %q, want %q", val, "张三")
	}
}

func TestExportExcel_EmptyRecords(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "empty.xlsx")

	// 空记录时代码提前返回 nil，不创建文件，这是预期行为
	if err := ExportExcel(xlsxPath, []Record{}); err != nil {
		t.Fatalf("ExportExcel() with empty records error = %v", err)
	}
}
