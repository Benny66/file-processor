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

// 输出目的地模式（单一真相；前端镜像在 frontend/src/lib/image.ts 的 OutputMode）。
const (
	// OutputModeSource 写回源文件同目录（默认，行为与历史版本一致）。
	OutputModeSource = "source"
	// OutputModeCustomFlat 写入用户指定目录，全部平铺。
	OutputModeCustomFlat = "custom_flat"
	// OutputModeCustomTree 写入用户指定目录，并按导入时的相对路径保留文件夹结构。
	OutputModeCustomTree = "custom_tree"
)

// OutputPatternDefault 是命名模板的缺省值（即历史行为的 <原名>-out）。
const OutputPatternDefault = "{name}-out"

// BatchRules 是一批图片的处理规则；0 表示关闭该项。
type BatchRules struct {
	OutputFormat string `json:"output_format"`
	TargetBytes  int64  `json:"target_bytes"`
	MaxWidth     int    `json:"max_width"`
	MaxHeight    int    `json:"max_height"`
}

// OutputPlan 描述结果写到哪里、叫什么名字（US-39~41）。
// 它是「环境粘性设置」而非规则：随上次设置持久化，但【不进预设】——
// 输出目录是机器相关的事实，进预设会让「应用预设」意外改写落盘位置（D-34）。
type OutputPlan struct {
	Mode      string `json:"mode"`      // OutputMode* 之一；空串按 source 处理
	Directory string `json:"directory"` // custom_* 模式必填，绝对路径
	Pattern   string `json:"pattern"`   // 词干命名模板；空串按 "{name}-out" 处理
}

// BatchItem 是批次里的一张图：绝对路径 + 导入根下的相对路径。
// RelPath 只为 custom_tree 模式服务（US-40）；source / custom_flat 模式可为空。
type BatchItem struct {
	Path    string `json:"path"`
	RelPath string `json:"rel_path"`
}

// ImageSettings 是「上次设置」的整体：处理规则 + 输出目的地（US-42）。
type ImageSettings struct {
	Rules  BatchRules `json:"rules"`
	Output OutputPlan `json:"output"`
}

// validateBatchStart 是「能不能开始一个批次」的完整判定（US-11）：
// 规则范围合法 + 输出目的地合法 + 至少一条有效规则。
//
// 「至少一条」只约束开跑，不约束保存预设/上次设置——用户可能先把空规则存下来。
func validateBatchStart(rules BatchRules, output OutputPlan) error {
	if err := validateBatchRules(rules); err != nil {
		return err
	}
	if err := validateOutputPlan(output); err != nil {
		return err
	}
	if rules.OutputFormat == imageproc.OutputKeep && rules.TargetBytes == 0 && rules.MaxWidth == 0 {
		return apperr.Validation("请至少选一项：转格式、压到指定大小或限制尺寸")
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
func (s *Service) ValidateImageRules(rules BatchRules, output OutputPlan) string {
	if err := validateBatchStart(rules, output); err != nil {
		return err.Error()
	}
	return ""
}

// validateOutputPlan 校验输出目的地；批次、预检与上次设置共用同一套规则。
// 自定义目录允许暂不存在（落位前统一 MkdirAll），但必须是绝对路径。
func validateOutputPlan(plan OutputPlan) error {
	switch plan.Mode {
	case "", OutputModeSource, OutputModeCustomFlat, OutputModeCustomTree:
	default:
		return apperr.Validation("输出模式取值非法")
	}
	if plan.Mode == OutputModeCustomFlat || plan.Mode == OutputModeCustomTree {
		if strings.TrimSpace(plan.Directory) == "" {
			return apperr.Validation("自定义输出目录不能为空")
		}
		if !filepath.IsAbs(filepath.Clean(plan.Directory)) {
			return apperr.Validation("输出目录必须是绝对路径")
		}
	}
	pattern := plan.Pattern
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

// ensureOutputDir 为 custom 模式创建输出根目录（幂等；source 模式无需创建）。
// 只负责根目录：custom_tree 的子目录随逐项落位创建（导入结构不可提前枚举）。
// convertOnce 内的 MkdirAll 保留为防御性兜底（批次运行中目录被删等极端情形）。
func ensureOutputDir(plan OutputPlan) error {
	switch plan.Mode {
	case OutputModeCustomFlat, OutputModeCustomTree:
		if err := os.MkdirAll(filepath.Clean(plan.Directory), 0o755); err != nil {
			return apperr.Validation(fmt.Sprintf("无法创建输出目录: %v", err))
		}
	}
	return nil
}

// resolveOutputDir 计算某一项的输出目录；custom_tree 模式按相对路径下钻子目录。
func resolveOutputDir(item BatchItem, plan OutputPlan) (string, error) {
	switch plan.Mode {
	case "", OutputModeSource:
		return filepath.Dir(item.Path), nil
	case OutputModeCustomFlat:
		return filepath.Clean(plan.Directory), nil
	case OutputModeCustomTree:
		rel := filepath.Clean(item.RelPath)
		if rel == "" || rel == "." {
			rel = filepath.Base(item.Path)
		}
		// RelPath 理论上只来自扫描（不含 ..），但绑定入参不可信：
		// 这里挡住路径穿越，防止把输出写到指定目录之外。
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", apperr.Validation("输出相对路径非法")
		}
		return filepath.Join(filepath.Clean(plan.Directory), filepath.Dir(rel)), nil
	default:
		return "", apperr.Validation("输出模式取值非法")
	}
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

// BatchRequest 是一次批量处理请求。Items 取代了原先的 Paths：保留结构模式
// 需要每项「导入根下的相对路径」（US-40），只有绝对路径推不出来。
type BatchRequest struct {
	Items  []BatchItem `json:"items"`
	Rules  BatchRules  `json:"rules"`
	Output OutputPlan  `json:"output"`
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
	if len(req.Items) == 0 {
		return BatchResult{}, apperr.Validation("请先添加要处理的图片")
	}
	if err := validateBatchStart(req.Rules, req.Output); err != nil {
		return BatchResult{}, err
	}
	// US-39：自定义输出目录的创建放在批次开始前——目录不可写在这里报 Validation，
	// 而不是让整批逐项做完编码才失败（那会白跑一整批转换）。
	if err := ensureOutputDir(req.Output); err != nil {
		return BatchResult{}, err
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

				res := s.convertOne(item, idx+1, req.Rules, req.Output)
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
		summary.Message = "已取消"
	} else if summary.Failed > 0 {
		summary.Message = "处理完成，有文件没成功"
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
func (s *Service) convertOne(item BatchItem, index int, rules BatchRules, output OutputPlan) ItemResult {
	res := s.convertOnce(item, index, rules, output)
	if res.Status == StatusFailed {
		slog.Warn("图片处理失败", "path", item.Path, "reason", res.Message)
	}
	return res
}

func (s *Service) convertOnce(item BatchItem, index int, rules BatchRules, output OutputPlan) ItemResult {
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
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dir, err := resolveOutputDir(item, output)
	if err != nil {
		res.Status = StatusFailed
		res.Message = err.Error()
		return res
	}
	// 目录不存在则创建（custom 模式常态；source 模式目录必已存在，MkdirAll 是无害的幂等调用）。
	if err := os.MkdirAll(dir, 0o755); err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("创建输出目录失败: %v", err)
		return res
	}
	finalPath, err := writeConvertedFile(dir, renderOutputStem(output.Pattern, stem, index), ext, converted.Data)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("保存文件失败: %v", err)
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
	if err := validateOutputPlan(OutputPlan{Mode: OutputModeSource, Pattern: pattern}); err != nil {
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
