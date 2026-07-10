package extractor

import (
	"encoding/csv"
	"encoding/json"
	"os"

	"github.com/xuri/excelize/v2"
)

// exportColumnOrder 是导出列的固定顺序，CSV 与 Excel 保持一致口径。
var exportColumnOrder = []string{"page", "defendant", "idNumber", "request", "factsReason"}

// exportHeaders 扫描全部记录的字段并集，按固定顺序返回列 key 与中文表头。
// 记录字段天然异构（仅匹配成功的字段才会写入），因此不能只看第一条记录，
// 否则第一条缺失的字段会导致整列静默丢失。
func exportHeaders(records []Record) (keys []string, headers []string) {
	present := make(map[string]bool)
	for _, r := range records {
		for k := range r {
			present[k] = true
		}
	}

	for _, k := range exportColumnOrder {
		if present[k] {
			keys = append(keys, k)
			headers = append(headers, PatternRegistry[k].Label)
		}
	}
	return keys, headers
}

func writeCSV(path string, records []Record) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString("\xEF\xBB\xBF"); err != nil { // BOM for Excel
		return err
	}

	w := csv.NewWriter(file)
	defer w.Flush()

	if len(records) == 0 {
		return nil
	}

	keys, headers := exportHeaders(records)
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, r := range records {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = r[k]
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// ExportCSV exports records to a CSV file
func ExportCSV(path string, records []Record) error {
	return writeCSV(path, records)
}

// ExportJSON exports records to a JSON file
func ExportJSON(path string, records []Record) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

// ExportExcel exports records to an Excel file
func ExportExcel(path string, records []Record) error {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	// Create a new sheet.
	sheetName := "Sheet1"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}

	// Set active sheet of the workbook.
	f.SetActiveSheet(index)

	if len(records) == 0 {
		return nil
	}

	keys, headers := exportHeaders(records)

	// Set headers
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return err
		}
	}

	// Set values
	wrapStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			WrapText: true,
			Vertical: "top",
		},
	})

	for i, r := range records {
		row := i + 2
		for j, k := range keys {
			cell, err := excelize.CoordinatesToCellName(j+1, row)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheetName, cell, r[k]); err != nil {
				return err
			}
			// Apply wrap text style
			if err := f.SetCellStyle(sheetName, cell, cell, wrapStyle); err != nil {
				return err
			}
		}
	}

	// 按字段设置列宽提高可读性（长文本字段更宽）
	columnWidths := map[string]float64{
		"page":        8,
		"defendant":   20,
		"idNumber":    24,
		"request":     50,
		"factsReason": 50,
	}
	for i, k := range keys {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if width, ok := columnWidths[k]; ok {
			if err := f.SetColWidth(sheetName, col, col, width); err != nil {
				return err
			}
		}
	}

	if err := f.SaveAs(path); err != nil {
		return err
	}
	return nil
}
