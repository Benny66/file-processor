import type { InspectImage, ProcessBatch, ScanPaths } from '../../../wailsjs/go/main/App'

export type ScanResult = Awaited<ReturnType<typeof ScanPaths>>
export type ScannedFile = ScanResult['files'][number]
export type ImageInfo = Awaited<ReturnType<typeof InspectImage>>
export type BatchResult = Awaited<ReturnType<typeof ProcessBatch>>
export type ItemResult = BatchResult['items'][number]

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
