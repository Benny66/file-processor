import { ItemStatus } from '../../lib/image'
import type {
  InspectImage,
  ListPresets,
  ProcessBatch,
  RenderPreview,
  ScanPaths,
} from '../../../wailsjs/go/main/App'
import type { service } from '../../../wailsjs/go/models'

export type ScanResult = Awaited<ReturnType<typeof ScanPaths>>
export type ScannedFile = ScanResult['files'][number]
export type ImageInfo = Awaited<ReturnType<typeof InspectImage>>
export type BatchResult = Awaited<ReturnType<typeof ProcessBatch>>
export type ItemResult = BatchResult['items'][number]
export type Preview = Awaited<ReturnType<typeof RenderPreview>>
export type Preset = Awaited<ReturnType<typeof ListPresets>>['list'][number]

/**
 * 保存预设与上次设置使用的规则快照。
 * 直接取 wails 生成物（service.BatchRules），不再手写一份字段镜像——
 * 生成的类型跟着 Go 结构体走，手写的只会跟着人的记性走。
 */
export type PresetRules = service.BatchRules

/**
 * 批次进度事件载荷（对应 Go 的 service.BatchProgress）。
 * 事件不经绑定生成类型，故这里保留一份【类型】镜像——只镜像类型，不镜像判断逻辑。
 */
export interface ImageProgress {
  done: number
  total: number
  current?: ItemResult
  /** 刚被 worker 领取的项（status=processing，US-14 的「处理中」来源）。 */
  started?: ItemResult
}

export type QueueItem = ScannedFile & {
  info: ImageInfo | null
  error: string | null
  inspecting: boolean
  result: ItemResult | null
  /** 批次已开跑、该项还没被 worker 领取（US-14 的「排队」）。 */
  queued: boolean
  /** 该项正被 worker 处理（来自 Go 的 started 事件，前端不自己猜）。 */
  processing: boolean
}

/**
 * 结果态是否可发起队列内重跑（US-43/D-38a：失败与已取消）。
 *
 * 为什么在 views 层而不是 lib/image.ts：lib/* 是契约镜像文件，只许放常量与类型——
 * 这是交互策略而非镜像。Go 侧没有也不需要同一判定（重跑只是对无状态批次接口的
 * 子集调用），集中一处供 QueueRow 与 ImageWorkbench 共用即可。
 */
export function isRetryableStatus(status: string): boolean {
  return status === ItemStatus.Failed || status === ItemStatus.Canceled
}
