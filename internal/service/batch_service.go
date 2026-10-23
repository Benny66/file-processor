package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

const batchWorkerCount = 2

// 尺寸盒宽高范围（单一真相；前端镜像在 frontend/src/lib/image.ts）。
const (
	BoxMinDimension = 1
	BoxMaxDimension = 65535
)

// 目标体积范围：1 KB–1 GB（单一真相；前端镜像在 frontend/src/lib/image.ts）。
const (
	TargetBytesMin int64 = 1024
	TargetBytesMax int64 = 1073741824
)

// BatchRules 是一批图片的处理规则；目标体积与尺寸字段在后续切片实现。
type BatchRules struct {
	OutputFormat string `json:"output_format"`
	TargetBytes  int64  `json:"target_bytes"`
	MaxWidth     int    `json:"max_width"`
	MaxHeight    int    `json:"max_height"`
}

// BatchRequest 是一次批量处理请求。
type BatchRequest struct {
	Paths []string   `json:"paths"`
	Rules BatchRules `json:"rules"`
}

// ItemStatus 是单个文件的处理结果状态。
type ItemStatus string

// 单个文件的处理状态（单一真相；前端镜像在 frontend/src/lib/image.ts）。
const (
	StatusSuccess  ItemStatus = "success"
	StatusFailed   ItemStatus = "failed"
	StatusSkipped  ItemStatus = "skipped"
	StatusCanceled ItemStatus = "canceled"
)

// ItemResult 是单个文件的处理结果。
type ItemResult struct {
	Path            string     `json:"path"`
	OutputPath      string     `json:"output_path"`
	Status          ItemStatus `json:"status"`
	SourceBytes     int64      `json:"source_bytes"`
	OutputBytes     int64      `json:"output_bytes"`
	Width           int        `json:"width"`
	Height          int        `json:"height"`
	ExtremeDegraded bool       `json:"extreme_degraded"`
	Message         string     `json:"message,omitempty"`
}

// BatchResult 是一个批次的汇总结果。
type BatchResult struct {
	OK              bool         `json:"ok"`
	Message         string       `json:"message,omitempty"`
	Items           []ItemResult `json:"items"`
	Success         int          `json:"success"`
	Failed          int          `json:"failed"`
	Skipped         int          `json:"skipped"`
	Canceled        int          `json:"canceled"`
	ExtremeDegraded int          `json:"extreme_degraded"`
	SavedBytes      int64        `json:"saved_bytes"`
}

// BatchProgress 是批次进度事件载荷。
type BatchProgress struct {
	Done    int         `json:"done"`
	Total   int         `json:"total"`
	Current *ItemResult `json:"current,omitempty"`
}

// ProcessBatch 以最多两个 worker 转换一批图片；取消在当前文件完成后生效。
// emit 可为 nil；非 nil 时按完成顺序推送进度。
func (s *Service) ProcessBatch(req BatchRequest, emit func(BatchProgress)) (BatchResult, error) {
	if len(req.Paths) == 0 {
		return BatchResult{}, apperr.Validation("没有可处理的图片")
	}
	if err := imageproc.ValidateOutputFormat(req.Rules.OutputFormat); err != nil {
		return BatchResult{}, err
	}
	if req.Rules.TargetBytes != 0 &&
		(req.Rules.TargetBytes < TargetBytesMin || req.Rules.TargetBytes > TargetBytesMax) {
		return BatchResult{}, apperr.Validation("目标体积需在 1 KB–1 GB")
	}
	if (req.Rules.MaxWidth == 0) != (req.Rules.MaxHeight == 0) {
		return BatchResult{}, apperr.Validation("宽高需同时设置")
	}
	if req.Rules.MaxWidth != 0 &&
		(req.Rules.MaxWidth < BoxMinDimension || req.Rules.MaxWidth > BoxMaxDimension ||
			req.Rules.MaxHeight < BoxMinDimension || req.Rules.MaxHeight > BoxMaxDimension) {
		return BatchResult{}, apperr.Validation("宽高需在 1–65535 px")
	}

	s.batchMu.Lock()
	if s.batchRunning {
		s.batchMu.Unlock()
		return BatchResult{}, apperr.Conflict("已有批次正在处理")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.batchRunning = true
	s.batchCancel = cancel
	s.batchMu.Unlock()
	defer func() {
		cancel()
		s.batchMu.Lock()
		s.batchRunning = false
		s.batchCancel = nil
		s.batchMu.Unlock()
	}()

	total := len(req.Paths)
	results := make([]ItemResult, total)
	jobs := make(chan int, total)
	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)

	workers := batchWorkerCount
	if total < workers {
		workers = total
	}
	var (
		wg         sync.WaitGroup
		progressMu sync.Mutex
		done       int
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				res := s.convertOne(req.Paths[idx], req.Rules)
				results[idx] = res
				progressMu.Lock()
				done++
				if emit != nil {
					current := res
					emit(BatchProgress{Done: done, Total: total, Current: &current})
				}
				progressMu.Unlock()
			}
		}()
	}
	wg.Wait()

	for i := range results {
		if results[i].Status == "" {
			results[i] = ItemResult{Path: req.Paths[i], Status: StatusCanceled, Message: "已取消"}
		}
	}

	summary := BatchResult{Items: results}
	for _, item := range results {
		switch item.Status {
		case StatusSuccess:
			summary.Success++
			if item.ExtremeDegraded {
				summary.ExtremeDegraded++
			}
			summary.SavedBytes += item.SourceBytes - item.OutputBytes
		case StatusFailed:
			summary.Failed++
		case StatusSkipped:
			summary.Skipped++
		case StatusCanceled:
			summary.Canceled++
		}
	}
	summary.OK = summary.Failed == 0 && summary.Canceled == 0
	if summary.Canceled > 0 {
		summary.Message = "批次已取消"
	} else if summary.Failed > 0 {
		summary.Message = "批次完成，部分文件失败"
	}
	return summary, nil
}

// CancelBatch 请求取消当前批次；没有批次时安全无操作。
func (s *Service) CancelBatch() {
	s.batchMu.Lock()
	cancel := s.batchCancel
	s.batchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) convertOne(path string, rules BatchRules) ItemResult {
	res := ItemResult{Path: path}
	stat, err := os.Stat(path)
	if err != nil {
		res.Status = StatusFailed
		res.Message = "无法读取图片"
		return res
	}
	res.SourceBytes = stat.Size()
	if !stat.Mode().IsRegular() {
		res.Status = StatusFailed
		res.Message = "不是普通文件"
		return res
	}
	if stat.Size() > imageproc.MaxFileBytes {
		res.Status = StatusFailed
		res.Message = "图片超过 512 MB 上限"
		return res
	}
	data, err := os.ReadFile(path)
	if err != nil {
		res.Status = StatusFailed
		res.Message = "无法读取图片"
		return res
	}
	res.SourceBytes = int64(len(data))

	probe, err := imageproc.ProbeImage(data, path)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	outFormat, err := imageproc.NormalizeOutputFormat(rules.OutputFormat, probe.Format)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	boxEnabled := rules.MaxWidth > 0
	targetOK := rules.TargetBytes == 0 || res.SourceBytes <= rules.TargetBytes
	boxOK := !boxEnabled || (probe.Width <= rules.MaxWidth && probe.Height <= rules.MaxHeight)
	if outFormat == probe.Format && targetOK && boxOK {
		res.Status = StatusSkipped
		res.Message = "已达标"
		res.Width = probe.Width
		res.Height = probe.Height
		return res
	}

	converted, err := imageproc.Convert(data, path, imageproc.ConvertOptions{
		Format:      rules.OutputFormat,
		MaxWidth:    rules.MaxWidth,
		MaxHeight:   rules.MaxHeight,
		TargetBytes: rules.TargetBytes,
	})
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	ext, err := outputExtension(converted.Format)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	dir := filepath.Dir(path)
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	finalPath, err := writeConvertedFile(dir, stem, ext, converted.Data)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("写入输出失败: %v", err)
		return res
	}
	res.Status = StatusSuccess
	res.OutputPath = finalPath
	res.OutputBytes = int64(len(converted.Data))
	res.Width = converted.Width
	res.Height = converted.Height
	res.ExtremeDegraded = converted.ExtremeDegraded
	return res
}

// writeConvertedFile 先写同目录临时文件，再用硬链接以不覆盖方式落位。
func writeConvertedFile(dir, stem, ext string, data []byte) (string, error) {
	temp, err := os.CreateTemp(dir, ".imageworkshop-*.tmp")
	if err != nil {
		return "", err
	}
	tempName := temp.Name()
	defer func() {
		if err := os.Remove(tempName); err != nil && !os.IsNotExist(err) {
			slog.Warn("清理输出临时文件失败", "path", tempName, "err", err)
		}
	}()

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	return placeOutputFile(tempName, dir, stem, ext)
}

// placeOutputFile 通过硬链接原子占位：目标存在时继续尝试下一个序号，绝不覆盖。
func placeOutputFile(tempPath, dir, stem, ext string) (string, error) {
	for i := 1; ; i++ {
		suffix := "-out"
		if i > 1 {
			suffix = fmt.Sprintf("-out-%d", i)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		if err := os.Link(tempPath, candidate); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", err
		}
		return candidate, nil
	}
}

func outputExtension(format string) (string, error) {
	switch format {
	case imageproc.OutputJPEG:
		return ".jpg", nil
	case imageproc.OutputPNG:
		return ".png", nil
	case imageproc.OutputWebP:
		return ".webp", nil
	default:
		return "", fmt.Errorf("未知输出格式: %s", format)
	}
}
