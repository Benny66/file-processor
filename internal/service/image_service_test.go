package service

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

func writeScanTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
}

func scanRejectionByPath(result ScanResult, path string) (ScanRejection, bool) {
	for _, r := range result.Rejected {
		if r.Path == path {
			return r, true
		}
	}
	return ScanRejection{}, false
}

func TestScanPathsRecursesAndFilters(t *testing.T) {
	root := t.TempDir()
	writeScanTestFile(t, filepath.Join(root, "a.jpg"), "a")
	writeScanTestFile(t, filepath.Join(root, "sub", "b.PNG"), "bb")
	writeScanTestFile(t, filepath.Join(root, "sub", "note.txt"), "x")
	writeScanTestFile(t, filepath.Join(root, "sub", "nested", "e.webp"), "eee")
	writeScanTestFile(t, filepath.Join(root, ".hidden.jpg"), "h")
	writeScanTestFile(t, filepath.Join(root, ".hidden", "d.png"), "h")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{root}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}

	want := []string{
		filepath.Join(root, "a.jpg"),
		filepath.Join(root, "sub", "b.PNG"),
		filepath.Join(root, "sub", "nested", "e.webp"),
	}
	if len(result.Files) != len(want) {
		t.Fatalf("扫描到 %d 个文件，期望 %d：%+v", len(result.Files), len(want), result.Files)
	}
	for i, path := range want {
		if result.Files[i].Path != path {
			t.Errorf("files[%d].Path = %q，期望 %q", i, result.Files[i].Path, path)
		}
	}
	if len(result.Rejected) != 0 {
		t.Errorf("文件夹内的非图片/隐藏项不应进入 rejected：%+v", result.Rejected)
	}
}

func TestScanPathsDirectFilesAndRejections(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "a.jpg")
	note := filepath.Join(root, "note.txt")
	missing := filepath.Join(root, "missing.jpg")
	writeScanTestFile(t, image, "abc")
	writeScanTestFile(t, note, "x")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{note, missing, image}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != image {
		t.Fatalf("直接选择的文件结果不对：%+v", result.Files)
	}
	if result.Files[0].Name != "a.jpg" || result.Files[0].Size != 3 {
		t.Errorf("文件元信息不对：%+v", result.Files[0])
	}
	if r, ok := scanRejectionByPath(result, note); !ok || r.Reason != "不支持的格式" {
		t.Errorf("不支持的格式未正确标记：%+v", result.Rejected)
	}
	if r, ok := scanRejectionByPath(result, missing); !ok || r.Reason != "无法访问" {
		t.Errorf("不存在的路径未正确标记：%+v", result.Rejected)
	}
}

func TestScanPathsDeduplicatesAndSkipsQueued(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "a.jpg")
	writeScanTestFile(t, image, "a")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{root, image}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != image {
		t.Fatalf("同一路径应只出现一次：%+v", result.Files)
	}
	if r, ok := scanRejectionByPath(result, image); !ok || r.Reason != "重复路径" {
		t.Errorf("扫描内重复未标记：%+v", result.Rejected)
	}

	result, err = svc.ScanPaths([]string{image}, []string{image})
	if err != nil {
		t.Fatalf("ScanPaths(skip) 失败: %v", err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("已在队列中的文件不应再次入队：%+v", result.Files)
	}
	if r, ok := scanRejectionByPath(result, image); !ok || r.Reason != "已在队列中" {
		t.Errorf("队列重复未标记：%+v", result.Rejected)
	}
}

func TestScanPathsSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a.jpg")
	link := filepath.Join(root, "link.jpg")
	writeScanTestFile(t, target, "a")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("当前环境不支持符号链接: %v", err)
	}

	svc := &Service{}
	result, err := svc.ScanPaths([]string{link, root}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != target {
		t.Fatalf("符号链接不应入队，目标文件应保留：%+v", result.Files)
	}
	if r, ok := scanRejectionByPath(result, link); !ok || r.Reason != "符号链接已跳过" {
		t.Errorf("符号链接未正确标记：%+v", result.Rejected)
	}
	if len(result.Rejected) != 1 {
		t.Errorf("同一符号链接应去重为 1 条拒绝记录，实际 %d：%+v", len(result.Rejected), result.Rejected)
	}
}

func TestScanPathsSortsAndPreservesSizes(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.jpg")
	b := filepath.Join(root, "b.jpg")
	c := filepath.Join(root, "c.png")
	writeScanTestFile(t, b, "bb")
	writeScanTestFile(t, c, "ccc")
	writeScanTestFile(t, a, "a")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{b, c, a}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	want := []struct {
		path string
		size int64
	}{{a, 1}, {b, 2}, {c, 3}}
	if len(result.Files) != len(want) {
		t.Fatalf("文件数 = %d，期望 %d", len(result.Files), len(want))
	}
	for i, w := range want {
		if result.Files[i].Path != w.path || result.Files[i].Size != w.size {
			t.Errorf("files[%d] = %+v，期望 path=%q size=%d", i, result.Files[i], w.path, w.size)
		}
	}
}

func TestScanPathsChineseAndSpaces(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "图 片")
	image := filepath.Join(dir, "照 片.jpg")
	writeScanTestFile(t, image, "hello")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{dir}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("文件数 = %d，期望 1：%+v", len(result.Files), result.Files)
	}
	if result.Files[0].Path != image || result.Files[0].Name != "照 片.jpg" || result.Files[0].Size != 5 {
		t.Errorf("中文/空格路径结果不对：%+v", result.Files[0])
	}
}

func TestScanPathsAllowsDirectHiddenFile(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, ".direct.jpg")
	writeScanTestFile(t, image, "a")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{image}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != image {
		t.Errorf("用户直接选择的隐藏文件应允许入队：%+v", result.Files)
	}
}

func TestScanPathsEmptyInput(t *testing.T) {
	svc := &Service{}
	result, err := svc.ScanPaths(nil, nil)
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if len(result.Files) != 0 || len(result.Rejected) != 0 {
		t.Errorf("空输入结果应为空：%+v", result)
	}
}

func TestScanPathsDoesNotTrimPathWhitespace(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "a.jpg")
	writeScanTestFile(t, imagePath, "a")

	svc := &Service{}
	result, err := svc.ScanPaths([]string{" " + imagePath}, nil)
	if err != nil {
		t.Fatalf("ScanPaths 失败: %v", err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("带前导空格的路径不应命中真实文件：%+v", result.Files)
	}
	if len(result.Rejected) != 1 || result.Rejected[0].Reason != "无法访问" {
		t.Fatalf("期望 1 条无法访问，实际：%+v", result.Rejected)
	}
}

func TestInspectImageReadsFileAndBuildsThumbnail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "图 片.png")
	img := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("写入图片失败: %v", err)
	}

	svc := &Service{}
	info, err := svc.InspectImage(path)
	if err != nil {
		t.Fatalf("InspectImage 失败: %v", err)
	}
	if info.Path != path || info.Format != "png" || info.Width != 20 || info.Height != 10 {
		t.Errorf("检查结果不对：%+v", info)
	}
	if info.Thumbnail == "" {
		t.Errorf("缩略图为空")
	}
}

func TestInspectImageRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建稀疏文件失败: %v", err)
	}
	if err := f.Truncate(imageproc.MaxFileBytes + 1); err != nil {
		f.Close()
		t.Fatalf("截断文件失败: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("关闭文件失败: %v", err)
	}

	svc := &Service{}
	_, err = svc.InspectImage(path)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
		t.Fatalf("期望 validation 错误，实际：%v", err)
	}
	if !bytes.Contains([]byte(ae.Error()), []byte("512 MB")) {
		t.Errorf("错误未说明 512 MB 上限：%q", ae.Error())
	}
}

func TestInspectImageMissingFile(t *testing.T) {
	svc := &Service{}
	_, err := svc.InspectImage(filepath.Join(t.TempDir(), "missing.png"))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFound {
		t.Fatalf("期望 not_found 错误，实际：%v", err)
	}
}
