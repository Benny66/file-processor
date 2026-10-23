package model

// ImagePreset 是图片处理参数的命名预设。
// 名称在 service 层去除首尾空白后校验唯一性；唯一索引兜底并发写入。
type ImagePreset struct {
	BaseModel

	Name         string `gorm:"size:100;not null;uniqueIndex" json:"name"`
	OutputFormat string `gorm:"size:8;not null" json:"output_format"`
	TargetBytes  int64  `gorm:"not null;default:0" json:"target_bytes"`
	MaxWidth     int    `gorm:"not null;default:0" json:"max_width"`
	MaxHeight    int    `gorm:"not null;default:0" json:"max_height"`
}
