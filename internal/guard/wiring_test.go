package guard

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 护栏 8：框架接线不得静默缺失
//
// 有些「约定」漏了不会有任何症状——不报错、不崩溃，只是某条链路默默断掉，
// 直到出问题要查日志时才发现日志里什么都没有。这类缺失只能靠检查兜住。
//
// 当前覆盖：
//   - options.App.Logger：不接 Wails logger，前端的 LogError/LogInfo 与 Wails 自身的
//     内部错误【只写 stdout】，打包后无人可见（详见 internal/logging/wails.go）。
//
// 铁律：解析到 0 个目标必须报错（护栏感知自己瞎了）。
// ---------------------------------------------------------------------------

// requiredWailsOptions 是必须出现在 options.App 字面量里的字段及其缺失后果。
var requiredWailsOptions = []struct {
	field  string
	reason string
}{
	{
		field:  "Logger",
		reason: "不设置它，前端日志与 Wails 内部错误只写 stdout（打包后无人可见），日志会静默丢失",
	},
	{
		field:  "DragAndDrop",
		reason: "不设置 EnableFileDrop，OnFileDrop 静默收不到任何拖拽事件（导入能力整条断掉且无症状）",
	},
}

// TestWailsOptionsAreWired 断言 main.go 里创建窗口的 options.App 字面量设置了必需字段。
func TestWailsOptionsAreWired(t *testing.T) {
	files, err := parseDir(projectRoot())
	if err != nil {
		t.Fatalf("解析项目根目录失败: %v", err)
	}
	f, ok := files["main.go"]
	if !ok {
		t.Fatalf("护栏找不到 main.go——写法可能已变更，请同步更新护栏解析规则")
	}

	lit := findWailsAppLiteral(f)
	if lit == nil {
		t.Fatal("护栏在 main.go 中未找到 &options.App{...} 字面量——写法可能已变更，请同步更新护栏解析规则")
	}

	present := map[string]bool{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok {
			present[ident.Name] = true
		}
	}
	if len(present) == 0 {
		t.Fatal("护栏解析到 options.App 字面量但没有任何具名字段——写法可能已变更，请同步更新护栏解析规则")
	}

	for _, req := range requiredWailsOptions {
		if !present[req.field] {
			t.Errorf("main.go 的 options.App 未设置 %s：%s", req.field, req.reason)
		}
	}
}

// ---------------------------------------------------------------------------
// 护栏 9：构建期注入目标必须真实存在
//
// 链接器的 `-X pkg.Symbol=value` 对【不存在的符号是静默忽略】的（已实测：
// 写错符号名时构建成功、无任何警告，变量保持零值，版本悄悄退回 "dev"）。
// 故打包脚本里的注入目标必须由护栏核对，否则改了变量名不会有任何症状，
// 只是发布的版本号从此永远是 "dev"。
// ---------------------------------------------------------------------------

// injectionTargetRe 匹配脚本里的 -X 注入：`-X <module>/internal/appinfo.<Symbol>=`
var injectionTargetRe = regexp.MustCompile(`-X\s+\S*internal/appinfo\.([A-Za-z0-9_]+)=`)

// TestVersionInjectionTargetsExist 断言构建脚本注入的符号在 appinfo 包真实存在。
//
// 注入点唯一：scripts/build.sh（make build / package.sh / release.sh / 容器构建 /
// smoke / verify-gen 都经它构建，见下方 TestWailsBuildHasSingleEntryPoint）。
// 这本身就是「版本号单一真相」的一部分——多处各算各的版本是双源漂移的温床。
func TestVersionInjectionTargetsExist(t *testing.T) {
	scripts := []string{"scripts/build.sh"}

	// 收集 internal/appinfo 里的包级变量名（注入目标必须是可寻址的字符串变量）
	dir := filepath.Join(projectRoot(), "internal", "appinfo")
	files, err := parseDir(dir)
	if err != nil {
		t.Fatalf("解析 internal/appinfo 失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("护栏在 %s 未解析到任何 .go 文件——写法可能已变更，请同步更新护栏解析规则", dir)
	}
	vars := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok.String() != "var" {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					vars[n.Name] = true
				}
			}
		}
	}
	if len(vars) == 0 {
		t.Fatal("护栏在 internal/appinfo 未解析到任何包级变量——写法可能已变更，请同步更新护栏解析规则")
	}

	checked := 0
	for _, rel := range scripts {
		data, err := os.ReadFile(filepath.Join(projectRoot(), rel))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", rel, err)
		}
		for _, m := range injectionTargetRe.FindAllStringSubmatch(string(data), -1) {
			checked++
			if !vars[m[1]] {
				t.Errorf("%s 注入 %s，但 internal/appinfo 中无此包级变量——"+
					"链接器会静默忽略，版本将永远退回 dev", rel, m[1])
			}
		}
	}
	if checked == 0 {
		t.Fatalf("护栏未在任何打包脚本中解析到 -X internal/appinfo.* 注入——"+
			"注入被移除或写法已变更，请同步更新护栏解析规则（%v）", scripts)
	}
}

// TestWailsBuildHasSingleEntryPoint 是护栏 9 的另一半：注入目标存在还不够，
// 构建【必须真的经过那个注入点】。
//
// 背景（005→011 累计审查抓到的真实缺陷）：scripts/build-linux-container.sh 在容器里
// 裸调 `wails build`，绕开了 build.sh 的 -ldflags 注入。产物照常构建成功、
// .deb 的 control 写着 0.1.0，而二进制内部 appinfo.Resolve() 恒为 "dev"
// （wails 传 -buildvcs=false，第二层回退也拿不到）。护栏 9 只扫 build.sh，
// 看不见这个旁路——于是同一版本两个真相，且没有任何症状。
//
// 规则：`wails build` 只允许出现在 scripts/build.sh。别的脚本要构建，必须调 build.sh。
// 解析到 0 处构建调用即报错（护栏感知自己瞎了）。
func TestWailsBuildHasSingleEntryPoint(t *testing.T) {
	root := projectRoot()
	const soleEntryPoint = "scripts/build.sh"

	// 匹配 wails CLI 的构建调用，覆盖三种写法：
	//   wails build / "$WAILS" build / "$(go env GOPATH)/bin/wails" build
	buildRe := regexp.MustCompile(`(?i)wails["'}]?\s+build\b`)

	files := []string{}
	scriptsDir := filepath.Join(root, "scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		t.Fatalf("读取 scripts 目录失败: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sh") {
			files = append(files, filepath.Join("scripts", e.Name()))
		}
	}
	workflows := filepath.Join(root, ".github", "workflows")
	wdEntries, err := os.ReadDir(workflows)
	if err != nil {
		t.Fatalf("读取 .github/workflows 失败——CI 也是构建入口，写法可能已变更: %v", err)
	}
	for _, e := range wdEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yml") {
			files = append(files, filepath.Join(".github", "workflows", e.Name()))
		}
	}
	if len(files) == 0 {
		t.Fatal("护栏未解析到任何构建脚本——写法可能已变更，请同步更新护栏解析规则")
	}

	matches := 0
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			// 注释与人读的输出文字不算构建调用（否则 "错误：wails build 失败" 会误报）。
			if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "echo ") {
				continue
			}
			if !buildRe.MatchString(line) {
				continue
			}
			matches++
			if rel != soleEntryPoint {
				t.Errorf("%s:%d 直接调用 wails build，绕开了 %s 的版本/提交号注入——"+
					"产物会自报 dev 而包元数据写着版本号。改为调用 bash %s\n\t%s",
					rel, i+1, soleEntryPoint, soleEntryPoint, strings.TrimSpace(line))
			}
		}
	}
	if matches == 0 {
		t.Fatalf("护栏在 scripts/ 与 .github/workflows/ 未解析到任何 wails build 调用——"+
			"构建入口被重写或写法已变更，请同步更新护栏解析规则（扫描 %d 个文件）", len(files))
	}
}

// ---------------------------------------------------------------------------
// 护栏 10：版本号与 bundle id 的单一真相不得失守
//
// 这两个字段都会「悄悄失效」：
//   - wails.json 缺 info.productVersion → wails 用默认值 "1.0.0"，
//     产物里写死 1.0.0，且不报任何错（本仓曾长期如此：系统显示 1.0.0、
//     日志显示 dev，两者都对不上）；
//   - build/darwin/Info.plist 沿用模板的 com.wails.<name> → 等于宣称 Wails 是厂商，
//     且与其他 Wails 项目共享 bundle id 命名空间。
// ---------------------------------------------------------------------------

// TestWailsJSONHasVersionSourceOfTruth 断言 wails.json 声明了可供产物渲染的版本与产品名。
func TestWailsJSONHasVersionSourceOfTruth(t *testing.T) {
	path := filepath.Join(projectRoot(), "wails.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 wails.json 失败: %v", err)
	}
	var cfg struct {
		Info struct {
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("解析 wails.json 失败: %v", err)
	}

	if cfg.Info.ProductVersion == "" {
		t.Error("wails.json 未声明 info.productVersion：wails 会静默用默认值 1.0.0，" +
			"产物里的版本从此与代码无关（macOS/Windows 版本资源都由它渲染）")
	} else if !numericVersionRe.MatchString(cfg.Info.ProductVersion) {
		t.Errorf("wails.json 的 info.productVersion = %q 不是数字点分格式："+
			"NSIS 的 VIProductVersion 要求数字版本，Windows 打包会在最后一步失败",
			cfg.Info.ProductVersion)
	}
	if cfg.Info.ProductName == "" {
		t.Error("wails.json 未声明 info.productName：macOS 的 CFBundleName 会为空")
	}
}

// numericVersionRe 匹配数字点分版本（如 0.1.0 / 1.2.3.4）。
var numericVersionRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+){2,3}$`)

// TestBundleIDIsProjectControlled 断言 bundle id 不沿用 wails 模板的 com.wails.* 前缀。
func TestBundleIDIsProjectControlled(t *testing.T) {
	path := filepath.Join(projectRoot(), "build", "darwin", "Info.plist")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 build/darwin/Info.plist 失败: %v", err)
	}
	content := string(data)

	idx := strings.Index(content, "CFBundleIdentifier")
	if idx < 0 {
		t.Fatal("Info.plist 中找不到 CFBundleIdentifier——写法可能已变更，请同步更新护栏解析规则")
	}
	// 取该键之后的一小段文本，只看它对应的取值行
	rest := content[idx:]
	if end := strings.Index(rest, "</string>"); end > 0 {
		rest = rest[:end]
	}
	if strings.Contains(rest, "com.wails.") {
		t.Error("Info.plist 的 CFBundleIdentifier 仍是模板默认的 com.wails.<name>——" +
			"等于把 Wails 当厂商，且与其他 Wails 项目共享 bundle id 命名空间。" +
			"应使用 scripts/init.sh 生成的 com.example.<name>（默认 com.example）")
	}
	if !strings.Contains(rest, "{{safeBundleID .Name}}") {
		t.Error("Info.plist 的 CFBundleIdentifier 未使用 {{safeBundleID .Name}} 派生应用名——" +
			"多个产品会撞同一个 bundle id")
	}
}

// ---------------------------------------------------------------------------
// 护栏 11：可观测性接线不得静默缺失
//
// 这类失效的共同点：删掉不会报错、不会崩溃，只是**失去可见性**——
// 慢查询回到无人可见的 stdout、启动优化重新变成盲调。症状要等到真出问题时才显形，
// 而那时正是最需要日志的时候。故只能靠检查兜住。
// ---------------------------------------------------------------------------

// TestGormLoggerIsWired 断言数据库连接配置了日志实现。
func TestGormLoggerIsWired(t *testing.T) {
	files, err := parseDir(filepath.Join(projectRoot(), "internal", "database"))
	if err != nil {
		t.Fatalf("解析 internal/database 失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("护栏在 internal/database 未解析到任何 .go 文件——写法可能已变更，请同步更新护栏解析规则")
	}

	lit := findCompositeLiteral(files, "gorm", "Config")
	if lit == nil {
		t.Fatal("护栏未找到 gorm.Config{...} 字面量——写法可能已变更，请同步更新护栏解析规则")
	}

	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Logger" {
				return // 已接线
			}
		}
	}
	t.Error("gorm.Config 未设置 Logger：gorm 默认 logger 写 stdout，" +
		"打包后的 GUI 应用无人可见，慢查询与 SQL 错误会静默消失")
}

// TestStartupTimelineStagesExist 断言组合根的启动阶段打点覆盖关键阶段。
//
// 删掉打点不会有任何症状，只是启动优化重新变成盲调（此前只有首尾两点，
// 实测 359–733ms 的中间部分是黑盒）。
func TestStartupTimelineStagesExist(t *testing.T) {
	files, err := parseDir(projectRoot())
	if err != nil {
		t.Fatalf("解析项目根目录失败: %v", err)
	}
	f, ok := files["main.go"]
	if !ok {
		t.Fatal("护栏找不到 main.go——写法可能已变更，请同步更新护栏解析规则")
	}

	// 期望覆盖的阶段（改名即失败，提示同步护栏）
	want := []string{"日志初始化", "数据库初始化", "配置加载与组装"}
	found := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "stage" {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if name, uerr := strconv.Unquote(lit.Value); uerr == nil {
				found[name] = true
			}
		}
		return true
	})

	if len(found) == 0 {
		t.Fatal("护栏在 main.go 未解析到任何启动阶段打点——写法可能已变更，请同步更新护栏解析规则")
	}
	for _, w := range want {
		if !found[w] {
			t.Errorf("启动时间线缺少阶段 %q：删掉打点不会有任何症状，"+
				"只会让启动优化重新变成盲调（现有阶段: %v）", w, found)
		}
	}
}

// findCompositeLiteral 在给定文件集合中定位 `pkg.Type{...}` 形式的复合字面量。
func findCompositeLiteral(files map[string]*ast.File, pkg, typeName string) *ast.CompositeLit {
	var found *ast.CompositeLit
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if found != nil {
				return false
			}
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := cl.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != pkg || sel.Sel.Name != typeName {
				return true
			}
			found = cl
			return false
		})
	}
	return found
}

// findWailsAppLiteral 在 main.go 中定位 `&options.App{...}`（wails.Run 的参数）。
func findWailsAppLiteral(f *ast.File) *ast.CompositeLit {
	var found *ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// 形如 options.App{...}
		sel, ok := cl.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkg.Name == "options" && strings.EqualFold(sel.Sel.Name, "App") {
			found = cl
			return false
		}
		return true
	})
	return found
}

// ---------------------------------------------------------------------------
// 护栏 12：构建/打包脚本不得依赖 python3
//
// 背景（005→011 累计审查抓到的真实缺陷）：package-deb.sh 与 app-name.sh 用 python3
// 解析 wails.json。本地 macOS 装了 python3 所以一切正常，而 CI 的 linux-deb job 跑在
// `container: ubuntu:20.04`——该镜像【没有 python3】（实测 command -v python3 为空），
// 于是 `set -euo pipefail` 下解释器缺失直接 127 退出，job 必红。
// 这类「只在 CI 环境暴露的依赖」不报错、不崩溃，只在推送后变红，故编译成会红的检查。
//
// 读 wails.json 请走 scripts/wails-info.sh（sed 实现，无解释器依赖）。
// 铁律：解析到 0 个脚本必须报错（护栏感知自己瞎了）。
// ---------------------------------------------------------------------------

func TestBuildScriptsDoNotDependOnPython(t *testing.T) {
	root := projectRoot()
	pythonRe := regexp.MustCompile(`\bpython3?\b`)

	files := []string{}
	for _, dir := range []string{"scripts", filepath.Join(".github", "workflows")} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("读取 %s 失败——写法可能已变更，请同步更新护栏解析规则: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".yml") {
				files = append(files, filepath.Join(dir, name))
			}
		}
	}
	if len(files) == 0 {
		t.Fatal("护栏未解析到任何构建脚本或工作流——写法可能已变更，请同步更新护栏解析规则")
	}

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue // 注释里讲历史原因会提到 python3，不是依赖
			}
			if pythonRe.MatchString(line) {
				t.Errorf("%s:%d 依赖 python3 —— CI 的 ubuntu:20.04 容器里没有它，该 job 会 127 退出。\n"+
					"读 wails.json 请改用 bash scripts/wails-info.sh <key>\n\t%s",
					rel, i+1, trimmed)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 护栏 13：Go 发出的事件必须有前端订阅（发射↔订阅双向核对）
//
// 背景（005→011 累计审查抓到的真实缺陷）：app.go 在批次结束时发 `image:done`，
// 前端全库只订了 `image:progress`，结果实际取自绑定调用的返回值。于是 done 是一条
// 永不被人接的信道——不报错、不崩溃，只是默默空转。这类接线漏口正是铁律 5 推论
// 要求编译成会红的检查。
//
// 做法：把两侧的 <domain>:<action> 都解析成字符串再比对（不靠约定、不靠人记）。
// 需要豁免时必须写出理由，且豁免项一旦真的被订阅就报错（防止白名单烂掉）。
// 铁律：任何一侧解析到 0 个结果都必须报错——护栏感知自己瞎了。
// ---------------------------------------------------------------------------

func TestEmittedEventsHaveSubscribers(t *testing.T) {
	root := projectRoot()

	// Go 侧动作常量的取值表：internal/event 的 Action*，与 main 包的 Action*。
	actionValues := map[string]string{}
	for key, val := range parseGoConstValues(t, filepath.Join(root, "internal", "event"), "Action", "") {
		actionValues[key] = val
	}
	for key, val := range parseGoConstValues(t, root, "Action", "") {
		actionValues[key] = val
	}

	// 1) 解析 Go 侧发射点：runtime.EventsEmit(ctx, event.Name("domain", [event.]ActionX), ...)
	emitRe := regexp.MustCompile(`EventsEmit\([^,]+,\s*event\.Name\(\s*"([^"]+)"\s*,\s*(?:event\.)?([A-Za-z0-9_]+)\s*\)`)
	goFiles, err := parseDir(root)
	if err != nil {
		t.Fatalf("解析根目录 Go 文件失败: %v", err)
	}
	emitted := map[string]string{} // "domain:action" -> 发射位置
	for name := range goFiles {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range emitRe.FindAllStringSubmatch(line, -1) {
				// actionValues 的键是去掉 Action 前缀后的部分（parseGoConstValues 的约定）。
				value, ok := actionValues[strings.TrimPrefix(m[2], "Action")]
				if !ok {
					t.Errorf("%s:%d 发射事件的 action 标识符 %s 解析不到常量取值——写法可能已变更，请同步更新护栏解析规则",
						name, i+1, m[2])
					continue
				}
				emitted[m[1]+":"+value] = fmt.Sprintf("%s:%d", name, i+1)
			}
		}
	}
	if len(emitted) == 0 {
		t.Fatalf("护栏在根目录未解析到任何 runtime.EventsEmit 发射点——"+
			"事件被移除或写法已变更，请同步更新护栏解析规则（%d 个文件）", len(goFiles))
	}

	// 2) 解析前端订阅点：useEvent/onEvent/EventsOn 的第一参数
	//    a) eventName('domain', EventAction.X)          b) 形如 SECOND_INSTANCE_EVENT 的常量
	eventMaps := map[string]map[string]string{}
	for _, spec := range []struct{ obj, file string }{
		{"EventAction", filepath.Join(root, "frontend/src/lib/event.ts")},
		{"AppEvent", filepath.Join(root, "frontend/src/lib/app.ts")},
	} {
		eventMaps[spec.obj] = parseTSObject(t, spec.file, spec.obj)
	}
	// 模板字符串常量：const NAME = `app:${AppEvent.SecondInstance}`
	constNameRe := regexp.MustCompile("const\\s+([A-Z0-9_]+)\\s*=\\s*`([^`]*)\\$\\{(EventAction|AppEvent)\\.([A-Za-z0-9_]+)\\}`")
	// 订阅的第一参数只有两种写法：eventName(...) 调用，或全大写的事件名常量。
	// 注意不能用 `[^,]+` 截断——eventName('image', EventAction.Progress) 内部就带逗号。
	subscribeRe := regexp.MustCompile(`(?:useEvent|onEvent|EventsOn)(?:<[^>]*>)?\(\s*(eventName\([^)]*\)|[A-Z0-9_]+)\s*,`)
	eventNameCallRe := regexp.MustCompile(`eventName\(\s*'([^']+)'\s*,\s*(EventAction|AppEvent)\.([A-Za-z0-9_]+)`)

	subscribed := map[string]string{}
	templateConsts := map[string]string{} // 常量名 -> 事件名，如 SECOND_INSTANCE_EVENT -> app:second-instance

	// 先把前端源码一次性收进来：常量可能定义在 A 文件、订阅写在 B 文件，
	// 单遍扫描会因遍历顺序漏配，故分两趟（先建常量表，再解析订阅）。
	type srcFile struct {
		rel  string
		text string
	}
	var sources []srcFile
	walkErr := filepath.WalkDir(filepath.Join(root, "frontend", "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if ext != ".ts" && ext != ".vue" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		sources = append(sources, srcFile{strings.TrimPrefix(path, root+"/"), string(data)})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("遍历前端源码失败: %v", walkErr)
	}
	if len(sources) == 0 {
		t.Fatal("护栏在 frontend/src 未解析到任何 .ts/.vue 文件——写法可能已变更，请同步更新护栏解析规则")
	}

	for _, src := range sources {
		for _, m := range constNameRe.FindAllStringSubmatch(src.text, -1) {
			if value, ok := eventMaps[m[3]][m[4]]; ok {
				templateConsts[m[1]] = m[2] + value
			}
		}
	}

	upperIdentRe := regexp.MustCompile(`^[A-Z0-9_]+$`)
	for _, src := range sources {
		for _, m := range subscribeRe.FindAllStringSubmatch(src.text, -1) {
			arg := strings.TrimSpace(m[1])
			if em := eventNameCallRe.FindStringSubmatch(arg); em != nil {
				if value, ok := eventMaps[em[2]][em[3]]; ok {
					subscribed[em[1]+":"+value] = src.rel
				}
				continue
			}
			if ident := upperIdentRe.FindString(arg); ident != "" {
				if name, ok := templateConsts[ident]; ok {
					subscribed[name] = src.rel
				}
			}
		}
	}
	if len(subscribed) == 0 {
		t.Fatal("护栏在 frontend/src 未解析到任何事件订阅（useEvent/onEvent/EventsOn）——" +
			"订阅写法已变更，请同步更新护栏解析规则")
	}

	// 豁免表：必须写理由；已被订阅或已无人发射的条目要报错（防白名单烂掉）。
	// 目前是空的——任何新增的「只发不收」信道都会在这里现形。
	exceptions := map[string]string{}

	for key, where := range emitted {
		if _, ok := subscribed[key]; ok {
			continue
		}
		if _, exempted := exceptions[key]; !exempted {
			t.Errorf("%s 由 %s 发射，但前端没有任何订阅——这条信道静默空转。"+
				"要么订阅，要么删掉发射；确需保留请在 exceptions 里写明理由", key, where)
		}
	}
	for key := range exceptions {
		if _, ok := subscribed[key]; ok {
			t.Errorf("豁免项 %s 现在已被订阅——请删除 exceptions 里的这条理由，白名单不该留着失效条目", key)
		}
		if _, ok := emitted[key]; !ok {
			t.Errorf("豁免项 %s 已无人发射——请删除 exceptions 里的这条理由", key)
		}
	}
}

// ---------------------------------------------------------------------------
// 护栏 14：启动底色必须与前端 tokens 的 --bg-base 对得上
//
// Go 在 webview 起来之前就得给 BackgroundColour，所以这份颜色【无法不复制】。
// 可复制的常量一旦没人看管就是第二处真相：005→011 累计审查发现它只对齐了深色档，
// 浅色主题用户启动时闪的是近黑色。护栏把「两边一致」编译成会红的检查。
//
// 铁律：任何一侧解析到 0 个取值必须报错（护栏感知自己瞎了）。
// ---------------------------------------------------------------------------

func TestStartupBackgroundMatchesThemeTokens(t *testing.T) {
	root := projectRoot()

	cssPath := filepath.Join(root, "frontend", "src", "styles", "theme.css")
	cssData, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("读取 theme.css 失败——写法可能已变更，请同步更新护栏解析规则: %v", err)
	}
	baseRe := regexp.MustCompile(`--bg-base\s*:\s*#([0-9a-fA-F]{6})`)
	tokens := map[string]bool{}
	for _, m := range baseRe.FindAllStringSubmatch(string(cssData), -1) {
		tokens[strings.ToLower(m[1])] = true
	}
	if len(tokens) == 0 {
		t.Fatal("护栏在 theme.css 未解析到任何 --bg-base token——写法可能已变更，请同步更新护栏解析规则")
	}

	mainData, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("读取 main.go 失败: %v", err)
	}
	fnRe := regexp.MustCompile(`(?s)func startupBackground[^{]*\{.*?\n\}`)
	body := fnRe.FindString(string(mainData))
	if body == "" {
		t.Fatal("护栏在 main.go 未解析到 func startupBackground——底色改回硬编码或改名了，请同步更新护栏解析规则")
	}
	retRe := regexp.MustCompile(`return\s+(\d+)\s*,\s*(\d+)\s*,\s*(\d+)`)
	returns := retRe.FindAllStringSubmatch(body, -1)
	if len(returns) == 0 {
		t.Fatal("护栏在 startupBackground 内未解析到任何 `return r, g, b`——写法可能已变更，请同步更新护栏解析规则")
	}

	for _, m := range returns {
		hex := fmt.Sprintf("%02x%02x%02x", mustInt(t, m[1]), mustInt(t, m[2]), mustInt(t, m[3]))
		if !tokens[hex] {
			t.Errorf("main.go 的启动底色 rgb(%s, %s, %s) = #%s 不在 theme.css 的 --bg-base 取值里（%v）——"+
				"改 token 时请同步 startupBackground", m[1], m[2], m[3], hex, keysOf(tokens))
		}
	}
}

func mustInt(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("解析数字 %q 失败: %v", s, err)
	}
	return n
}

func keysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, "#"+k)
	}
	return out
}
