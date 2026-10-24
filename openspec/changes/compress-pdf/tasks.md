# Tasks: compress-pdf

- [x] 1. 依赖：`go get github.com/pdfcpu/pdfcpu@v0.16.3`，登记 `deps.yaml`（理由引 ADR-0009）；`make lint` 护栏双向校验过。
- [x] 2. `internal/imageproc/pdf.go`：`CompressPDF` 纯函数（内存态，无临时文件）——图像重压缩（DCTDecode/RGB|Gray/8bpc，q75 不缩像素，更小才替换，SMask 保留）+ pdfcpu 无损优化；返回统计，跳过分原因（不支持的图片类型 / 已经很省空间）。夹具测试：DCT 大图（显著变小）、Flate 图像（计入不支持的图片类型）、加密（业务错）、损坏（业务错）。签名拒绝以 SigFlags bit0 判定，造签名夹具需证书工具链，暂以代码审查保障；「已紧凑跳过」由 service 层桩测覆盖（imageproc 层二次压缩的体积关系不确定，不宜作断言）。
- [x] 3. `formats.go`：`.pdf` 注册 + 展示名，`formats_test.go` 补例；`%PDF-` 内容嗅探落在 `pdf.go` 的 `IsPDF`（与图片嗅探的互斥由批次分流顺序保证），`inspect.go`/`inspect_test.go` 无需改动。
- [x] 4. `service/batch_service.go`：按 sniff 分流（PDF 忽略三条规则）；输出 ≥ 输入时 skipped「无需处理（PDF 已经很紧凑）」；消息文案先落 GLOSSARY 再引用。
- [x] 5. 契约：新错误码 `pdf_encrypted` / `pdf_signed` / `pdf_damaged` 进 `internal/apperr` 并同步前端镜像（`parity_test.go` 双向校验）；图片计数经逐项 message 转述（GLOSSARY 词汇），不新增无人消费的 payload 字段——出现结构化消费方时再进契约。
- [x] 6. 前端 `views/image`：导入预筛接受 `.pdf`；规则区纯 PDF 批禁用+说明；队列行 PDF 徽标与消息展示；历史/下载行零改动验证（`-out.pdf`）。
- [x] 7. GLOSSARY 录入词条（压缩 PDF / PDF 已经很紧凑 / 加密与签名说明）；README「能做什么」补 PDF 行；`docs/map.md` 登记 `pdf.go`。
- [ ] 8. `make test` / `make lint` 全绿；真机冒烟：拖入扫描件 PDF + 图片混批，验证队列、暂存、下载、历史。
- [x] 9. 扫描页降采样（ADR-0010）：`scanImageTargets` 满页单图识别 + 150 DPI 下限 + `ImagesDownsampled` 统计与消息转述；夹具测试：300 DPI 扫描页压到 600px、72 DPI 导入页不触发；GLOSSARY 补「压到了屏幕清晰度」。
