// Package config 负责应用本地配置（config.json）的读写。
//
// 配置落在应用数据目录（见 internal/appdir，与数据库/日志同目录）。
// 首次启动无配置文件时写入默认值。
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"file-processor/internal/appdir"
)

// 窗口尺寸的默认值与合法范围（单一真相处；回退与夹取行为由 config_test.go 的
// TestWindowSizeClamps 覆盖——曾经这里写着「config 与护栏共同引用」而 guard 里并无对应检查，
// 注释承诺了一个不存在的护栏，比不写更糟）。
const (
	// DefaultWindowWidth / DefaultWindowHeight 首次启动或配置不可用时的窗口尺寸。
	DefaultWindowWidth  = 1024
	DefaultWindowHeight = 768

	// MinWindowWidth / MinWindowHeight 窗口尺寸下界。
	// 配置是用户可编辑的文件，手改坏了不能把窗口弄成不可用（见 WindowSize）。
	MinWindowWidth  = 640
	MinWindowHeight = 420
)

// Config 是应用配置的持久化结构。
// 字段带 json tag，新增配置项在此扩展并给默认值。
type Config struct {
	// Theme 界面主题：dark / light。空串（""）表示"未设置"，语义为"跟随系统"。
	Theme string `json:"theme"`

	// WindowWidth / WindowHeight 上次关闭时的窗口尺寸。
	// 0 表示"未记录"（首次启动），此时取默认值。
	WindowWidth  int `json:"window_width"`
	WindowHeight int `json:"window_height"`

	// WindowMaximised 上次关闭时窗口是否处于最大化。true 时启动直接最大化，
	// 尺寸字段仍保留（还原后回到该尺寸）。
	WindowMaximised bool `json:"window_maximised"`

	// 上次使用的图片处理参数。0 表示关闭；空 OutputFormat 表示保持原格式
	// （旧配置没有这些字段，读取方按默认语义处理）。
	ImageOutputFormat string `json:"image_output_format"`
	ImageTargetBytes  int64  `json:"image_target_bytes"`
	ImageMaxWidth     int    `json:"image_max_width"`
	ImageMaxHeight    int    `json:"image_max_height"`

	// path 是配置文件路径，不序列化（json:"-"）。
	path string `json:"-"`
}

// Default 返回默认配置。
// 字段默认值为空串/零值，表示"未设置"——由调用方决定 fallback 行为，
// 而非硬编码一个具体默认值（如 Theme 空串 = 跟随系统，窗口尺寸 0 = 未记录）。
func Default() *Config {
	return &Config{Theme: ""}
}

// WindowSize 返回可直接用于创建窗口的尺寸：非法值（未记录、负数、过小）一律回退默认值。
//
// 配置是用户可写文件，取值必须经此夹取，否则一个手改坏的 config.json
// 会让窗口小到不可用（且用户不知道要去改配置）。
// 上限不夹取：用户把窗口拉到 4K 全屏是正常诉求，屏幕装不下由窗口管理器处理。
func (c *Config) WindowSize() (width, height int) {
	width, height = c.WindowWidth, c.WindowHeight
	if width < MinWindowWidth {
		width = DefaultWindowWidth
	}
	if height < MinWindowHeight {
		height = DefaultWindowHeight
	}
	return width, height
}

// Load 读取配置；文件不存在时写入默认值并返回默认配置。
//
// 配置是可选项：读取失败或内容损坏时返回【可用的默认配置】以及错误，
// 调用方应记录错误但继续启动（US-13）。只有连配置路径都无法确定时才返回 nil。
func Load(appName string) (*Config, error) {
	path, err := appdir.File(appName, "config.json")
	if err != nil {
		return nil, err
	}

	c := Default()
	c.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := c.Save(); err != nil {
				return c, fmt.Errorf("写入默认配置失败: %w", err)
			}
			return c, nil
		}
		return c, fmt.Errorf("读取配置失败，已使用默认值: %w", err)
	}

	if err := json.Unmarshal(data, c); err != nil {
		fallback := Default()
		fallback.path = path
		if saveErr := fallback.Save(); saveErr != nil {
			return fallback, fmt.Errorf("配置损坏，重写默认值失败: %w", saveErr)
		}
		return fallback, fmt.Errorf("配置损坏，已回退默认值: %w", err)
	}
	return c, nil
}

// Save 将当前配置写回文件。
func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("配置未绑定路径")
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}
