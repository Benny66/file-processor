package service

// 输出目的地与命名模板（US-39~42 / D-34~37）的单元测试。
// 路径解析、模板渲染、防覆盖回退、扫描相对路径与设置校验都在这里覆盖。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

func TestValidateOutputPlan(t *testing.T) {
	cases := []struct {
		name    string
		plan    OutputPlan
		wantErr bool
	}{
		{"空模式按 source", OutputPlan{}, false},
		{"显式 source", OutputPlan{Mode: OutputModeSource}, false},
		{"flat 绝对目录", OutputPlan{Mode: OutputModeCustomFlat, Directory: "/tmp/out"}, false},
		{"tree 绝对目录+模板", OutputPlan{Mode: OutputModeCustomTree, Directory: "/tmp/out", Pattern: "{n}"}, false},
		{"模式取值非法", OutputPlan{Mode: "weird"}, true},
		{"flat 缺目录", OutputPlan{Mode: OutputModeCustomFlat}, true},
		{"tree 相对目录", OutputPlan{Mode: OutputModeCustomTree, Directory: "out"}, true},
		{"模板缺变量", OutputPlan{Mode: OutputModeCustomFlat, Directory: "/tmp/out", Pattern: "photo"}, true},
		{"模板含分隔符", OutputPlan{Mode: OutputModeCustomFlat, Directory: "/tmp/out", Pattern: "a/{name}"}, true},
		{"模板含 ..", OutputPlan{Mode: OutputModeCustomFlat, Directory: "/tmp/out", Pattern: "{name}.."}, true},
		{"模板超长", OutputPlan{Mode: OutputModeCustomFlat, Directory: "/tmp/out", Pattern: strings.Repeat("a", 60) + "{name}"}, true},
		{"source 模式忽略目录", OutputPlan{Mode: OutputModeSource, Directory: "relative"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOutputPlan(tc.plan)
			if tc.wantErr {
				var ae *apperr.Error
				if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
					t.Fatalf("期望 validation 错误，实际：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("合法计划被拒：%v", err)
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
		{"", "photo", 3, "photo-out"},           // 空模板 → 默认 {name}-out
		{"{name}-web", "photo", 1, "photo-web"}, // 前后缀
		{"图-{n}", "photo", 7, "图-7"},            // 队列序号
		{"{n}-{name}", "相册/a", 2, "2-相册/a"},     // 注意 stem 本身来自文件名，不含分隔符；此用例只验证替换顺序
		{"{ext}", "photo", 1, "{ext}"},          // 未知占位符原样保留
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

func TestProcessBatchCustomTreePreservesStructure(t *testing.T) {
	srcRoot := t.TempDir()
	sub := filepath.Join(srcRoot, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建子目录失败: %v", err)
	}
	deep := filepath.Join(sub, "deep.png")
	top := filepath.Join(srcRoot, "top.png")
	if err := os.WriteFile(deep, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(top, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "归档输出")
	svc := &Service{}
	result, err := svc.ProcessBatch(BatchRequest{
		Items: []BatchItem{
			{Path: deep, RelPath: filepath.Join("sub", "deep.png")},
			{Path: top, RelPath: "top.png"},
		},
		Rules:  BatchRules{OutputFormat: imageproc.OutputJPEG},
		Output: OutputPlan{Mode: OutputModeCustomTree, Directory: outDir},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	wantDeep := filepath.Join(outDir, "sub", "deep-out.jpg")
	wantTop := filepath.Join(outDir, "top-out.jpg")
	if result.Items[0].OutputPath != wantDeep || result.Items[1].OutputPath != wantTop {
		t.Fatalf("输出路径不对：%q / %q，期望 %q / %q",
			result.Items[0].OutputPath, result.Items[1].OutputPath, wantDeep, wantTop)
	}
	for _, path := range []string{wantDeep, wantTop} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("输出不存在: %v", err)
		}
	}
	// 源目录保持干净：不因为默认行为残留 -out 产物。
	if _, err := os.Stat(filepath.Join(srcRoot, "top-out.jpg")); !os.IsNotExist(err) {
		t.Errorf("源目录不应出现输出文件")
	}
}

func TestProcessBatchCustomFlatPatternAndCollision(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	if err := os.WriteFile(a, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(b, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	outDir := t.TempDir()
	// 预占第一个候选名：命名模板下防覆盖序号同样生效。
	if err := os.WriteFile(filepath.Join(outDir, "img-1.jpg"), []byte("old"), 0o644); err != nil {
		t.Fatalf("预占候选名失败: %v", err)
	}

	svc := &Service{}
	result, err := svc.ProcessBatch(BatchRequest{
		Items:  batchItems(a, b),
		Rules:  BatchRules{OutputFormat: imageproc.OutputJPEG},
		Output: OutputPlan{Mode: OutputModeCustomFlat, Directory: outDir, Pattern: "img-{n}"},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	if got := result.Items[0].OutputPath; got != filepath.Join(outDir, "img-1-2.jpg") {
		t.Errorf("冲突命名 = %q，期望 img-1-2.jpg", got)
	}
	if got := result.Items[1].OutputPath; got != filepath.Join(outDir, "img-2.jpg") {
		t.Errorf("序号命名 = %q，期望 img-2.jpg", got)
	}
	occupied, err := os.ReadFile(filepath.Join(outDir, "img-1.jpg"))
	if err != nil || string(occupied) != "old" {
		t.Errorf("已存在的文件被覆盖：%q", string(occupied))
	}
}

// TestProcessBatchCustomTreeRejectsTraversal 相对路径来自绑定入参，不可信：
// 携带 .. 的 RelPath 必须按单项失败处理，而不是把输出写到指定目录之外。
func TestProcessBatchCustomTreeRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.png")
	if err := os.WriteFile(src, batchTestPNG(t, 20, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	root := t.TempDir()
	outDir := filepath.Join(root, "out")

	svc := &Service{}
	result, err := svc.ProcessBatch(BatchRequest{
		Items:  []BatchItem{{Path: src, RelPath: filepath.Join("..", "escape.png")}},
		Rules:  BatchRules{OutputFormat: imageproc.OutputJPEG},
		Output: OutputPlan{Mode: OutputModeCustomTree, Directory: outDir},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 不应整体失败: %v", err)
	}
	if result.Items[0].Status != StatusFailed || !strings.Contains(result.Items[0].Message, "非法") {
		t.Fatalf("穿越相对路径应单项失败：%+v", result.Items[0])
	}
	// root 下只应剩下空的 out 目录本身——穿越目标 escape.png 不应出现在输出目录之外。
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("读取输出根目录失败: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "out" {
			t.Errorf("输出目录之外出现文件：%s", entry.Name())
		}
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
