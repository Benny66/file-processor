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
} as const

/** 输出格式取值类型。 */
export type OutputFormatValue = (typeof OutputFormat)[keyof typeof OutputFormat]

/** 单个文件的处理状态。 */
export const ItemStatus = {
  Success: 'success',
  Failed: 'failed',
  Skipped: 'skipped',
  Canceled: 'canceled',
} as const

/** 处理状态取值类型。 */
export type ItemStatusValue = (typeof ItemStatus)[keyof typeof ItemStatus]

/** 尺寸盒宽高上下界。单一真相在 Go 的 internal/service（BoxMinDimension / BoxMaxDimension）。 */
export const BoxDimension = {
  Min: 1,
  Max: 65535,
} as const
