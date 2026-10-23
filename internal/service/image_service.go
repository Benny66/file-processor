package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

// ScannedFile 是扫描阶段确认的图片文件，不包含解码结果。
type ScannedFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ScanRejection 是扫描阶段被跳过的路径及原因。
type ScanRejection struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// ScanResult 是一次扫描的结果：可入队的文件与带原因的跳过项。
type ScanResult struct {
	Files    []ScannedFile   `json:"files"`
	Rejected []ScanRejection `json:"rejected"`
}

// ScanPaths 扫描用户选择的文件与文件夹；skip 是队列中已存在的绝对路径。
//
// 文件夹递归扫描只收支持的图片扩展名，非图片静默忽略；隐藏文件/目录不扫描，
// 符号链接不跟随。直接选择的文件按用户意图处理，不做隐藏过滤。
func (s *Service) ScanPaths(paths, skip []string) (ScanResult, error) {
	result := ScanResult{Files: []ScannedFile{}, Rejected: []ScanRejection{}}

	skipped := make(map[string]struct{}, len(skip))
	for _, path := range skip {
		if abs, ok := absoluteCleanPath(path); ok {
			skipped[abs] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	for _, raw := range paths {
		abs, ok := absoluteCleanPath(raw)
		if !ok {
			result.Rejected = append(result.Rejected, ScanRejection{Path: raw, Reason: "路径无效"})
			continue
		}
		info, err := os.Lstat(abs)
		if err != nil {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "无法访问"})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "符号链接已跳过"})
			continue
		}
		if info.IsDir() {
			scanImageDirectory(abs, skipped, seen, &result)
			continue
		}
		if !info.Mode().IsRegular() {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "不是普通文件"})
			continue
		}
		if !imageproc.IsSupportedPath(abs) {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "不支持的格式"})
			continue
		}
		if _, ok := skipped[abs]; ok {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "已在队列中"})
			continue
		}
		if _, ok := seen[abs]; ok {
			result.Rejected = append(result.Rejected, ScanRejection{Path: abs, Reason: "重复路径"})
			continue
		}
		seen[abs] = struct{}{}
		result.Files = append(result.Files, ScannedFile{Path: abs, Name: info.Name(), Size: info.Size()})
	}

	sort.Slice(result.Files, func(i, j int) bool {
		return result.Files[i].Path < result.Files[j].Path
	})
	result.Rejected = uniqueScanRejections(result.Rejected)
	sort.Slice(result.Rejected, func(i, j int) bool {
		if result.Rejected[i].Path == result.Rejected[j].Path {
			return result.Rejected[i].Reason < result.Rejected[j].Reason
		}
		return result.Rejected[i].Path < result.Rejected[j].Path
	})
	return result, nil
}

// InspectImage 读取图片元信息与缩略图；文件 I/O 在 service 层完成，
// imageproc 只处理字节。失败返回可读的 apperr。
func (s *Service) InspectImage(path string) (imageproc.Info, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return imageproc.Info{}, apperr.Validation("图片路径无效")
	}
	abs = filepath.Clean(abs)

	stat, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return imageproc.Info{}, apperr.NotFound("图片不存在")
		}
		return imageproc.Info{}, apperr.Validation("无法读取图片")
	}
	if !stat.Mode().IsRegular() {
		return imageproc.Info{}, apperr.Validation("不是普通文件")
	}
	if stat.Size() > imageproc.MaxFileBytes {
		return imageproc.Info{}, apperr.Validation("图片超过 512 MB 上限")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return imageproc.Info{}, apperr.Validation("无法读取图片")
	}
	return imageproc.Inspect(data, abs)
}

// scanImageDirectory 递归收集目录下的图片；隐藏目录、符号链接与普通非图片静默跳过。
func scanImageDirectory(root string, skipped, seen map[string]struct{}, result *ScanResult) {
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Rejected = append(result.Rejected, ScanRejection{Path: path, Reason: "无法访问"})
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			result.Rejected = append(result.Rejected, ScanRejection{Path: path, Reason: "符号链接已跳过"})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() || !imageproc.IsSupportedPath(path) {
			return nil
		}
		if _, ok := skipped[path]; ok {
			result.Rejected = append(result.Rejected, ScanRejection{Path: path, Reason: "已在队列中"})
			return nil
		}
		if _, ok := seen[path]; ok {
			result.Rejected = append(result.Rejected, ScanRejection{Path: path, Reason: "重复路径"})
			return nil
		}
		info, err := d.Info()
		if err != nil {
			result.Rejected = append(result.Rejected, ScanRejection{Path: path, Reason: "无法访问"})
			return nil
		}
		seen[path] = struct{}{}
		result.Files = append(result.Files, ScannedFile{Path: path, Name: d.Name(), Size: info.Size()})
		return nil
	}); err != nil {
		result.Rejected = append(result.Rejected, ScanRejection{Path: root, Reason: "扫描目录失败"})
	}
}

func absoluteCleanPath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func uniqueScanRejections(list []ScanRejection) []ScanRejection {
	if len(list) == 0 {
		return list
	}
	seen := make(map[ScanRejection]struct{}, len(list))
	out := list[:0]
	for _, item := range list {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
