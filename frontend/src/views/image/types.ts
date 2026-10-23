import type { InspectImage, ScanPaths } from '../../../wailsjs/go/main/App'

export type ScanResult = Awaited<ReturnType<typeof ScanPaths>>
export type ScannedFile = ScanResult['files'][number]
export type ImageInfo = Awaited<ReturnType<typeof InspectImage>>

export type QueueItem = ScannedFile & {
  info: ImageInfo | null
  error: string | null
  inspecting: boolean
}
