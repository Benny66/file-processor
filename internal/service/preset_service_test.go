package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"file-processor/internal/apperr"
	"file-processor/internal/config"
	"file-processor/internal/model"
	"file-processor/internal/page"
)

func newPresetTestService(t *testing.T) *Service {
	t.Helper()
	isolateHome(t)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "preset.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.ImagePreset{}); err != nil {
		t.Fatalf("迁移预设表失败: %v", err)
	}
	cfg, err := config.Load("presettest")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	return New(db, cfg)
}

func requireValidation(t *testing.T, err error) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
		t.Fatalf("期望 validation 错误，实际：%v", err)
	}
}

func TestPresetCRUDTrimsAndKeepsNamesUnique(t *testing.T) {
	svc := newPresetTestService(t)

	created, err := svc.SavePreset(model.ImagePreset{
		Name:         "  海报  ",
		OutputFormat: "jpeg",
		TargetBytes:  300 * 1024,
		MaxWidth:     1600,
		MaxHeight:    1200,
	})
	if err != nil {
		t.Fatalf("SavePreset 失败: %v", err)
	}
	if created.ID == 0 || created.Name != "海报" {
		t.Fatalf("预设未正确创建：%+v", created)
	}

	result, err := svc.ListPresets(page.Request{})
	if err != nil {
		t.Fatalf("ListPresets 失败: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].Name != "海报" {
		t.Fatalf("列表结果不对：%+v", result)
	}

	if _, err := svc.SavePreset(model.ImagePreset{
		Name:         " 海报 ",
		OutputFormat: "png",
	}); err == nil {
		t.Fatal("去空白后重名应被拒绝")
	} else {
		requireValidation(t, err)
	}

	created.Name = "海报 2"
	created.OutputFormat = "webp"
	created.TargetBytes = 512 * 1024
	renamed, err := svc.SavePreset(created)
	if err != nil {
		t.Fatalf("重命名失败: %v", err)
	}
	if renamed.ID != created.ID || renamed.Name != "海报 2" || renamed.OutputFormat != "webp" {
		t.Errorf("重命名结果不对：%+v", renamed)
	}

	// 重命名后旧名称立即释放。
	if _, err := svc.SavePreset(model.ImagePreset{Name: "海报", OutputFormat: "keep"}); err != nil {
		t.Fatalf("重命名后旧名称应可复用: %v", err)
	}
	if _, err := svc.SavePreset(model.ImagePreset{Name: "海报 2", OutputFormat: "keep"}); err == nil {
		t.Fatal("已占用的名称再次创建应被拒绝")
	} else {
		requireValidation(t, err)
	}

	if err := svc.DeletePreset(created.ID); err != nil {
		t.Fatalf("删除预设失败: %v", err)
	}
	result, err = svc.ListPresets(page.Request{})
	if err != nil {
		t.Fatalf("删除后列表失败: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].Name != "海报" {
		t.Errorf("删除后应只剩「海报」：%+v", result)
	}

	// 硬删除才能让名称真正释放；软删除会因唯一索引永久占用该名称。
	if _, err := svc.SavePreset(model.ImagePreset{Name: "海报 2", OutputFormat: "keep"}); err != nil {
		t.Fatalf("删除后应能复用名称: %v", err)
	}
}

func TestPresetPersistsAcrossReopen(t *testing.T) {
	isolateHome(t)
	dbPath := filepath.Join(t.TempDir(), "preset.db")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			t.Fatalf("打开测试库失败: %v", err)
		}
		if err := db.AutoMigrate(&model.ImagePreset{}); err != nil {
			t.Fatalf("迁移预设表失败: %v", err)
		}
		return db
	}
	cfg, err := config.Load("presettest")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}

	first := open()
	if _, err := New(first, cfg).SavePreset(model.ImagePreset{
		Name:         "持久化",
		OutputFormat: "webp",
		TargetBytes:  200 * 1024,
		MaxWidth:     800,
		MaxHeight:    600,
	}); err != nil {
		t.Fatalf("保存预设失败: %v", err)
	}
	sqlDB, err := first.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("关闭测试库失败: %v", err)
	}

	result, err := New(open(), cfg).ListPresets(page.Request{})
	if err != nil {
		t.Fatalf("重开数据库后列表失败: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].Name != "持久化" {
		t.Errorf("重开数据库后预设丢失：%+v", result)
	}
}

func TestPresetListPagination(t *testing.T) {
	svc := newPresetTestService(t)
	for _, name := range []string{"A", "B", "C"} {
		if _, err := svc.SavePreset(model.ImagePreset{Name: name, OutputFormat: "keep"}); err != nil {
			t.Fatalf("创建 %s 失败: %v", name, err)
		}
	}

	result, err := svc.ListPresets(page.Request{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("ListPresets 失败: %v", err)
	}
	if result.Total != 3 || len(result.List) != 1 || result.List[0].Name != "C" {
		t.Errorf("分页结果不对：%+v", result)
	}
	if result.Page != 2 || result.PageSize != 2 {
		t.Errorf("分页回显不对：page=%d page_size=%d", result.Page, result.PageSize)
	}
}

func TestPresetRejectsInvalidRules(t *testing.T) {
	svc := newPresetTestService(t)
	cases := []struct {
		name   string
		preset model.ImagePreset
	}{
		{"空名称", model.ImagePreset{Name: "  ", OutputFormat: "keep"}},
		{"非法格式", model.ImagePreset{Name: "格式", OutputFormat: "gif"}},
		{"目标过小", model.ImagePreset{Name: "过小", OutputFormat: "keep", TargetBytes: 512}},
		{"目标非整数 KB", model.ImagePreset{Name: "非整", OutputFormat: "keep", TargetBytes: 1500}},
		{"目标过大", model.ImagePreset{Name: "过大", OutputFormat: "keep", TargetBytes: TargetBytesMax + 1}},
		{"只设宽度", model.ImagePreset{Name: "只宽", OutputFormat: "keep", MaxWidth: 100}},
		{"尺寸越界", model.ImagePreset{Name: "越界", OutputFormat: "keep", MaxWidth: 70000, MaxHeight: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SavePreset(tc.preset)
			if err == nil {
				t.Fatal("非法预设未被拒绝")
			}
			requireValidation(t, err)
		})
	}
}

func TestPresetNotFound(t *testing.T) {
	svc := newPresetTestService(t)

	_, err := svc.SavePreset(model.ImagePreset{
		BaseModel:    model.BaseModel{ID: 999},
		Name:         "不存在",
		OutputFormat: "keep",
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFound {
		t.Fatalf("更新不存在的预设应返回 not_found：%v", err)
	}

	if err := svc.DeletePreset(999); err == nil {
		t.Fatal("删除不存在的预设应失败")
	} else if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFound {
		t.Fatalf("删除不存在的预设应返回 not_found：%v", err)
	}

	requireValidation(t, svc.DeletePreset(0))
}

func TestImageSettingsRoundTrip(t *testing.T) {
	svc := newPresetTestService(t)

	defaults, err := svc.GetImageSettings()
	if err != nil {
		t.Fatalf("GetImageSettings 失败: %v", err)
	}
	if defaults.OutputFormat != "keep" || defaults.TargetBytes != 0 ||
		defaults.MaxWidth != 0 || defaults.MaxHeight != 0 {
		t.Fatalf("默认设置不对：%+v", defaults)
	}

	want := BatchRules{OutputFormat: "webp", TargetBytes: 512 * 1024, MaxWidth: 1920, MaxHeight: 1080}
	if err := svc.SaveImageSettings(want); err != nil {
		t.Fatalf("SaveImageSettings 失败: %v", err)
	}

	// 重新从磁盘加载配置，证明跨重启可恢复。
	cfg, err := config.Load("presettest")
	if err != nil {
		t.Fatalf("重新加载配置失败: %v", err)
	}
	reloaded := New(nil, cfg)
	got, err := reloaded.GetImageSettings()
	if err != nil {
		t.Fatalf("重新加载设置失败: %v", err)
	}
	if got != want {
		t.Errorf("设置的往返结果 = %+v，期望 %+v", got, want)
	}

	invalid := BatchRules{OutputFormat: "jpeg", TargetBytes: 1500}
	requireValidation(t, svc.SaveImageSettings(invalid))
	got, err = svc.GetImageSettings()
	if err != nil {
		t.Fatalf("非法保存后 Get 失败: %v", err)
	}
	if got != want {
		t.Errorf("非法保存不应覆盖旧设置：%+v", got)
	}
}
