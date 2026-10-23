package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
	"file-processor/internal/model"
	"file-processor/internal/page"
)

// validatePersistedRules 在批次规则之上要求目标体积是整数 KB/MB：
// UI 只有 KB/MB 两档，持久化的值必须能原样回填到输入框。
func validatePersistedRules(rules BatchRules) error {
	if err := validateBatchRules(rules); err != nil {
		return err
	}
	if rules.TargetBytes != 0 && rules.TargetBytes%1024 != 0 {
		return apperr.Validation("目标体积需为整数 KB 或 MB")
	}
	return nil
}

// GetImageSettings 返回上次使用的处理参数；旧配置的空格式按“保持原格式”处理。
// 值已不可用时返回默认规则与错误，调用方记录后继续使用默认值。
func (s *Service) GetImageSettings() (BatchRules, error) {
	if s.cfg == nil {
		return BatchRules{}, apperr.Wrap(errors.New("配置未初始化"))
	}
	format := s.cfg.ImageOutputFormat
	if format == "" {
		format = imageproc.OutputKeep
	}
	rules := BatchRules{
		OutputFormat: format,
		TargetBytes:  s.cfg.ImageTargetBytes,
		MaxWidth:     s.cfg.ImageMaxWidth,
		MaxHeight:    s.cfg.ImageMaxHeight,
	}
	if err := validatePersistedRules(rules); err != nil {
		return BatchRules{OutputFormat: imageproc.OutputKeep}, fmt.Errorf("图片设置无效，已使用默认值: %w", err)
	}
	return rules, nil
}

// SaveImageSettings 校验并持久化上次使用的处理参数。
func (s *Service) SaveImageSettings(rules BatchRules) error {
	if s.cfg == nil {
		return apperr.Wrap(errors.New("配置未初始化"))
	}
	if err := validatePersistedRules(rules); err != nil {
		return err
	}
	s.cfg.ImageOutputFormat = rules.OutputFormat
	s.cfg.ImageTargetBytes = rules.TargetBytes
	s.cfg.ImageMaxWidth = rules.MaxWidth
	s.cfg.ImageMaxHeight = rules.MaxHeight
	if err := s.cfg.Save(); err != nil {
		return apperr.Wrap(fmt.Errorf("保存图片设置失败: %w", err))
	}
	return nil
}

// ListPresets 按名称分页返回预设。
func (s *Service) ListPresets(req page.Request) (page.Result[model.ImagePreset], error) {
	if s.db == nil {
		return page.Result[model.ImagePreset]{}, apperr.Wrap(errors.New("数据库未初始化"))
	}
	normalized := req.Normalized()
	var total int64
	if err := s.db.Model(&model.ImagePreset{}).Count(&total).Error; err != nil {
		return page.Result[model.ImagePreset]{}, apperr.Wrap(fmt.Errorf("统计预设失败: %w", err))
	}
	var list []model.ImagePreset
	if err := s.db.Order("name ASC").Offset(normalized.Offset()).Limit(normalized.PageSize).Find(&list).Error; err != nil {
		return page.Result[model.ImagePreset]{}, apperr.Wrap(fmt.Errorf("查询预设失败: %w", err))
	}
	return page.NewResult(req, list, total), nil
}

// SavePreset 新建（ID=0）或更新预设；名称去空白后必须唯一。
func (s *Service) SavePreset(preset model.ImagePreset) (model.ImagePreset, error) {
	name := strings.TrimSpace(preset.Name)
	if name == "" {
		return model.ImagePreset{}, apperr.Validation("请输入预设名称")
	}
	rules := BatchRules{
		OutputFormat: preset.OutputFormat,
		TargetBytes:  preset.TargetBytes,
		MaxWidth:     preset.MaxWidth,
		MaxHeight:    preset.MaxHeight,
	}
	if err := validatePersistedRules(rules); err != nil {
		return model.ImagePreset{}, err
	}
	if s.db == nil {
		return model.ImagePreset{}, apperr.Wrap(errors.New("数据库未初始化"))
	}

	query := s.db.Model(&model.ImagePreset{}).Where("name = ?", name)
	if preset.ID != 0 {
		query = query.Where("id <> ?", preset.ID)
	}
	var duplicates int64
	if err := query.Count(&duplicates).Error; err != nil {
		return model.ImagePreset{}, apperr.Wrap(fmt.Errorf("校验预设名称失败: %w", err))
	}
	if duplicates > 0 {
		return model.ImagePreset{}, apperr.Validation("预设名称已存在")
	}

	if preset.ID == 0 {
		created := model.ImagePreset{
			Name:         name,
			OutputFormat: rules.OutputFormat,
			TargetBytes:  rules.TargetBytes,
			MaxWidth:     rules.MaxWidth,
			MaxHeight:    rules.MaxHeight,
		}
		if err := s.db.Create(&created).Error; err != nil {
			return model.ImagePreset{}, apperr.Wrap(fmt.Errorf("保存预设失败: %w", err))
		}
		return created, nil
	}

	var existing model.ImagePreset
	if err := s.db.First(&existing, preset.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ImagePreset{}, apperr.NotFound("预设不存在")
		}
		return model.ImagePreset{}, apperr.Wrap(fmt.Errorf("读取预设失败: %w", err))
	}
	existing.Name = name
	existing.OutputFormat = rules.OutputFormat
	existing.TargetBytes = rules.TargetBytes
	existing.MaxWidth = rules.MaxWidth
	existing.MaxHeight = rules.MaxHeight
	if err := s.db.Save(&existing).Error; err != nil {
		return model.ImagePreset{}, apperr.Wrap(fmt.Errorf("更新预设失败: %w", err))
	}
	return existing, nil
}

// DeletePreset 删除预设。
// 使用硬删除：名称上的唯一索引包含软删除行，软删除会让旧名称永久不可复用。
func (s *Service) DeletePreset(id uint) error {
	if id == 0 {
		return apperr.Validation("预设 ID 无效")
	}
	if s.db == nil {
		return apperr.Wrap(errors.New("数据库未初始化"))
	}
	result := s.db.Unscoped().Delete(&model.ImagePreset{}, id)
	if result.Error != nil {
		return apperr.Wrap(fmt.Errorf("删除预设失败: %w", result.Error))
	}
	if result.RowsAffected == 0 {
		return apperr.NotFound("预设不存在")
	}
	return nil
}
