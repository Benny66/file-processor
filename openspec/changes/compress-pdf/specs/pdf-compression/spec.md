# compress-pdf 能力增量（新能力：pdf-compression）

## ADDED Requirements

### Requirement: PDF 可导入并压缩

导入链路 MUST 接受 `.pdf` 扩展名，且以 `%PDF-` 内容嗅探确认（扩展名只作预筛）；
处理输出 MUST 仍是 PDF（`<原名>-out.pdf`），走同一暂存区与下载流。

#### Scenario: 拖入 PDF 与图片混批
- **WHEN** 一个批次同时含图片与 PDF
- **THEN** 图片项按三条规则处理，PDF 项走压缩管线，共用同一队列与状态机

#### Scenario: 纯 PDF 批次
- **WHEN** 批次只有 PDF
- **THEN** 规则区三条控件禁用并附说明，处理仍可开始

### Requirement: 压缩不降低内容清晰度

图像重压缩 MUST 质量下限 75，且仅当重编码结果更小时替换；
SMask MUST 原样保留；无损结构优化 MUST 不改动内容流。
满页扫描图（页面唯一图像 + 填满页面 + 有效 DPI 高于 150）MUST 默认等比压到
150 DPI（屏幕清晰度）再重存（ADR-0010）；其余图片 MUST 保持原像素尺寸。

#### Scenario: 高 DPI 扫描件默认压到屏幕清晰度
- **WHEN** 某页只有一张填满页面的图且有效 DPI 高于 150（如 300 DPI 扫描）
- **THEN** 该图等比压到 150 DPI 后重存，消息转述「其中 x 张压到了屏幕清晰度」

#### Scenario: 非扫描页不触发降采样
- **WHEN** 页面有多张图、图未填满页面（长宽比不符）或有效 DPI 不高于 150
- **THEN** 不缩像素，仅按质量下限重存

#### Scenario: 已紧凑的图像不降质
- **WHEN** 某图像重编码后不小于原图
- **THEN** 该图跳过不替换，保持原字节

### Requirement: 诚实汇报与拒绝面

处理结果 MUST 报告跳过张数及原因；输出不小于输入时 MUST 标「无需处理」不产出文件；
加密或签名 PDF MUST 以明确业务错误拒绝，MUST NOT 解密或改动签名文件。

#### Scenario: 加密 PDF
- **WHEN** 导入受密码保护的 PDF
- **THEN** 该项失败并提示「这个 PDF 加了密，暂不支持」，批次其余项不受影响

#### Scenario: 无可压缩空间
- **WHEN** 压缩后体积未小于原文件
- **THEN** 该项标「无需处理（PDF 已经很紧凑）」，不产出文件
