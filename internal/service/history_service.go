package service

// 历史记录与下载（ADR-0007 / US-59~64）：处理结果只进暂存区，下载是唯一落盘出口。
// 本文件承接原「输出目的地」三模式的全部机制（命名渲染/保留层级/防覆盖/原子落位），
// 触发点从「处理时」移到「下载时」。

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
	"file-processor/internal/model"
	"file-processor/internal/page"
)

// historyKeepBatches 历史保留最近多少次处理（D-56）；更早的清理暂存图片、保留记录。
const historyKeepBatches = 30

// historySeq 同秒批次的去重序号。
var historySeq atomic.Uint64

// newBatchKey 生成暂存目录名（时间 + 序号，人类可读且稳定排序）。
func newBatchKey() string {
	return fmt.Sprintf("%s-%03d", time.Now().Format("20060102-150405"), historySeq.Add(1)%1000)
}

func (s *Service) stagingDir(batchKey string) string {
	return filepath.Join(s.historyRoot, batchKey)
}

// DownloadOptions 是一次下载的选项（D-55/D-58）：目录 + 是否保留导入层级 + 文件名格式。
type DownloadOptions struct {
	Directory     string `json:"directory"`
	KeepStructure bool   `json:"keep_structure"`
	Pattern       string `json:"pattern"`
}

// DownloadResult 是一张图的下载结果；失败时 Path 为源路径、Message 给出人话原因。
type DownloadResult struct {
	Path    string `json:"path"`
	Message string `json:"message,omitempty"`
	OK      bool   `json:"ok"`
}

// GetDownloadOptions 返回下载选项的记忆值（D-58）：目录为空时回退 系统「下载」文件夹 → 用户主目录。
func (s *Service) GetDownloadOptions() (DownloadOptions, error) {
	opts := DownloadOptions{}
	if s.cfg != nil {
		opts.Directory = s.cfg.DownloadDir
		opts.KeepStructure = s.cfg.DownloadKeepStructure
		opts.Pattern = s.cfg.DownloadPattern
	}
	if strings.TrimSpace(opts.Directory) == "" || !dirExists(opts.Directory) {
		opts.Directory = defaultDownloadDir()
	}
	return opts, nil
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	downloads := filepath.Join(home, "Downloads")
	if dirExists(downloads) {
		return downloads
	}
	return home
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// DownloadItems 把一批暂存图片保存到用户目录（US-60）：按清单顺序渲染文件名
// （{n} = 清单内序号），保留层级时按导入相对路径下钻，重名防覆盖（-2/-3），
// 原子落位（临时文件同目录 + 链接/独占复制）。成功后记忆选项到 config。
func (s *Service) DownloadItems(batchKey string, sourcePaths []string, opts DownloadOptions) ([]DownloadResult, error) {
	if len(sourcePaths) == 0 {
		return nil, apperr.Validation("没有要保存的图片")
	}
	if strings.TrimSpace(opts.Directory) == "" || !filepath.IsAbs(filepath.Clean(opts.Directory)) {
		return nil, apperr.Validation("请先选择保存的文件夹")
	}
	if err := validatePattern(opts.Pattern); err != nil {
		return nil, err
	}
	if err := ensureDir(opts.Directory); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, apperr.Wrap(errors.New("数据库未初始化"))
	}
	var batch model.HistoryBatch
	if err := s.db.Where("batch_key = ?", batchKey).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("找不到这次处理")
		}
		return nil, apperr.Wrap(fmt.Errorf("读取处理记录失败: %w", err))
	}
	if batch.FilesPurged {
		return nil, apperr.NotFound("这批图片已被清理，不能再保存")
	}
	var items []model.HistoryItem
	if err := s.db.Where("batch_id = ?", batch.ID).Find(&items).Error; err != nil {
		return nil, apperr.Wrap(fmt.Errorf("读取图片明细失败: %w", err))
	}
	byPath := make(map[string]*model.HistoryItem, len(items))
	for i := range items {
		byPath[items[i].SourcePath] = &items[i]
	}

	results := make([]DownloadResult, 0, len(sourcePaths))
	saved := 0
	for n, path := range sourcePaths {
		res := downloadOne(s, batchKey, path, n+1, opts, byPath[path])
		if res.OK {
			saved++
		}
		results = append(results, res)
	}
	if saved > 0 && s.cfg != nil {
		s.cfg.DownloadDir = opts.Directory
		s.cfg.DownloadKeepStructure = opts.KeepStructure
		s.cfg.DownloadPattern = opts.Pattern
		if err := s.cfg.Save(); err != nil {
			slog.Warn("记住下载选项失败", "err", err)
		}
	}
	return results, nil
}

func downloadOne(s *Service, batchKey, path string, index int, opts DownloadOptions, item *model.HistoryItem) DownloadResult {
	if item == nil {
		return DownloadResult{Path: path, OK: false, Message: "这张图不在这次处理里"}
	}
	if item.Status != string(StatusSuccess) || item.StagingName == "" {
		return DownloadResult{Path: path, OK: false, Message: "这张图没有可保存的结果"}
	}
	data, err := os.ReadFile(filepath.Join(s.stagingDir(batchKey), item.StagingName))
	if err != nil {
		return DownloadResult{Path: path, OK: false, Message: "图片文件不在了（可能已被清理）"}
	}
	destDir := opts.Directory
	if opts.KeepStructure {
		rel, err := safeRelDir(item.RelPath, item.SourceName)
		if err != nil {
			return DownloadResult{Path: path, OK: false, Message: err.Error()}
		}
		if rel != "." {
			destDir = filepath.Join(destDir, rel)
			if err := ensureDir(destDir); err != nil {
				return DownloadResult{Path: path, OK: false, Message: err.Error()}
			}
		}
	}
	stem := strings.TrimSuffix(item.SourceName, filepath.Ext(item.SourceName))
	name := renderOutputStem(opts.Pattern, stem, index)
	finalPath, err := writeConvertedFile(destDir, name, filepath.Ext(item.StagingName), data)
	if err != nil {
		return DownloadResult{Path: path, OK: false, Message: fmt.Sprintf("保存失败: %v", err)}
	}
	item.Downloaded = true
	item.DownloadedAt = time.Now().Unix()
	if err := s.db.Save(item).Error; err != nil {
		slog.Warn("记录下载状态失败", "path", path, "err", err)
	}
	return DownloadResult{Path: finalPath, OK: true}
}

// safeRelDir 校验导入相对路径并取其目录部分（下载保留层级用）。
// 相对路径来自绑定入参，不可信：挡住 .. 与绝对路径，防写出目标目录之外。
func safeRelDir(rel, fallbackName string) (string, error) {
	clean := filepath.Clean(rel)
	if clean == "" || clean == "." {
		clean = fallbackName
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", apperr.Validation("文件夹层级不合法")
	}
	return filepath.Dir(clean), nil
}

// ensureDir 创建目录（含父级）；下载目标目录不存在时自动建（US-60）。
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return apperr.Validation(fmt.Sprintf("无法创建保存的文件夹: %v", err))
	}
	return nil
}

// persistHistory 把一次处理写入历史（best-effort：失败只记日志，不影响批次结果）。
func (s *Service) persistHistory(batchKey string, req BatchRequest, summary BatchResult) {
	if s.db == nil {
		return
	}
	batch := model.HistoryBatch{
		BatchKey:     batchKey,
		OutputFormat: req.Rules.OutputFormat,
		TargetBytes:  req.Rules.TargetBytes,
		MaxWidth:     req.Rules.MaxWidth,
		MaxHeight:    req.Rules.MaxHeight,
		Summary:      describeRulesHuman(req.Rules),
		Total:        len(summary.Items),
		Success:      summary.Success,
		Failed:       summary.Failed,
		Skipped:      summary.Skipped,
		Canceled:     summary.Canceled,
		SavedBytes:   summary.SavedBytes,
	}
	if err := s.db.Create(&batch).Error; err != nil {
		slog.Warn("历史批次写入失败", "err", err)
		return
	}
	for i, res := range summary.Items {
		item := model.HistoryItem{
			BatchID:     batch.ID,
			SourcePath:  res.Path,
			SourceName:  filepath.Base(res.Path),
			SourceBytes: res.SourceBytes,
			RelPath:     req.Items[i].RelPath,
			Status:      string(res.Status),
			Message:     res.Message,
			Format:      res.Format,
			Width:       res.Width,
			Height:      res.Height,
			OutputBytes: res.OutputBytes,
		}
		if res.Status == StatusSuccess && res.OutputPath != "" {
			item.StagingName = filepath.Base(res.OutputPath)
			// 缩略图由输出字节生成（通常已小于原图）；失败不阻断——明细只是少了预览。
			if data, err := os.ReadFile(res.OutputPath); err == nil {
				if thumb, err := imageproc.ThumbnailDataURL(data, imageproc.ThumbnailMaxEdge); err == nil {
					item.Thumbnail = thumb
				}
			}
		}
		if err := s.db.Create(&item).Error; err != nil {
			slog.Warn("历史明细写入失败", "path", res.Path, "err", err)
		}
	}
	if err := s.PruneHistory(); err != nil {
		slog.Warn("历史修剪失败", "err", err)
	}
}

// describeRulesHuman 处理设置的人话摘要（历史条目展示用；写入时生成一份，读取方不再拼装）。
func describeRulesHuman(rules BatchRules) string {
	var parts []string
	switch rules.OutputFormat {
	case imageproc.OutputKeep:
		parts = append(parts, "保持原格式")
	case imageproc.OutputAuto:
		parts = append(parts, "自动选最省空间的格式")
	default:
		parts = append(parts, "转成 "+formatDisplayName(rules.OutputFormat))
	}
	if rules.TargetBytes > 0 {
		parts = append(parts, "压到 "+humanBytes(rules.TargetBytes))
	}
	if rules.MaxWidth > 0 {
		parts = append(parts, fmt.Sprintf("最宽 %d", rules.MaxWidth))
	}
	return strings.Join(parts, " · ")
}

func humanBytes(b int64) string {
	const mb = 1024 * 1024
	if b >= mb && b%mb == 0 {
		return fmt.Sprintf("%d MB", b/mb)
	}
	return fmt.Sprintf("%d KB", (b+1023)/1024)
}

// PruneHistory 只保留最近 historyKeepBatches 次处理的暂存图片（D-56）：
// 更早的删除暂存目录、置 FilesPurged、清明细的 StagingName——记录仍在，不可再下载。
func (s *Service) PruneHistory() error {
	if s.db == nil {
		return nil
	}
	var batches []model.HistoryBatch
	if err := s.db.Order("created_at DESC, id DESC").Find(&batches).Error; err != nil {
		return err
	}
	if len(batches) <= historyKeepBatches {
		return nil
	}
	for _, batch := range batches[historyKeepBatches:] {
		if batch.FilesPurged {
			continue
		}
		if err := os.RemoveAll(s.stagingDir(batch.BatchKey)); err != nil {
			slog.Warn("清理暂存目录失败", "key", batch.BatchKey, "err", err)
		}
		if err := s.db.Model(&model.HistoryBatch{}).Where("id = ?", batch.ID).
			Update("files_purged", true).Error; err != nil {
			return err
		}
		if err := s.db.Model(&model.HistoryItem{}).Where("batch_id = ?", batch.ID).
			Update("staging_name", "").Error; err != nil {
			return err
		}
	}
	return nil
}

// ClearHistory 清空全部历史（记录 + 暂存图片）。
func (s *Service) ClearHistory() error {
	if s.db == nil {
		return apperr.Wrap(errors.New("数据库未初始化"))
	}
	if err := s.db.Unscoped().Where("1 = 1").Delete(&model.HistoryItem{}).Error; err != nil {
		return apperr.Wrap(fmt.Errorf("清空历史明细失败: %w", err))
	}
	if err := s.db.Unscoped().Where("1 = 1").Delete(&model.HistoryBatch{}).Error; err != nil {
		return apperr.Wrap(fmt.Errorf("清空历史记录失败: %w", err))
	}
	if s.historyRoot != "" {
		if err := os.RemoveAll(s.historyRoot); err != nil {
			return apperr.Wrap(fmt.Errorf("清理暂存图片失败: %w", err))
		}
		if err := os.MkdirAll(s.historyRoot, 0o755); err != nil {
			return apperr.Wrap(fmt.Errorf("重建暂存目录失败: %w", err))
		}
	}
	return nil
}

// HistoryUsage 统计暂存图片的磁盘占用（设置页展示用）。
func (s *Service) HistoryUsage() (int64, error) {
	if s.historyRoot == "" {
		return 0, nil
	}
	var total int64
	err := filepath.WalkDir(s.historyRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, apperr.Wrap(fmt.Errorf("统计暂存占用失败: %w", err))
	}
	return total, nil
}

// ListHistoryBatches 分页返回历史批次（最近在前）。
func (s *Service) ListHistoryBatches(req page.Request) (page.Result[model.HistoryBatch], error) {
	if s.db == nil {
		return page.Result[model.HistoryBatch]{}, apperr.Wrap(errors.New("数据库未初始化"))
	}
	normalized := req.Normalized()
	var total int64
	if err := s.db.Model(&model.HistoryBatch{}).Count(&total).Error; err != nil {
		return page.Result[model.HistoryBatch]{}, apperr.Wrap(fmt.Errorf("统计历史失败: %w", err))
	}
	var list []model.HistoryBatch
	if err := s.db.Order("created_at DESC, id DESC").
		Offset(normalized.Offset()).Limit(normalized.PageSize).Find(&list).Error; err != nil {
		return page.Result[model.HistoryBatch]{}, apperr.Wrap(fmt.Errorf("查询历史失败: %w", err))
	}
	return page.NewResult(req, list, total), nil
}

// ListHistoryItems 分页返回某次处理的图片明细（按入队顺序）。
func (s *Service) ListHistoryItems(batchKey string, req page.Request) (page.Result[model.HistoryItem], error) {
	if s.db == nil {
		return page.Result[model.HistoryItem]{}, apperr.Wrap(errors.New("数据库未初始化"))
	}
	var batch model.HistoryBatch
	if err := s.db.Where("batch_key = ?", batchKey).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return page.Result[model.HistoryItem]{}, apperr.NotFound("找不到这次处理")
		}
		return page.Result[model.HistoryItem]{}, apperr.Wrap(fmt.Errorf("读取处理记录失败: %w", err))
	}
	normalized := req.Normalized()
	var total int64
	if err := s.db.Model(&model.HistoryItem{}).Where("batch_id = ?", batch.ID).Count(&total).Error; err != nil {
		return page.Result[model.HistoryItem]{}, apperr.Wrap(fmt.Errorf("统计明细失败: %w", err))
	}
	var list []model.HistoryItem
	if err := s.db.Where("batch_id = ?", batch.ID).Order("id ASC").
		Offset(normalized.Offset()).Limit(normalized.PageSize).Find(&list).Error; err != nil {
		return page.Result[model.HistoryItem]{}, apperr.Wrap(fmt.Errorf("查明细失败: %w", err))
	}
	return page.NewResult(req, list, total), nil
}
