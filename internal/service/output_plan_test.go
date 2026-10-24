package service

// 输出命名与下载相关（US-58~64 / D-54~58）的单元测试。
// 「输出目的地」三模式已在 ADR-0007 中迁移为下载选项；命名渲染/防覆盖/层级/穿越
// 防护等机制全部保留，测试也随之迁到下载层（见 history_test.go）。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

func TestValidatePattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"空串按默认", "", false},
		{"默认格式", "{name}-out", false},
		{"序号格式", "img-{n}", false},
		{"缺变量", "photo", true},
		{"含分隔符", "a/{name}", true},
		{"含 ..", "{name}..", true},
		{"超长", strings.Repeat("a", 60) + "{name}", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePattern(tc.pattern)
			if tc.wantErr {
				var ae *apperr.Error
				if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
					t.Fatalf("期望 validation 错误，实际：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("合法格式被拒：%v", err)
			}
		})
	}
}

func TestRenderOutputStem(t *testing.T) {
	cases := []struct {
		pattern, stem string
		index         int
		want          string
	}{
		{"", "photo", 3, "photo-out"}, // 空模板 → 默认 {name}-out
		{"{name}-web", "photo", 1, "photo-web"},
		{"图-{n}", "photo", 7, "图-7"},   // 清单序号
		{"{ext}", "photo", 1, "{ext}"}, // 未知占位符原样保留
	}
	for _, tc := range cases {
		if got := renderOutputStem(tc.pattern, tc.stem, tc.index); got != tc.want {
			t.Errorf("render(%q, %q, %d) = %q，期望 %q", tc.pattern, tc.stem, tc.index, got, tc.want)
		}
	}
}

func TestScanPathsRecordsRelPath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建子目录失败: %v", err)
	}
	inside := filepath.Join(sub, "a.png")
	top := filepath.Join(root, "b.png")
	if err := os.WriteFile(inside, batchTestPNG(t, 4, 4), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(top, batchTestPNG(t, 4, 4), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	svc := &Service{}
	result, err := svc.ScanPaths([]string{root, top}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	rels := map[string]string{}
	for _, file := range result.Files {
		rels[file.Path] = file.RelPath
	}
	if rels[inside] != filepath.Join("sub", "a.png") {
		t.Errorf("目录导入的相对路径 = %q，期望 sub/a.png", rels[inside])
	}
	if rels[top] != "b.png" {
		t.Errorf("直接选文件的相对路径 = %q，期望 b.png", rels[top])
	}
}

// TestProcessBatchSkippedProducesNothing 「无需处理」语义不变：不产出文件、不改源文件。
func TestProcessBatchSkippedProducesNothing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.png")
	if err := os.WriteFile(src, batchTestPNG(t, 100, 50), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
			Rules: BatchRules{OutputFormat: imageproc.OutputKeep, TargetBytes: 1024 * 1024},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	item := result.Items[0]
	if item.Status != StatusSkipped || item.Message != "无需处理（原图已经合适）" || item.OutputPath != "" {
		t.Fatalf("应无需处理：%+v", item)
	}
	if _, err := os.Stat(filepath.Join(dir, "small-out.png")); !os.IsNotExist(err) {
		t.Errorf("无需处理的项不应产出文件")
	}
}

// TestProcessBatchSubsetReturnsOnlyRequested 队列内「再试一次」（US-43）依赖的 Go 侧契约：
// 批次接口无状态，只返回请求中列出的项——重试即对它做子集调用。
func TestProcessBatchSubsetReturnsOnlyRequested(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	if err := os.WriteFile(a, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(b, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(b), // 队列里有两张，批次只请求 b
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Path != b {
		t.Fatalf("子集请求应只返回请求项：%+v", result.Items)
	}
	if _, err := os.Stat(filepath.Join(dir, "a-out.jpg")); !os.IsNotExist(err) {
		t.Errorf("未被请求的文件不应被处理")
	}
}

// TestRenderOutputSample 命名示例与真实落盘同一套实现（US-41/60）。
func TestRenderOutputSample(t *testing.T) {
	svc := &Service{}
	cases := []struct {
		name, pattern, sample, format, want string
	}{
		{"默认格式", "", "photo.jpg", imageproc.OutputJPEG, "photo-out.jpg"},
		{"序号格式", "img-{n}", "photo.jpg", imageproc.OutputPNG, "img-1.png"},
		{"keep 沿用扩展名", "{name}-web", "Shot.PNG", imageproc.OutputKeep, "Shot-web.png"},
		{"auto 占位", "{name}-out", "photo.jpg", imageproc.OutputAuto, "photo-out.<自动>"},
	}
	for _, tc := range cases {
		got, err := svc.RenderOutputSample(tc.pattern, tc.sample, tc.format)
		if err != nil {
			t.Errorf("%s：不应失败：%v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s：示例 = %q，期望 %q", tc.name, got, tc.want)
		}
	}
	if _, err := svc.RenderOutputSample("photo", "photo.jpg", imageproc.OutputJPEG); err == nil {
		t.Error("缺变量的格式应报错")
	}
	if _, err := svc.RenderOutputSample("", "photo.jpg", "gif"); err == nil {
		t.Error("非法格式应报错")
	}
}

// TestPlaceOutputFallsBackWhenLinkUnavailable D-37：链接不可用（网络盘等）时
// 退化为独占创建+复制，防覆盖与序号递增语义不变。
func TestPlaceOutputFallsBackWhenLinkUnavailable(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, ".tmp-src")
	if err := os.WriteFile(temp, []byte("data"), 0o644); err != nil {
		t.Fatalf("写临时文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photo-out.jpg"), []byte("old"), 0o644); err != nil {
		t.Fatalf("预占候选名失败: %v", err)
	}

	prev := linkOutputFile
	linkOutputFile = func(_, _ string) error { return errors.New("link not supported") }
	t.Cleanup(func() { linkOutputFile = prev })

	got, err := placeOutputFile(temp, dir, "photo-out", ".jpg")
	if err != nil {
		t.Fatalf("placeOutputFile 失败: %v", err)
	}
	if got != filepath.Join(dir, "photo-out-2.jpg") {
		t.Errorf("回退路径的冲突命名 = %q，期望 photo-out-2.jpg", got)
	}
	copied, err := os.ReadFile(got)
	if err != nil || string(copied) != "data" {
		t.Errorf("复制内容不对：%q", string(copied))
	}
	occupied, err := os.ReadFile(filepath.Join(dir, "photo-out.jpg"))
	if err != nil || string(occupied) != "old" {
		t.Errorf("已存在的文件被覆盖：%q", string(occupied))
	}
}
