import type {
  InspectImage,
  ListPresets,
  ProcessBatch,
  RenderPreview,
  ScanPaths,
} from '../../../wailsjs/go/main/App'

export type ScanResult = Awaited<ReturnType<typeof ScanPaths>>
export type ScannedFile = ScanResult['files'][number]
export type ImageInfo = Awaited<ReturnType<typeof InspectImage>>
export type BatchResult = Awaited<ReturnType<typeof ProcessBatch>>
export type ItemResult = BatchResult['items'][number]
export type Preview = Awaited<ReturnType<typeof RenderPreview>>
export type Preset = Awaited<ReturnType<typeof ListPresets>>['list'][number]

/** 保存预设与上次设置时使用的规则快照。 */
export interface PresetRules {
  output_format: string
  target_bytes: number
  max_width: number
  max_height: number
}

export interface ImageProgress {
  done: number
  total: number
  current?: ItemResult
}

export type QueueItem = ScannedFile & {
  info: ImageInfo | null
  error: string | null
  inspecting: boolean
  result: ItemResult | null
}
