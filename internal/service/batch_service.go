package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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

// OutputPatternDefault 是文件名格式的缺省值（即历史的 <原名>-out）。
const OutputPatternDefault = "{name}-out"

// BatchRules 是一批图片的处理规则；0 表示关闭该项。
type BatchRules struct {
	OutputFormat string `json:"output_format"`
	TargetBytes  int64  `json:"target_bytes"`
	MaxWidth     int    `json:"max_width"`
	MaxHeight    int    `json:"max_height"`
}

// BatchItem 是批次里的一张图：绝对路径 + 导入根下的相对路径。
// RelPath 只为下载「保留文件夹层级」服务（US-60）；平铺下载时用不到。
type BatchItem struct {
	Path    string `json:"path"`
	RelPath string `json:"rel_path"`
}

// validateBatchStart 是「能不能开始一个批次」的完整判定（US-11）：范围合法 + 至少一条有效规则。
//
// 「至少一条」只约束开跑，不约束保存常用设置/上次设置——用户可能先把空规则存下来。
// hasImages=false（纯 PDF 批）时三条规则全部无用武之地，整段校验让路（compress-pdf）。
func validateBatchStart(rules BatchRules, hasImages bool) error {
	if !hasImages {
		return nil
	}
	if err := validateBatchRules(rules); err != nil {
		return err
	}
	if rules.OutputFormat == imageproc.OutputKeep && rules.TargetBytes == 0 && rules.MaxWidth == 0 {
		return apperr.Validation("请至少选一项：转格式、压到指定大小或限制尺寸")
	}
	return nil
}

// ValidateImageRules 给前端做「开始处理」之前的预检：合法返回空串，否则返回
// 可直接展示的提示文案。hasImages 表示队列里是否有图片项——纯 PDF 批免规则校验。
//
// 为什么要有这个绑定（005→011 累计审查的修复）：此前前端在 ImageWorkbench.vue 里
// 自己比较 targetValid/sizeBoxValid 并抄了一份中文提示，与 Go 的 validateBatchRules
// 是【同一条规则的两个实现】，而且已经漂移——Go 有「宽高需同时设置」，前端没有，
// 用户看到的原因与真实被拒的原因不一致。铁律 1：镜像只许复制常量与类型，不许复制逻辑。
func (s *Service) ValidateImageRules(rules BatchRules, hasImages bool) string {
	if err := validateBatchStart(rules, hasImages); err != nil {
		return err.Error()
	}
	return ""
}

// validatePattern 校验文件名格式（下载与命名示例共用；空串 = 默认 "{name}-out"）。
func validatePattern(pattern string) error {
	if pattern == "" {
		pattern = OutputPatternDefault
	}
	if len(pattern) > 64 {
		return apperr.Validation("文件名格式最长 64 个字符")
	}
	if !strings.Contains(pattern, "{name}") && !strings.Contains(pattern, "{n}") {
		return apperr.Validation("文件名格式要包含 {name} 或 {n}")
	}
	if strings.ContainsAny(pattern, `/\`) || strings.Contains(pattern, "..") {
		return apperr.Validation("文件名格式不能包含 / 或 ..")
	}
	return nil
}

// renderOutputStem 把命名模板渲染成输出词干；未知占位符原样保留（如 "{ext}" 视作字面文本）。
func renderOutputStem(pattern, stem string, index int) string {
	if pattern == "" {
		pattern = OutputPatternDefault
	}
	out := strings.ReplaceAll(pattern, "{name}", stem)
	return strings.ReplaceAll(out, "{n}", strconv.Itoa(index))
}

// validateBatchRules 校验规则取值范围；批次、预设与上次设置共用同一套规则。
func validateBatchRules(rules BatchRules) error {
	if err := imageproc.ValidateOutputFormat(rules.OutputFormat); err != nil {
		return err
	}
	if rules.TargetBytes != 0 &&
		(rules.TargetBytes < TargetBytesMin || rules.TargetBytes > TargetBytesMax) {
		return apperr.Validation("大小要在 1 KB 到 1 GB 之间")
	}
	if (rules.MaxWidth == 0) != (rules.MaxHeight == 0) {
		return apperr.Validation("宽和高要一起填")
	}
	if rules.MaxWidth != 0 &&
		(rules.MaxWidth < BoxMinDimension || rules.MaxWidth > BoxMaxDimension ||
			rules.MaxHeight < BoxMinDimension || rules.MaxHeight > BoxMaxDimension) {
		return apperr.Validation("宽和高要在 1 到 65535 之间")
	}
	return nil
}

// BatchRequest 是一次批量处理请求。Items 携带导入相对路径（下载保留层级用，US-60）。
type BatchRequest struct {
	Items []BatchItem `json:"items"`
	Rules BatchRules  `json:"rules"`
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
	Format      string     `json:"format,omitempty"`
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
	// BatchKey 本次处理在暂存区的目录名：下载与历史都以它为凭（US-59/60）。
	BatchKey string `json:"batch_key"`
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
	if len(req.Items) == 0 {
		return BatchResult{}, apperr.Validation("请先添加要处理的图片")
	}
	// 纯 PDF 批：三条规则都不适用，免「至少一条」门槛（compress-pdf）。
	// 混批时规则作用于图片项，照旧全量校验。
	hasImages := false
	for _, item := range req.Items {
		if !imageproc.IsPDFPath(item.Path) {
			hasImages = true
			break
		}
	}
	if err := validateBatchStart(req.Rules, hasImages); err != nil {
		return BatchResult{}, err
	}
	// US-59（ADR-0007）：处理只进暂存区，原图所在文件夹零写入；下载是唯一落盘出口。
	if s.historyRoot == "" {
		return BatchResult{}, apperr.Wrap(errors.New("暂存位置未初始化"))
	}
	batchKey := newBatchKey()
	stagingDir := s.stagingDir(batchKey)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return BatchResult{}, apperr.Validation("无法创建暂存位置")
	}

	s.batchMu.Lock()
	if s.batchRunning {
		s.batchMu.Unlock()
		return BatchResult{}, apperr.Conflict("正在处理中，请等它完成")
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

	total := len(req.Items)
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
				item := req.Items[idx]
				// 开工先播报「处理中」——进度事件是唯一的状态来源，前端不自己猜。
				progressMu.Lock()
				if emit != nil {
					emit(BatchProgress{Done: done, Total: total,
						Started: &ItemResult{Path: item.Path, Status: StatusProcessing}})
				}
				progressMu.Unlock()

				res := s.convertOne(ctx, item, idx+1, req.Rules, stagingDir)
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
			results[i] = ItemResult{Path: req.Items[i].Path, Status: StatusCanceled, Message: "已取消"}
		}
	}

	summary := BatchResult{Items: results, BatchKey: batchKey}
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
		summary.Message = "已取消"
	} else if summary.Failed > 0 {
		summary.Message = "处理完成，有文件没成功"
	}
	// 历史落库 best-effort：失败只记日志，不影响批次结果（历史页能看到本次记录）。
	s.persistHistory(batchKey, req, summary)
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
func (s *Service) convertOne(ctx context.Context, item BatchItem, index int, rules BatchRules, stagingDir string) ItemResult {
	res := s.convertOnce(ctx, item, index, rules, stagingDir)
	if res.Status == StatusFailed {
		slog.Warn("图片处理失败", "path", item.Path, "reason", res.Message)
	}
	return res
}

// compressPDFFunc 抽出 PDF 压缩调用，测试用它注入桩实现（沿用 linkOutputFile 的手法）。
var compressPDFFunc = imageproc.CompressPDF

func (s *Service) convertOnce(ctx context.Context, item BatchItem, index int, rules BatchRules, stagingDir string) ItemResult {
	path := item.Path
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
		res.Message = "图片超过 512 MB，处理不了"
		return res
	}
	data, err := os.ReadFile(path)
	if err != nil {
		res.Status = StatusFailed
		res.Message = "无法读取图片"
		return res
	}
	res.SourceBytes = int64(len(data))

	// PDF 走压缩管线（compress-pdf）：三条规则不适用，内容嗅探优先于图片探测。
	if imageproc.IsPDF(data) {
		return s.compressOnePDF(ctx, item, index, data, stagingDir)
	}

	probe, err := imageproc.ProbeImage(data, path)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	if shouldSkipConvert(rules, res.SourceBytes, probe.Format, probe.Width, probe.Height) {
		res.Status = StatusSkipped
		res.Message = "无需处理（原图已经合适）"
		res.Width = probe.Width
		res.Height = probe.Height
		return res
	}

	var converted imageproc.Converted
	if rules.OutputFormat == imageproc.OutputAuto {
		converted, err = convertBestFormat(data, path, rules)
	} else {
		converted, err = imageproc.Convert(data, path, imageproc.ConvertOptions{
			Format:      rules.OutputFormat,
			MaxWidth:    rules.MaxWidth,
			MaxHeight:   rules.MaxHeight,
			TargetBytes: rules.TargetBytes,
		})
	}
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	if rules.OutputFormat == imageproc.OutputAuto {
		res.Message = fmt.Sprintf("已自动选了最省空间的格式（%s）", formatDisplayName(converted.Format))
	}
	ext, err := outputExtension(converted.Format)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	// 暂存区命名用批次内序号（0001.jpg…）：区内天然无重名；用户可见的命名/防覆盖
	// 在下载时渲染（ADR-0007），处理阶段不再碰用户目录。
	stagingName := fmt.Sprintf("%04d%s", index, ext)
	finalPath, err := writeConvertedFile(stagingDir, strings.TrimSuffix(stagingName, ext), ext, converted.Data)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("保存到暂存区失败: %v", err)
		return res
	}
	res.Status = StatusSuccess
	res.OutputPath = finalPath
	res.OutputBytes = int64(len(converted.Data))
	res.Format = converted.Format
	res.Width = converted.Width
	res.Height = converted.Height
	res.ExtremeDegraded = converted.ExtremeDegraded
	res.AlphaFlattened = converted.AlphaFlattened
	return res
}

// compressOnePDF 处理单个 PDF：压缩并落暂存区（%04d.pdf，与图片同一套防覆盖落位）；
// 压不动时如实标「无需处理」，不产出文件——与图片「已达标」的行为对齐（compress-pdf）。
func (s *Service) compressOnePDF(ctx context.Context, item BatchItem, index int, data []byte, stagingDir string) ItemResult {
	res := ItemResult{Path: item.Path, SourceBytes: int64(len(data)), Format: "pdf"}
	out, stats, err := compressPDFFunc(ctx, data)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	if int64(len(out)) >= res.SourceBytes {
		res.Status = StatusSkipped
		res.Message = "无需处理（PDF 已经很紧凑）"
		return res
	}
	stagingName := fmt.Sprintf("%04d", index)
	finalPath, err := writeConvertedFile(stagingDir, stagingName, ".pdf", out)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("保存到暂存区失败: %v", err)
		return res
	}
	res.Status = StatusSuccess
	res.OutputPath = finalPath
	res.OutputBytes = int64(len(out))
	res.Message = pdfSuccessMessage(stats)
	return res
}

// pdfSuccessMessage 按图片统计组装诚实的结果文案，跳过必须带原因
// （spec：「报告跳过张数及原因」；词汇以 GLOSSARY「压缩 PDF」各词条为准）。
func pdfSuccessMessage(stats imageproc.PDFStats) string {
	var parts []string
	if stats.ImagesReplaced > 0 {
		s := fmt.Sprintf("重新保存了 %d 张图片", stats.ImagesReplaced)
		if stats.ImagesDownsampled > 0 {
			s += fmt.Sprintf("（其中 %d 张压到了屏幕清晰度）", stats.ImagesDownsampled)
		}
		parts = append(parts, s)
	}
	var reasons []string
	if stats.ImagesUnsupported > 0 {
		reasons = append(reasons, fmt.Sprintf("%d 张是不支持的图片类型", stats.ImagesUnsupported))
	}
	if stats.ImagesNotSmaller > 0 {
		reasons = append(reasons, fmt.Sprintf("%d 张已经很省空间", stats.ImagesNotSmaller))
	}
	if len(reasons) > 0 {
		parts = append(parts, "另 "+strings.Join(reasons, "、")+"，保持原样")
	}
	if len(parts) == 0 {
		return "已压缩 PDF（清理了内部冗余）"
	}
	return "已压缩 PDF（" + strings.Join(parts, "，") + "）"
}

// linkOutputFile 抽出硬链接调用，测试用它模拟不支持链接的文件系统（网络盘等）。
var linkOutputFile = os.Link

// writeConvertedFile 先写目标目录下的临时文件，再以不覆盖方式落位。
// 临时文件与最终位置【同目录】：硬链接落位天然原子；即便走复制回退也不跨目录搬运。
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

// placeOutputFile 以不覆盖方式落位：目标名已存在时尝试下一个序号（-2、-3 …，US-16）。
//
// 首选硬链接（原子占位）；链接不可用（跨卷或网络盘不支持）时退化为
// 「独占创建 + 复制」——原子性变弱，但「绝不覆盖已有文件」的承诺不破（O_EXCL）。
func placeOutputFile(tempPath, dir, stem, ext string) (string, error) {
	for i := 1; ; i++ {
		number := ""
		if i > 1 {
			number = fmt.Sprintf("-%d", i)
		}
		candidate := filepath.Join(dir, stem+number+ext)
		err := linkOutputFile(tempPath, candidate)
		if err == nil {
			return candidate, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err := copyExclusive(tempPath, candidate); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", err
		}
		return candidate, nil
	}
}

// copyExclusive 以 O_EXCL 独占创建 dst 并复制 src 内容；中途失败清理半成品。
func copyExclusive(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// shouldSkipConvert 判定「源文件已满足规则、无需转换」（US-24；auto 的边界见 D-42）。
// 批次与单张试算（PreviewConvert）共用同一份判定——同一规则不许出现两个实现。
func shouldSkipConvert(rules BatchRules, sourceBytes int64, format string, width, height int) bool {
	boxEnabled := rules.MaxWidth > 0
	targetOK := rules.TargetBytes == 0 || sourceBytes <= rules.TargetBytes
	boxOK := !boxEnabled || (width <= rules.MaxWidth && height <= rules.MaxHeight)
	if rules.OutputFormat == imageproc.OutputAuto {
		// auto：至少启用目标体积或尺寸盒其一且均满足才跳过；两者皆未启用时不跳过——
		// 用户单独选 auto 正是为了重编码到最小体积格式，源文件谈不上「已达标」。
		return (rules.TargetBytes > 0 || boxEnabled) && targetOK && boxOK
	}
	outFormat, err := imageproc.NormalizeOutputFormat(rules.OutputFormat, format)
	if err != nil {
		return false
	}
	return outFormat == format && targetOK && boxOK
}

// autoFormatCandidates 是「自动（最小体积）」的候选集与平局偏好序（D-43）：
// 平局（字节数完全相同）取先到者，JPEG 在前——兼容性最稳的格式优先。
var autoFormatCandidates = []string{imageproc.OutputJPEG, imageproc.OutputWebP, imageproc.OutputPNG}

// convertBestFormat 对同一张图逐个候选格式跑一次完整转换（含目标体积搜索），
// 取输出体积最小者（US-49）。有目标体积时 Convert 失败即该候选不达标，
// 全部不达标则整项失败——与单格式「不可达标即失败」的语义一致。
// 透明图转 JPEG 的铺白候选照常参与竞速（铺白标记随所选结果回传，用户自行取舍）。
func convertBestFormat(data []byte, path string, rules BatchRules) (imageproc.Converted, error) {
	var best imageproc.Converted
	var firstErr error
	for _, format := range autoFormatCandidates {
		converted, err := imageproc.Convert(data, path, imageproc.ConvertOptions{
			Format:      format,
			MaxWidth:    rules.MaxWidth,
			MaxHeight:   rules.MaxHeight,
			TargetBytes: rules.TargetBytes,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if best.Data == nil || len(converted.Data) < len(best.Data) {
			best = converted
		}
	}
	if best.Data == nil {
		if firstErr == nil {
			firstErr = apperr.Validation("没有可用的输出格式")
		}
		return imageproc.Converted{}, firstErr
	}
	return best, nil
}

func formatDisplayName(format string) string {
	switch format {
	case imageproc.OutputJPEG:
		return "JPEG"
	case imageproc.OutputPNG:
		return "PNG"
	case imageproc.OutputWebP:
		return "WebP"
	default:
		return format
	}
}

// RenderOutputSample 把命名模板与输出格式渲染成示例文件名，供规则区实时预览（US-41）。
// 模板替换与扩展名映射与真实落盘共用同一套实现（renderOutputStem/outputExtension）——
// 预览若是第二份实现，边界（未知占位符、keep 扩展名、auto）迟早分叉。
// auto 格式的扩展名逐项而定，示例以 <自动> 占位。sampleName 非法时返回错误。
func (s *Service) RenderOutputSample(pattern, sampleName, outputFormat string) (string, error) {
	if err := validatePattern(pattern); err != nil {
		return "", err
	}
	stem := strings.TrimSuffix(sampleName, filepath.Ext(sampleName))
	rendered := renderOutputStem(pattern, stem, 1)
	switch outputFormat {
	case imageproc.OutputKeep:
		// 保持原格式：示例沿用样例文件自己的扩展名（无扩展名的极端情形按 PNG 展示，
		// 与 keep 对 BMP/TIFF/GIF 的兜底一致）。
		ext := strings.ToLower(filepath.Ext(sampleName))
		if ext == "" {
			ext = ".png"
		}
		return rendered + ext, nil
	case imageproc.OutputJPEG, imageproc.OutputPNG, imageproc.OutputWebP:
		ext, err := outputExtension(outputFormat)
		if err != nil {
			return "", err
		}
		return rendered + ext, nil
	case imageproc.OutputAuto:
		return rendered + ".<自动>", nil
	default:
		return "", apperr.Validation("输出格式非法")
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
