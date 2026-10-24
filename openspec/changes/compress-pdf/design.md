# Design: compress-pdf

## 依赖与分层

- 新直接依赖 `github.com/pdfcpu/pdfcpu`（Apache-2.0，纯 Go）：登记 `deps.yaml` 并附理由（ADR-0009）。
  其传递依赖均为纯 Go（hhrutter/tiff、x/image/ccitt、x/text、x/crypto、yaml、go-runewidth 等），与 ADR-0004 相容。
- 新文件 `internal/imageproc/pdf.go`（+ `pdf_test.go`）：PDF 压缩纯函数层，接口形状与现有图片处理一致：

```go
// CompressPDF 压缩 PDF：先逐图重压缩（q=75、不缩像素、仅更小才替换），
// 再做 pdfcpu 无损结构优化。返回替换/跳过计数与输出字节。
func CompressPDF(ctx context.Context, src []byte) (out []byte, stats PDFStats, err error)
```

  纯函数（字节进字节出），不经临时文件（pdfcpu 支持 `ReadContext`/`WriteContext` 内存态），测试可用表驱动夹具。

## 图像重压缩的取舍（spike 结论固化）

- 只处理「单一 DCTDecode + DeviceRGB/DeviceGray + 8bpc」：标准库 `image/jpeg` 可解码，重编码语义清晰。
- 扫描页降采样（ADR-0010，默认开启）：`scanImageTargets` 先按页树定位「满页单图」（资源唯一图像 XObject + 长宽比 ±15% 匹配页面，/Rotate 已换轴），从页面尺寸精确算有效 DPI，高于 150 才给降采样目标；缩放直接复用图片管线的 `resizeImage`（CatmullRom）。窄带：DPI 略超 150 时目标像素恰等于原尺寸，回落普通重存而非跳过。跨页共享同一图像而页尺寸不同（目标矛盾）时放弃该图不缩。实现坑：pdfcpu `ReadContext` 后 `PageCount` 为 0，须先 `EnsurePageCount` 再 `PageDict`，否则识别静默失效。
- **质量下限 75 + 仅更小才替换**：已是 75 档以下的图天然跳过（已紧凑的图不降质）；像素尺寸只对满页扫描图例外（见上条）。
- **SMask（透明蒙版）原样保留**，不解不删（spike 为测试删过，真实实现禁止——JPEG 无 alpha，蒙版独立成对象）。
- 跳过即诚实计数并分原因：Flate 图像、JBIG2、CCITT、JPX、Indexed 色彩空间等不在 v1 处理面，计「不支持的图片类型」；重存不更小计「已经很省空间」——逐项消息如实转述（词汇见 GLOSSARY）。
- 加密 / 签名 PDF：读入阶段检测（`api.ReadContext` 报错 / V 字段存在），返回业务错误「加密的 PDF 暂不支持」/「带签名的 PDF 不改动」，不静默降级。

## 与现有管线的接缝

- `formats.go`：`supportedFormats` 加 `{".pdf", "pdf"}`；`inspect.go` 内容嗅探补 `%PDF-` 分支（图片与 PDF 嗅探互斥）。
- `service/batch_service.go`：逐项处理前按 sniff 结果分流——图片走现行规则链，PDF 调 `CompressPDF`（忽略三条规则参数）。
  状态机不动（success/failed/skipped/canceled），新增消息文案走 GLOSSARY。
- 「无需处理」语义：PDF 输出 ≥ 输入（含 <1% 抖动）时标 skipped、消息「无需处理（PDF 已经很紧凑）」，不产出文件——与图片「已达标」行为对齐。
- 暂存区/下载/历史：零改动（`<原名>-out.pdf` 落 `<一次处理>/`，下载流复用）。
- 契约镜像：新错误码 `pdf_encrypted` / `pdf_signed` / `pdf_damaged` 同步 `frontend/src/lib/invoke.ts`，`parity_test.go` 双向校验；图片计数经逐项 message 转述，不新增无人消费的 payload 字段。
- 前端：规则区在「本批全是 PDF」时禁用并附说明；混批时 PDF 行显示「PDF 压缩」徽标、规则仅作用于图片行（`views/image` 队列行组件加类型分支）。

## GLOSSARY 词条（已随实现录入正文）

压缩 PDF / 重新保存了 x 张图片 / 不支持的图片类型 / 已经很省空间 / 清理了内部冗余 / 这个 PDF 打不开 / PDF 已经很紧凑 / 这个 PDF 加了密 / 带签名的 PDF——共 9 条，见 GLOSSARY.md「PDF」小节。
