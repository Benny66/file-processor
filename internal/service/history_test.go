package service

// 暂存 + 下载 + 历史（ADR-0007 / US-59~64）的单元测试。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"file-processor/internal/config"
	"file-processor/internal/imageproc"
	"file-processor/internal/model"
	"file-processor/internal/page"
)

func newHistoryTestService(t *testing.T) *Service {
	t.Helper()
	isolateHome(t)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "history.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("迁移模型失败: %v", err)
	}
	cfg, err := config.Load("historytest")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	svc := New(db, cfg)
	svc.SetHistoryRoot(t.TempDir())
	return svc
}

// runTestBatch 跑一个含「成功 + 失败」的批次并返回结果。
func runTestBatch(t *testing.T, svc *Service, dir string) BatchResult {
	t.Helper()
	valid := filepath.Join(dir, "ok.png")
	corrupt := filepath.Join(dir, "bad.jpg")
	if err := os.WriteFile(valid, batchTestPNG(t, 40, 30), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(valid, corrupt),
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	return result
}

// TestProcessBatchStagesAndPersistsHistory US-59/62：结果进暂存区、批次与明细落历史库，
// 成功项带缩略图与暂存文件名，失败项没有。
func TestProcessBatchStagesAndPersistsHistory(t *testing.T) {
	svc := newHistoryTestService(t)
	srcDir := t.TempDir()
	result := runTestBatch(t, svc, srcDir)

	if result.BatchKey == "" {
		t.Fatal("结果缺 BatchKey")
	}
	staged := filepath.Join(svc.historyRoot, result.BatchKey, "0001.jpg")
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("暂存文件不存在: %v", err)
	}

	var batch model.HistoryBatch
	if err := svc.db.Where("batch_key = ?", result.BatchKey).First(&batch).Error; err != nil {
		t.Fatalf("历史批次未落库: %v", err)
	}
	if batch.Success != 1 || batch.Failed != 1 || batch.Total != 2 {
		t.Errorf("批次统计不对：%+v", batch)
	}
	if !strings.Contains(batch.Summary, "转成 JPEG") {
		t.Errorf("人话摘要不对：%q", batch.Summary)
	}
	var items []model.HistoryItem
	if err := svc.db.Where("batch_id = ?", batch.ID).Order("id ASC").Find(&items).Error; err != nil {
		t.Fatalf("明细未落库: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("明细条数 = %d", len(items))
	}
	if items[0].Status != string(StatusSuccess) || items[0].StagingName == "" {
		t.Errorf("成功明细缺暂存名：%+v", items[0])
	}
	if !strings.HasPrefix(items[0].Thumbnail, "data:image/") {
		t.Errorf("成功明细缺缩略图：%q", items[0].Thumbnail[:min(30, len(items[0].Thumbnail))])
	}
	if items[1].Status != string(StatusFailed) || items[1].StagingName != "" {
		t.Errorf("失败明细不应有暂存名：%+v", items[1])
	}
}

// TestDownloadItemsFlatPatternCollision US-60：平铺命名 + 防覆盖 + 已下载标记 + 选项记忆。
func TestDownloadItemsFlatPatternCollision(t *testing.T) {
	svc := newHistoryTestService(t)
	srcDir := t.TempDir()
	valid := filepath.Join(srcDir, "ok.png")
	if err := os.WriteFile(valid, batchTestPNG(t, 40, 30), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	result, err := svc.ProcessBatch(BatchRequest{
		Items: batchItems(valid),
		Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}

	dest := t.TempDir()
	// 预占第一个候选名：下载命名的防覆盖序号同样生效。
	if err := os.WriteFile(filepath.Join(dest, "img-1.jpg"), []byte("old"), 0o644); err != nil {
		t.Fatalf("预占候选名失败: %v", err)
	}
	results, err := svc.DownloadItems(result.BatchKey, []string{valid}, DownloadOptions{
		Directory: dest,
		Pattern:   "img-{n}",
	})
	if err != nil {
		t.Fatalf("DownloadItems 失败: %v", err)
	}
	if len(results) != 1 || !results[0].OK || results[0].Path != filepath.Join(dest, "img-1-2.jpg") {
		t.Fatalf("下载结果不对：%+v", results)
	}
	if data, _ := os.ReadFile(filepath.Join(dest, "img-1.jpg")); string(data) != "old" {
		t.Errorf("已有文件被覆盖")
	}

	// 已下载标记 + 选项记忆（目录/格式随成功下载写回 config）。
	var item model.HistoryItem
	if err := svc.db.Where("batch_id = (SELECT id FROM history_batches WHERE batch_key = ?)", result.BatchKey).
		First(&item).Error; err != nil || !item.Downloaded {
		t.Errorf("已下载标记未落库：%v %+v", err, item)
	}
	opts, _ := svc.GetDownloadOptions()
	if opts.Directory != dest || opts.Pattern != "img-{n}" {
		t.Errorf("下载选项未记忆：%+v", opts)
	}
}

// TestDownloadItemsKeepStructureAndTraversal US-60：保留层级按导入相对路径下钻；
// 相对路径不可信（..）时该张失败，绝不写出目标目录之外。
func TestDownloadItemsKeepStructureAndTraversal(t *testing.T) {
	svc := newHistoryTestService(t)
	srcRoot := t.TempDir()
	sub := filepath.Join(srcRoot, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建子目录失败: %v", err)
	}
	inside := filepath.Join(sub, "deep.png")
	if err := os.WriteFile(inside, batchTestPNG(t, 40, 30), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	result, err := svc.ProcessBatch(BatchRequest{
		Items: []BatchItem{{Path: inside, RelPath: filepath.Join("sub", "deep.png")}},
		Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}

	dest := t.TempDir()
	results, err := svc.DownloadItems(result.BatchKey, []string{inside}, DownloadOptions{
		Directory:     dest,
		KeepStructure: true,
	})
	if err != nil {
		t.Fatalf("DownloadItems 失败: %v", err)
	}
	if !results[0].OK || results[0].Path != filepath.Join(dest, "sub", "deep-out.jpg") {
		t.Fatalf("保留层级下载不对：%+v", results)
	}

	// 篡改相对路径为穿越后：该项失败，dest 之外无文件。
	svc.db.Model(&model.HistoryItem{}).Where("source_path = ?", inside).
		Update("rel_path", filepath.Join("..", "escape.png"))
	results, err = svc.DownloadItems(result.BatchKey, []string{inside}, DownloadOptions{
		Directory:     dest,
		KeepStructure: true,
	})
	if err != nil {
		t.Fatalf("DownloadItems 不应整体失败: %v", err)
	}
	if results[0].OK || !strings.Contains(results[0].Message, "不合法") {
		t.Fatalf("穿越相对路径应单项失败：%+v", results[0])
	}
	rootEntries, _ := os.ReadDir(filepath.Dir(dest))
	for _, entry := range rootEntries {
		if entry.Name() != filepath.Base(dest) && entry.Name() == "escape.png" {
			t.Errorf("输出目录之外出现文件：%s", entry.Name())
		}
	}
}

// TestDownloadItemsEdgeCases 不在批次里的路径 / 已清理批次：人话失败，不崩溃。
func TestDownloadItemsEdgeCases(t *testing.T) {
	svc := newHistoryTestService(t)
	srcDir := t.TempDir()
	result := runTestBatch(t, svc, srcDir)
	valid := filepath.Join(srcDir, "ok.png")

	dest := t.TempDir()
	results, err := svc.DownloadItems(result.BatchKey, []string{valid, filepath.Join(srcDir, "nope.png")}, DownloadOptions{Directory: dest})
	if err != nil {
		t.Fatalf("DownloadItems 失败: %v", err)
	}
	if !results[0].OK {
		t.Errorf("有效项应成功：%+v", results[0])
	}
	if results[1].OK || !strings.Contains(results[1].Message, "不在这次处理里") {
		t.Errorf("未知路径应人话失败：%+v", results[1])
	}

	svc.db.Model(&model.HistoryBatch{}).Where("batch_key = ?", result.BatchKey).Update("files_purged", true)
	if _, err := svc.DownloadItems(result.BatchKey, []string{valid}, DownloadOptions{Directory: dest}); err == nil ||
		!strings.Contains(err.Error(), "清理") {
		t.Errorf("已清理批次应整体报错：%v", err)
	}
	if _, err := svc.DownloadItems(result.BatchKey, nil, DownloadOptions{Directory: dest}); err == nil {
		t.Error("空清单应报错")
	}
	if _, err := svc.DownloadItems(result.BatchKey, []string{valid}, DownloadOptions{Directory: "relative/dir"}); err == nil {
		t.Error("相对目录应报错")
	}
	if _, err := svc.DownloadItems(result.BatchKey, []string{valid}, DownloadOptions{Directory: dest, Pattern: "photo"}); err == nil {
		t.Error("缺变量格式应报错")
	} else {
		requireValidation(t, err)
	}
}

// TestPruneHistoryKeepsRecent D-56：只保留最近 N 次的可下载暂存；更早的清文件、
// 留记录（FilesPurged），再下载被拒。
func TestPruneHistoryKeepsRecent(t *testing.T) {
	svc := newHistoryTestService(t)
	// 直接造 32 条批次 + 对应暂存目录（比跑 32 次真实批次快得多）。
	for i := 0; i < historyKeepBatches+2; i++ {
		batch := model.HistoryBatch{BatchKey: newBatchKey(), Summary: "test", Total: 1, Success: 1}
		if err := svc.db.Create(&batch).Error; err != nil {
			t.Fatalf("造批次失败: %v", err)
		}
		dir := svc.stagingDir(batch.BatchKey)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("造暂存目录失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "0001.jpg"), []byte("x"), 0o644); err != nil {
			t.Fatalf("造暂存文件失败: %v", err)
		}
		item := model.HistoryItem{BatchID: batch.ID, SourcePath: filepath.Join(dir, "s.png"), SourceName: "s.png", Status: string(StatusSuccess), StagingName: "0001.jpg"}
		if err := svc.db.Create(&item).Error; err != nil {
			t.Fatalf("造明细失败: %v", err)
		}
	}
	if err := svc.PruneHistory(); err != nil {
		t.Fatalf("PruneHistory 失败: %v", err)
	}
	var total, purged int64
	svc.db.Model(&model.HistoryBatch{}).Count(&total)
	svc.db.Model(&model.HistoryBatch{}).Where("files_purged = ?", true).Count(&purged)
	if total != historyKeepBatches+2 || purged != 2 {
		t.Fatalf("修剪后 total=%d purged=%d，期望 %d/2", total, purged, historyKeepBatches+2)
	}
	var purgedBatch model.HistoryBatch
	if err := svc.db.Where("files_purged = ?", true).First(&purgedBatch).Error; err != nil {
		t.Fatalf("找不到被清理批次: %v", err)
	}
	if _, err := os.Stat(svc.stagingDir(purgedBatch.BatchKey)); !os.IsNotExist(err) {
		t.Errorf("被清理批次的暂存目录应已删除")
	}
	var clearedItems int64
	svc.db.Model(&model.HistoryItem{}).Where("batch_id = ? AND staging_name = ''", purgedBatch.ID).Count(&clearedItems)
	if clearedItems != 1 {
		t.Errorf("被清理批次的明细应清空暂存名：%d", clearedItems)
	}
}

// TestClearHistoryAndUsage US-64：清空删光记录与暂存；占用统计可用。
func TestClearHistoryAndUsage(t *testing.T) {
	svc := newHistoryTestService(t)
	srcDir := t.TempDir()
	_ = runTestBatch(t, svc, srcDir)

	usage, err := svc.HistoryUsage()
	if err != nil || usage <= 0 {
		t.Fatalf("占用统计不对：%d %v", usage, err)
	}
	if err := svc.ClearHistory(); err != nil {
		t.Fatalf("ClearHistory 失败: %v", err)
	}
	var batches, items int64
	svc.db.Model(&model.HistoryBatch{}).Count(&batches)
	svc.db.Model(&model.HistoryItem{}).Count(&items)
	if batches != 0 || items != 0 {
		t.Fatalf("清空后仍有记录：%d/%d", batches, items)
	}
	usage, err = svc.HistoryUsage()
	if err != nil || usage != 0 {
		t.Errorf("清空后占用应为 0：%d %v", usage, err)
	}
}

// TestListHistoryBatchesAndItems US-62/63：列表最近在前、按批次键查明细。
func TestListHistoryBatchesAndItems(t *testing.T) {
	svc := newHistoryTestService(t)
	srcDir := t.TempDir()
	first := runTestBatch(t, svc, srcDir)
	second := runTestBatch(t, svc, srcDir)

	batches, err := svc.ListHistoryBatches(page.Request{})
	if err != nil {
		t.Fatalf("ListHistoryBatches 失败: %v", err)
	}
	if batches.Total != 2 || batches.List[0].BatchKey == first.BatchKey || batches.List[0].BatchKey != second.BatchKey {
		t.Fatalf("列表应最近在前：%+v", batches.List)
	}
	items, err := svc.ListHistoryItems(second.BatchKey, page.Request{})
	if err != nil || items.Total != 2 {
		t.Fatalf("明细不对：%d %v", items.Total, err)
	}
	if _, err := svc.ListHistoryItems("no-such-key", page.Request{}); err == nil {
		t.Error("未知批次键应报错")
	}
}
