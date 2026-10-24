package model

// HistoryBatch / HistoryItem 是「历史记录」的持久化模型（ADR-0007 / US-62~64）：
// 一次处理一条批次记录，其下逐图一条明细；成功项的图片文件在暂存区
// （应用数据目录 history/<BatchKey>/），文件名记在 StagingName。
// 暂存图片被历史清理后（保留最近 N 次），行保留、StagingName 置空、FilesPurged 置真——
// 记录还在、不能再下载。

// HistoryBatch 是一次处理的汇总记录。
type HistoryBatch struct {
	BaseModel
	// BatchKey 暂存目录名，与磁盘一一对应。
	BatchKey string `gorm:"size:64;not null;uniqueIndex" json:"batch_key"`
	// 处理设置快照（规则四元组）与写入时生成的人话摘要（读取方不再拼装）。
	OutputFormat string `gorm:"size:8;not null" json:"output_format"`
	TargetBytes  int64  `gorm:"not null;default:0" json:"target_bytes"`
	MaxWidth     int    `gorm:"not null;default:0" json:"max_width"`
	MaxHeight    int    `gorm:"not null;default:0" json:"max_height"`
	Summary      string `gorm:"size:255;not null" json:"summary"`

	Total      int   `gorm:"not null;default:0" json:"total"`
	Success    int   `gorm:"not null;default:0" json:"success"`
	Failed     int   `gorm:"not null;default:0" json:"failed"`
	Skipped    int   `gorm:"not null;default:0" json:"skipped"`
	Canceled   int   `gorm:"not null;default:0" json:"canceled"`
	SavedBytes int64 `gorm:"not null;default:0" json:"saved_bytes"`

	// FilesPurged 暂存图片已被历史清理：记录可见、不可再下载。
	FilesPurged bool `gorm:"not null;default:false" json:"files_purged"`
}

// HistoryItem 是一次处理里一张图的明细。
type HistoryItem struct {
	BaseModel
	BatchID uint `gorm:"index;not null" json:"batch_id"`

	SourcePath  string `gorm:"size:1024;not null" json:"source_path"`
	SourceName  string `gorm:"size:255;not null" json:"source_name"`
	SourceBytes int64  `gorm:"not null;default:0" json:"source_bytes"`
	// RelPath 导入根下的相对路径（下载「保留文件夹层级」用）。
	RelPath string `gorm:"size:1024;not null;default:''" json:"rel_path"`

	Status      string `gorm:"size:16;not null" json:"status"`
	Message     string `gorm:"size:512;not null;default:''" json:"message"`
	Format      string `gorm:"size:8;not null;default:''" json:"format"`
	Width       int    `gorm:"not null;default:0" json:"width"`
	Height      int    `gorm:"not null;default:0" json:"height"`
	OutputBytes int64  `gorm:"not null;default:0" json:"output_bytes"`
	// Thumbnail 96px base64 缩略图：源文件被删/移动后历史明细仍可看。
	Thumbnail string `gorm:"size:65535;not null;default:''" json:"thumbnail"`
	// StagingName 暂存文件名（成功项）；清理后置空。
	StagingName string `gorm:"size:64;not null;default:''" json:"staging_name"`

	Downloaded   bool  `gorm:"not null;default:false" json:"downloaded"`
	DownloadedAt int64 `gorm:"not null;default:0" json:"downloaded_at"`
}
