package extractor

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// collectPageRecords 是本地文本层与 WinOCR 两条 PDF 路径共用的页级并发骨架：
// 固定数量 worker 逐页调用 processPage，为结果标注页码，聚合后按页码排序返回。
//
// 取消语义：worker 命中取消后不再处理剩余页（只回填占位结果尽快清空队列），
// 聚合循环命中取消立即排空通道并返回 ErrCancelled。
func (e *Extractor) collectPageRecords(
	ctx context.Context,
	totalPages, numWorkers int,
	processPage func(pageNum int) []Record,
	onProgress ProgressCallback,
	progressMsg func(pageNum int) string,
) ([]Record, error) {
	if numWorkers > totalPages {
		numWorkers = totalPages
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	type pageResult struct {
		pageNum int
		records []Record
	}

	jobs := make(chan int, totalPages)
	results := make(chan pageResult, totalPages)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNum := range jobs {
				if ctx.Err() != nil {
					results <- pageResult{pageNum: pageNum}
					continue
				}
				records := processPage(pageNum)
				for _, rec := range records {
					rec["page"] = fmt.Sprintf("%d", pageNum)
				}
				results <- pageResult{pageNum: pageNum, records: records}
			}
		}()
	}

	go func() {
		for i := 1; i <= totalPages; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var allPageResults []pageResult
	processed := 0
	for res := range results {
		if err := ctx.Err(); err != nil {
			// drain remaining results to avoid goroutine leak, then bail
			go func() {
				for range results {
				}
			}()
			return nil, ErrCancelled
		}
		processed++
		if onProgress != nil {
			onProgress(processed, totalPages, progressMsg(res.pageNum))
		}
		if len(res.records) > 0 {
			allPageResults = append(allPageResults, res)
		}
	}

	// 按照页码排序，保证输出顺序一致
	sort.Slice(allPageResults, func(i, j int) bool {
		return allPageResults[i].pageNum < allPageResults[j].pageNum
	})

	var finalRecords []Record
	for _, pr := range allPageResults {
		finalRecords = append(finalRecords, pr.records...)
	}
	return finalRecords, nil
}
