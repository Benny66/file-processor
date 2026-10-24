// pdf.go — PDF 压缩（compress-pdf 变更，ADR-0009）。
//
// 纯函数层：字节进字节出，不经临时文件。两级压缩：
//  1. 图像重压缩——遍历 xref 表定位图像 XObject，对「单一 DCTDecode + DeviceRGB/
//     DeviceGray + 8bpc」的图用标准库重编码 JPEG（质量下限 75、不缩像素、仅更小才
//     替换）；其余（Flate 图像、JBIG2、CCITT、JPX、CMYK、Indexed、带 Decode 映射）
//     保持原字节并计数。
//  2. 无损结构优化——pdfcpu optimize：去重内嵌资源、对象流/xref 流压缩、清理无用
//     对象；内容字节级不变。
//
// 「内容不更不清楚」的下限形态：不缩像素 + 质量下限 75 + 重编码不更小就不动 + 已是
// 75 档以下的图天然跳过。加密与带签名的 PDF 直接拒绝（不解密、不破坏签名效力）。
package imageproc

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"file-processor/internal/apperr"
)

// pdfJPEGQuality 是 PDF 内图片重保存的质量档（「够用的质量」单一真相）。
const pdfJPEGQuality = 75

// filterDCTDecode 是 PDF 图像的 JPEG 流滤镜名。
const filterDCTDecode = "DCTDecode"

// PDFStats 描述一次 PDF 压缩里图片层的处理情况，供逐项消息转述。
// 跳过按原因分两档（spec 要求「跳过张数及原因」）：不支持的类型 / 已经很省空间。
type PDFStats struct {
	// ImagesTotal 是文档里的图像对象总数（多个页面共用一个图像对象只算一次）。
	ImagesTotal int `json:"images_total"`
	// ImagesReplaced 是被重编码并替换的图像数。
	ImagesReplaced int `json:"images_replaced"`
	// ImagesUnsupported 是因滤镜/色彩空间/位深不在处理面而保持原样的图像数。
	ImagesUnsupported int `json:"images_unsupported"`
	// ImagesNotSmaller 是重编码后不小于原图而保持原样的图像数（已经够省空间）。
	ImagesNotSmaller int `json:"images_not_smaller"`
}

// pdfImageResult 是单张图像的处理结论。
type pdfImageResult int

const (
	pdfImageReplaced pdfImageResult = iota
	pdfImageNotSmaller
	pdfImageUnsupported
)

// IsPDF 按内容判断是否 PDF：%PDF- 头。规范允许文件头前有少量杂字节，取前 1KB 窗口搜索。
func IsPDF(data []byte) bool {
	window := data
	if len(window) > 1024 {
		window = window[:1024]
	}
	return bytes.Contains(window, []byte("%PDF-"))
}

// IsPDFPath 按扩展名判断（导入预筛与批次分流用；内容不符会在处理时按损坏拒绝）。
func IsPDFPath(path string) bool {
	return FormatFromPath(path) == "pdf"
}

// InspectPDF 返回 PDF 的队列检查结果：无像素尺寸、无缩略图（队列行显示 PDF 标识）。
func InspectPDF(data []byte, path string) Info {
	return Info{Path: path, Format: "pdf", Bytes: int64(len(data))}
}

// CompressPDF 压缩 PDF 字节并返回（新字节, 图片统计, 错误）。
// 加密/带签名/损坏分别返回对应业务错误码（apperr.CodePDF*），前端据码分流。
func CompressPDF(ctx context.Context, src []byte) ([]byte, PDFStats, error) {
	if int64(len(src)) > MaxFileBytes {
		return nil, PDFStats{}, apperr.Validation("PDF 超过 512 MB 上限")
	}
	doc, err := api.ReadContext(ctx, bytes.NewReader(src), nil)
	if err != nil {
		return nil, PDFStats{}, apperr.New(apperr.CodePDFDamaged, "这个 PDF 打不开，可能已损坏")
	}
	if doc.Encrypt != nil {
		return nil, PDFStats{}, apperr.New(apperr.CodePDFEncrypted, "这个 PDF 加了密，暂不支持")
	}
	if hasPDFSignatures(doc) {
		return nil, PDFStats{}, apperr.New(apperr.CodePDFSigned, "这个 PDF 带签名，不改动它")
	}

	stats := recompressPDFImages(doc)
	if err := api.OptimizeContext(ctx, doc); err != nil {
		return nil, PDFStats{}, apperr.Wrap(err)
	}
	var buf bytes.Buffer
	if err := api.WriteContext(ctx, doc, &buf); err != nil {
		return nil, PDFStats{}, apperr.Wrap(err)
	}
	return buf.Bytes(), stats, nil
}

// hasPDFSignatures 检测文档是否含数字签名：SigFlags 的 bit0（SignaturesExist）。
// 只含未签署的签名表单字段不算——那是可正常处理的普通表单。
func hasPDFSignatures(doc *model.Context) bool {
	catalog, err := doc.XRefTable.Catalog()
	if err != nil {
		return false // 目录读不出时交给后续优化/写回暴露真实错误，这里不臆断
	}
	obj, found := catalog.Find("AcroForm")
	if !found {
		return false
	}
	acroForm, ok := obj.(types.Dict)
	if !ok {
		return false // 间接引用等复杂形态：优化阶段读得到就继续，读不到会自然报错
	}
	flagsObj, found := acroForm.Find("SigFlags")
	if !found {
		return false
	}
	flags, ok := flagsObj.(types.Integer)
	return ok && flags.Value()&1 != 0
}

// recompressPDFImages 就地重保存可处理的内嵌 JPEG，返回统计。
//
// xref 表里的 StreamDict 是【值类型】：改完必须整体写回条目（spike 踩平的坑）。
func recompressPDFImages(doc *model.Context) PDFStats {
	var stats PDFStats
	for _, entry := range doc.XRefTable.Table {
		sd, ok := entry.Object.(types.StreamDict)
		if !ok || !sd.Image() {
			continue
		}
		stats.ImagesTotal++
		recompressed, result := recompressPDFImage(sd)
		switch result {
		case pdfImageReplaced:
			entry.Object = recompressed
			stats.ImagesReplaced++
		case pdfImageNotSmaller:
			stats.ImagesNotSmaller++
		case pdfImageUnsupported:
			stats.ImagesUnsupported++
		}
	}
	return stats
}

// recompressPDFImage 尝试重保存单张图像，返回写回用的新对象与处理结论。
// 「不满足处理面」（滤镜/色彩空间/位深/尺寸）与「重存不更小」分开回报，
// 前者计入不支持的类型，后者说明它已经够省空间——两种保持原样的原因不同。
func recompressPDFImage(sd types.StreamDict) (types.StreamDict, pdfImageResult) {
	if !sd.HasSoleFilterNamed(filterDCTDecode) || !recompressibleColorSpace(sd) {
		return types.StreamDict{}, pdfImageUnsupported
	}
	width, height := pdfImageDims(sd)
	if width <= 0 || height <= 0 || int64(width)*int64(height) > MaxPixels {
		return types.StreamDict{}, pdfImageUnsupported
	}
	img, _, err := image.Decode(bytes.NewReader(sd.Raw))
	if err != nil {
		return types.StreamDict{}, pdfImageUnsupported
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: pdfJPEGQuality}); err != nil {
		return types.StreamDict{}, pdfImageUnsupported
	}
	if out.Len() >= len(sd.Raw) {
		return types.StreamDict{}, pdfImageNotSmaller
	}
	// 回写：替换编码字节与长度，色彩空间按实际编码通道数重申（灰度图重编码仍是灰度），
	// 删掉 JPEG 不存在的解码参数。SMask（透明蒙版）是独立对象，不在本函数管辖内，原样保留。
	n := int64(out.Len())
	sd.Raw = out.Bytes()
	sd.Content = nil
	sd.StreamLength = &n
	sd.FilterPipeline = []types.PDFFilter{{Name: filterDCTDecode}}
	colorSpace := types.Name("DeviceRGB")
	if _, isGray := img.(*image.Gray); isGray {
		colorSpace = types.Name("DeviceGray")
	}
	sd.Dict["ColorSpace"] = colorSpace
	sd.Dict["BitsPerComponent"] = types.Integer(8)
	delete(sd.Dict, "DecodeParms")
	return sd, pdfImageReplaced
}

// recompressibleColorSpace 只吃 DeviceRGB / DeviceGray、8bpc 且无 Decode 映射的图：
// CMYK 转 RGB 会偏色、Indexed 查色表语义复杂、Decode 是分量重映射——都不碰。
// 位深显式对齐 spec 的「8bpc」谓词（非 8bit DCT 罕见，标准库也解不了，双重保险）。
func recompressibleColorSpace(sd types.StreamDict) bool {
	cs, found := sd.Dict["ColorSpace"]
	if !found {
		return false
	}
	name, ok := cs.(types.Name)
	if !ok || (name != "DeviceRGB" && name != "DeviceGray") {
		return false
	}
	bpc, ok := sd.Dict["BitsPerComponent"].(types.Integer)
	if !ok || bpc.Value() != 8 {
		return false
	}
	_, hasDecode := sd.Dict["Decode"]
	return !hasDecode
}

// pdfImageDims 读图像对象的像素尺寸；缺失或非法返回 (0, 0)。
func pdfImageDims(sd types.StreamDict) (int, int) {
	w, wOK := sd.Dict["Width"].(types.Integer)
	h, hOK := sd.Dict["Height"].(types.Integer)
	if !wOK || !hOK {
		return 0, 0
	}
	return w.Value(), h.Value()
}
