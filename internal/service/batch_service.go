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

// BatchRules 是一批图片的处理规则；0 表示关闭该项。
type BatchRules struct {
	OutputFormat string `json:"output_format"`
	TargetBytes  int64  `json:"target_bytes"`
	MaxWidth     int    `json:"max_width"`
	MaxHeight    int    `json:"max_height"`
}

// validateBatchStart 是「能不能开始一个批次」的完整判定（US-11）：范围合法 + 至少一条有效规则。
//
// 「至少一条」只约束开跑，不约束保存预设/上次设置——用户可能先把空规则存下来。
func validateBatchStart(rules BatchRules) error {
	if err := validateBatchRules(rules); err != nil {
		return err
	}
	if rules.OutputFormat == imageproc.OutputKeep && rules.TargetBytes == 0 && rules.MaxWidth == 0 {
		return apperr.Validation("请选择输出格式，或开启目标体积/尺寸限制")
	}
	return nil
}

// ValidateImageRules 给前端做「开始处理」之前的预检：合法返回空串，否则返回
// 可直接展示的提示文案。
//
// 为什么要有这个绑定（005→011 累计审查的修复）：此前前端在 ImageWorkbench.vue 里
// 自己比较 targetValid/sizeBoxValid 并抄了一份中文提示，与 Go 的 validateBatchRules
// 是【同一条规则的两个实现】，而且已经漂移——Go 有「宽高需同时设置」，前端没有，
// 用户看到的原因与真实被拒的原因不一致。铁律 1：镜像只许复制常量与类型，不许复制逻辑。
func (s *Service) ValidateImageRules(rules BatchRules) string {
	if err := validateBatchStart(rules); err != nil {
		return err.Error()
	}
	return ""
}

// validateBatchRules 校验规则取值范围；批次、预设与上次设置共用同一套规则。
func validateBatchRules(rules BatchRules) error {
	if err := imageproc.ValidateOutputFormat(rules.OutputFormat); err != nil {
		return err
	}
	if rules.TargetBytes != 0 &&
		(rules.TargetBytes < TargetBytesMin || rules.TargetBytes > TargetBytesMax) {
		return apperr.Validation("目标体积需在 1 KB–1 GB")
	}
	if (rules.MaxWidth == 0) != (rules.MaxHeight == 0) {
		return apperr.Validation("宽高需同时设置")
	}
	if rules.MaxWidth != 0 &&
		(rules.MaxWidth < BoxMinDimension || rules.MaxWidth > BoxMaxDimension ||
			rules.MaxHeight < BoxMinDimension || rules.MaxHeight > BoxMaxDimension) {
		return apperr.Validation("宽高需在 1–65535 px")
	}
	return nil
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
	// StatusProcessing 表示该项正在被某个 worker 处理（US-14 状态机的中间格）。
	// 只经进度事件下发、不会出现在最终 BatchResult 里。
	StatusProcessing ItemStatus = "processing"
)

// ItemResult 是单个文件的处理结果。
type ItemResult struct {
	Path        string     `json:"path"`
	OutputPath  string     `json:"output_path"`
	Status      ItemStatus `json:"status"`
	SourceBytes int64      `json:"source_bytes"`
	OutputBytes int64      `json:"output_bytes"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	// ExtremeDegraded 由引擎判定（阈值单一真相在 imageproc），前端只展示不再判断。
	ExtremeDegraded bool `json:"extreme_degraded"`
	// AlphaFlattened 表示透明图转 JPEG 时铺了白底（US-20 要求队列给出提示）。
	AlphaFlattened bool   `json:"alpha_flattened"`
	Message        string `json:"message,omitempty"`
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
	// Started 是刚被某个 worker 领取的项（Status=processing）。
	// 为什么需要它：默认 2 个 worker 并行，前端无法从「完成」事件反推哪些项正在跑，
	// US-14 的「排队 → 处理中 → 结果」状态机只能由 Go 在开工时播报。
	Started *ItemResult `json:"started,omitempty"`
}

// ProcessBatch 以最多两个 worker 转换一批图片；取消在当前文件完成后生效。
// emit 可为 nil；非 nil 时按完成顺序推送进度。
func (s *Service) ProcessBatch(req BatchRequest, emit func(BatchProgress)) (BatchResult, error) {
	if len(req.Paths) == 0 {
		return BatchResult{}, apperr.Validation("没有可处理的图片")
	}
	if err := validateBatchStart(req.Rules); err != nil {
		return BatchResult{}, err
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
				path := req.Paths[idx]
				// 开工先播报「处理中」——进度事件是唯一的状态来源，前端不自己猜。
				progressMu.Lock()
				if emit != nil {
					emit(BatchProgress{Done: done, Total: total,
						Started: &ItemResult{Path: path, Status: StatusProcessing}})
				}
				progressMu.Unlock()

				res := s.convertOne(path, req.Rules)
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

// convertOne 处理单个文件，并按 US-26 把每一次失败写进日志。
//
// 为什么单独包一层：失败原因此前只回到前端做展示，日志里没有——用户报「有几张坏了」
// 时无法事后核对。日志只记路径与原因，**不记图片内容**（US-26 明确要求，也避免
// 把素材写进日志目录）。
func (s *Service) convertOne(path string, rules BatchRules) ItemResult {
	res := s.convertOnce(path, rules)
	if res.Status == StatusFailed {
		slog.Warn("图片处理失败", "path", path, "reason", res.Message)
	}
	return res
}

func (s *Service) convertOnce(path string, rules BatchRules) ItemResult {
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
	res.AlphaFlattened = converted.AlphaFlattened
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
