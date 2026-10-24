package guard

// ---------------------------------------------------------------------------
// 护栏：图标派生一致性（icon.manifest 哈希校验，ADR-0008）
//
// 图标资产是「SVG 源（唯一真相）→ make icon 派生 → 派生物入库」的管线，
// CI 不重新栅格化。漂移不会报错、不会崩溃，只是某处图标悄悄变旧——
// 典型的「无症状失效」（AGENTS.md 铁律 5），故编译成本检查：
//   1. 改了 SVG 源没跑 make icon → manifest 里源的哈希对不上；
//   2. 手改了派生物 → 派生物哈希对不上（派生物不许手改，改源再派生）；
//   3. 管线产物清单变了没同步护栏 → 期望清单缺项。
// ---------------------------------------------------------------------------

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// iconManifest 是 build/icon.manifest 的结构（由 frontend/scripts/icon.mjs 生成）。
type iconManifest struct {
	Sources map[string]string `json:"sources"`
	Derived map[string]string `json:"derived"`
}

// expectedIconDerived 是管线必须产出的派生物清单（相对仓库根，/ 分隔）。
// 新增派生物时同步更新此清单与 icon.mjs。
var expectedIconDerived = []string{
	"build/appicon.png",
	"build/windows/icon.ico",
	"build/tray.png",
	"build/darwin/dmg-background.tpl.png",
}

func TestIconArtifactsMatchManifest(t *testing.T) {
	root := projectRoot()

	raw, err := os.ReadFile(filepath.Join(root, "build", "icon.manifest"))
	if err != nil {
		t.Fatalf("build/icon.manifest 不存在：图标派生登记表缺失，先跑 make icon：%v", err)
	}
	var m iconManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("build/icon.manifest 不是合法 JSON：%v", err)
	}

	// 铁律 5：解析到 0 个结果必须报错，而非当作通过静默放行。
	if len(m.Sources) == 0 || len(m.Derived) == 0 {
		t.Fatal("icon.manifest 解析到 0 条源或 0 条派生物——manifest 写法可能已变更，" +
			"请同步更新本护栏与 frontend/scripts/icon.mjs 的生成格式")
	}

	for _, p := range expectedIconDerived {
		if _, ok := m.Derived[p]; !ok {
			t.Errorf("图标派生物 %s 不在 icon.manifest 中：管线产物清单与护栏期望不一致，"+
				"请跑 make icon 并核对 expectedIconDerived", p)
		}
	}

	checkIconHashes := func(kind string, files map[string]string) {
		rels := make([]string, 0, len(files))
		for rel := range files {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		for _, rel := range rels {
			if strings.Contains(rel, "..") || filepath.IsAbs(rel) {
				t.Errorf("icon.manifest 含非法路径 %q", rel)
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Errorf("%s %s 读不到：%v（跑 make icon 重新派生）", kind, rel, err)
				continue
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != files[rel] {
				t.Errorf("%s %s 与 icon.manifest 登记的哈希不一致——改了 SVG 源请跑 make icon "+
					"重新派生并提交；派生物不许手改（ADR-0008）", kind, rel)
			}
		}
	}
	checkIconHashes("源", m.Sources)
	checkIconHashes("派生物", m.Derived)
}
