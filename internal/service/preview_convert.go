package service

// 单张调参试算（US-45~48）：复用批次的读取/探测/达标判定/转换管线，
// 把「写文件」换成「渲染输出预览」，永不落盘。

import (
	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

// PreviewConvertResult 是单张试算的结果。体积数字一律以 OutputBytes 为准
// （真实编码字节）；PreviewURL 是输出像素的预览级重编码，给人眼看、不当测量仪。
type PreviewConvertResult struct {
	Status          string `json:"status"` // success | skipped | failed
	Message         string `json:"message,omitempty"`
	OutputBytes     int64  `json:"output_bytes"`
	Format          string `json:"format"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Quality         int    `json:"quality"` // 达标所用最高编码质量；PNG 为 0
	ExtremeDegraded bool   `json:"extreme_degraded"`
	AlphaFlattened  bool   `json:"alpha_flattened"`
	PreviewURL      string `json:"preview_url"`
}

// previewMaxEdge 是工作台预览的渲染上限：取 imageproc 允许的最大值（2048）。
// 放大镜在这个分辨率上做预览级缩放（D-40：不做像素级，规格 §6 边界维持）。
const previewMaxEdge = 2048

// PreviewConvert 用给定规则对单张图做一次真实试算（US-46），不写出任何文件：
// 读取上限/内容探测/已达标判定（shouldSkipConvert）与批次完全同一套；
// 有目标体积时内部同样走质量二分搜索，单次典型耗时零点几秒（D-39：规则不含
// 质量滑杆，质量作为结果回传）。批次运行中不受理（D-41：避免与 worker 抢 CPU）。
func (s *Service) PreviewConvert(path string, rules BatchRules) (PreviewConvertResult, error) {
	s.batchMu.Lock()
	running := s.batchRunning
	s.batchMu.Unlock()
	if running {
		return PreviewConvertResult{}, apperr.Conflict("批次处理进行中，暂不能试算")
	}
	if err := validateBatchRules(rules); err != nil {
		return PreviewConvertResult{}, err
	}

	abs, data, err := readImageFile(path)
	if err != nil {
		return PreviewConvertResult{}, err
	}
	probe, err := imageproc.ProbeImage(data, abs)
	if err != nil {
		return PreviewConvertResult{}, err
	}

	if shouldSkipConvert(rules, int64(len(data)), probe.Format, probe.Width, probe.Height) {
		// 已达标也是有效结论：渲染源图预览，让工作台仍有东西可看。
		preview, previewErr := imageproc.RenderPreview(data, abs, previewMaxEdge)
		previewURL := ""
		if previewErr == nil {
			previewURL = preview.DataURL
		}
		return PreviewConvertResult{
			Status:     string(StatusSkipped),
			Message:    "已达标，不会产出文件",
			Width:      probe.Width,
			Height:     probe.Height,
			PreviewURL: previewURL,
		}, nil
	}

	var converted imageproc.Converted
	if rules.OutputFormat == imageproc.OutputAuto {
		converted, err = convertBestFormat(data, abs, rules)
	} else {
		converted, err = imageproc.Convert(data, abs, imageproc.ConvertOptions{
			Format:      rules.OutputFormat,
			MaxWidth:    rules.MaxWidth,
			MaxHeight:   rules.MaxHeight,
			TargetBytes: rules.TargetBytes,
		})
	}
	if err != nil {
		// 试算失败是「结论」不是事故（如目标体积过小不可达），随结果带回供界面内展示。
		return PreviewConvertResult{Status: string(StatusFailed), Message: err.Error()}, nil
	}

	preview, previewErr := imageproc.RenderPreview(converted.Data, abs, previewMaxEdge)
	if previewErr != nil {
		return PreviewConvertResult{Status: string(StatusFailed), Message: "渲染输出预览失败"}, nil
	}

	message := ""
	if rules.OutputFormat == imageproc.OutputAuto {
		message = "自动选择 " + formatDisplayName(converted.Format) + "（最优）"
	}
	return PreviewConvertResult{
		Status:          string(StatusSuccess),
		Message:         message,
		OutputBytes:     int64(len(converted.Data)),
		Format:          converted.Format,
		Width:           converted.Width,
		Height:          converted.Height,
		Quality:         converted.Quality,
		ExtremeDegraded: converted.ExtremeDegraded,
		AlphaFlattened:  converted.AlphaFlattened,
		PreviewURL:      preview.DataURL,
	}, nil
}
