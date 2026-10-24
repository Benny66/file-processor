# PDF 压缩：选 pdfcpu 做纯 Go 对象级图像重压缩

「图片工坊」要支持 PDF 压缩（内容不能变不清楚）。PDF 体积大头是内嵌图片流，无损结构优化收益有限；图像重压缩在纯 Go 生态没有现成能力——唯一活跃维护的纯 Go PDF 库 **pdfcpu**（v0.16.3，Apache-2.0）的 `optimize` 只做无损结构优化（去重/对象流/清理），图像重压缩是其 2017 年至今的 open feature request。决定：**引入 pdfcpu 作解析/回写底座，图像重压缩自己做对象级手术**——遍历 xref 表定位图像 XObject，对「单一 DCTDecode + DeviceRGB/DeviceGray + 8bpc」用标准库 `image/jpeg` 重编码（质量下限 75、**不缩小像素尺寸**、仅更小才替换、SMask 原样保留），再叠加 pdfcpu 无损优化。已 spike 验证（2026-10-24）：2400² DCT 图像 q75 重编码，整文件 -42.9%，validate 通过，渲染像素对比平均差 0.41/765（肉眼不可辨）。实现要点：xref 条目里 `StreamDict` 是**值类型**，改后须整体写回。

**备选方案**：只做无损结构优化（安全但扫描件收益小，用户已明确要图像重压缩）；Ghostscript（AGPL 许可 + 外部二进制，破坏三平台自包含与麒麟容器约束，ADR-0004 哲学不容）；UniPDF（商业许可）；qpdf（同样无图像处理能力）；基于 pdfcpu 提取图像→外部重组（无「替换回写」API，需重排整个文档树，风险更高）。

**后果**：`deps.yaml` 新增 pdfcpu 直接依赖（传递依赖全纯 Go：hhrutter/tiff、x/image/ccitt、x/text、x/crypto、yaml、go-runewidth）；v1 明确跳过 Flate 图像/JBIG2/CCITT/JPX/Indexed（诚实计数汇报）；加密与签名 PDF 直接拒绝；DPI 下限与空间降采样留待后续按页面显示尺寸评估。变更提案见 `openspec/changes/compress-pdf/`。
