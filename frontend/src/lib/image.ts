// 管道契约：图片输出格式与处理状态的前端镜像。
//
// 单一真相在 Go 的 internal/imageproc（Output*）与 internal/service（Status*），
// 由 internal/guard/parity_test.go 断言两边逐项一致（漂移即失败）。

/** 输出格式取值。 */
export const OutputFormat = {
  Keep: 'keep',
  JPEG: 'jpeg',
  PNG: 'png',
  WebP: 'webp',
  /** 自动（最小体积）：逐项从 JPEG/WebP/PNG 里选输出体积最小者（US-49）。 */
  Auto: 'auto',
} as const

/** 输出格式取值类型。 */
export type OutputFormatValue = (typeof OutputFormat)[keyof typeof OutputFormat]

/** 单个文件的处理状态。 */
export const ItemStatus = {
  Success: 'success',
  Failed: 'failed',
  Skipped: 'skipped',
  Canceled: 'canceled',
  /** 处理中：只经进度事件出现，不会出现在最终批次结果里（US-14）。 */
  Processing: 'processing',
} as const

/** 处理状态取值类型。 */
export type ItemStatusValue = (typeof ItemStatus)[keyof typeof ItemStatus]

/** 尺寸盒宽高上下界。单一真相在 Go 的 internal/service（BoxMinDimension / BoxMaxDimension）。 */
export const BoxDimension = {
  Min: 1,
  Max: 65535,
} as const

/** 目标体积范围（字节）。单一真相在 Go 的 internal/service（TargetBytesMin / TargetBytesMax）。 */
export const TargetBytes = {
  Min: 1024,
  Max: 1073741824,
} as const

/** 输出目的地模式。单一真相在 Go 的 internal/service（OutputMode*）。 */
export const OutputMode = {
  /** 源文件同目录（默认）。 */
  Source: 'source',
  /** 自定义目录，平铺。 */
  CustomFlat: 'custom_flat',
  /** 自定义目录，按导入相对路径保留结构。 */
  CustomTree: 'custom_tree',
} as const

/** 输出目的地模式取值类型。 */
export type OutputModeValue = (typeof OutputMode)[keyof typeof OutputMode]
