package extractor

import (
	"bytes"
	"image"
	"math"

	_ "image/jpeg" // 注册解码器：图片路径的尺寸兜底探测
	_ "image/png"
)

// OcrLine 云端 OCR 识别出的一行文字，坐标已归一化到 [0,1]
// （相对该页栅格图尺寸），前端乘以显示尺寸即可绝对定位。
type OcrLine struct {
	Text string  `json:"text"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

// OcrPageLines 一页的 OCR 文字行；Page 为全文档 1-based 页码。
type OcrPageLines struct {
	Page  int       `json:"page"`
	Lines []OcrLine `json:"lines"`
}

// ExtractOutput 一次提取的完整产物。
// OcrPages 仅云端 OCR 路径产出（扫描件/图片的坐标高亮数据源）；
// 本地文本层、DOCX、WinOCR 路径为 nil——文本层 PDF 前端另有更精确的
// pdf.js 坐标，WinOCR 桥不吐坐标，两者由前端按既有方式处理。
type ExtractOutput struct {
	Records  []Record       `json:"records"`
	OcrPages []OcrPageLines `json:"ocrPages,omitempty"`
}

// buildOcrLines 把一页的 rec_texts 与 rec_boxes（或 rec_polys 兜底）
// 组装为归一化文字行。任一前提不满足（数量不齐、页尺寸非法）返回 nil——
// 宁缺毋错：没有坐标时前端回退 REC 角标，绝不画错位置。
func buildOcrLines(texts []string, boxes [][]float64, polys [][][]float64, pageW, pageH float64) []OcrLine {
	if len(texts) == 0 || pageW <= 0 || pageH <= 0 {
		return nil
	}

	// 每行取包围盒 [x1,y1,x2,y2]：优先 rec_boxes，数量不齐时尝试 rec_polys
	rects := make([][4]float64, 0, len(texts))
	switch {
	case len(boxes) == len(texts):
		for _, b := range boxes {
			if len(b) < 4 {
				return nil
			}
			rects = append(rects, [4]float64{b[0], b[1], b[2], b[3]})
		}
	case len(polys) == len(texts):
		for _, poly := range polys {
			if len(poly) == 0 {
				return nil
			}
			minX, minY := math.Inf(1), math.Inf(1)
			maxX, maxY := math.Inf(-1), math.Inf(-1)
			for _, pt := range poly {
				if len(pt) < 2 {
					return nil
				}
				minX = math.Min(minX, pt[0])
				minY = math.Min(minY, pt[1])
				maxX = math.Max(maxX, pt[0])
				maxY = math.Max(maxY, pt[1])
			}
			rects = append(rects, [4]float64{minX, minY, maxX, maxY})
		}
	default:
		return nil
	}

	clamp01 := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	lines := make([]OcrLine, 0, len(texts))
	for i, text := range texts {
		if text == "" {
			continue
		}
		r := rects[i]
		x1, y1 := clamp01(r[0]/pageW), clamp01(r[1]/pageH)
		x2, y2 := clamp01(r[2]/pageW), clamp01(r[3]/pageH)
		w, h := x2-x1, y2-y1
		if w <= 0 || h <= 0 || math.IsNaN(w) || math.IsNaN(h) {
			continue // 退化框：跳过该行，不影响其余行
		}
		lines = append(lines, OcrLine{Text: text, X: x1, Y: y1, W: w, H: h})
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// imageDims 本地探测图片尺寸（仅读文件头），供响应缺失 dataInfo 时兜底。
func imageDims(fileData []byte) (w, h float64, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(fileData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return float64(cfg.Width), float64(cfg.Height), true
}

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// isPng 按魔数判断内容是否为 PNG（无 EXIF 方向元数据，尺寸兜底安全）。
func isPng(fileData []byte) bool {
	return bytes.HasPrefix(fileData, pngMagic)
}
