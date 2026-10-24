package main

import (
	"embed"
	"log"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"file-processor/internal/appdir"
	"file-processor/internal/apperr"
	"file-processor/internal/appinfo"
	"file-processor/internal/config"
	"file-processor/internal/database"
	"file-processor/internal/logging"
	"file-processor/internal/service"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIconBytes []byte

// appName 应用标识，用于定位用户配置目录下的数据文件与日志。
const appName = "ImageWorkshop"

// appDisplayName 用户可见的产品名，用于窗口标题与托盘提示。
const appDisplayName = "图片工坊"

// bootTimer 记录组合根各阶段的耗时，输出启动时间线的 Go 侧半段。
//
// 为什么需要：此前启动只有「应用启动」与「前端已挂载」两点之差，实测 359–733ms
// （同一天两次运行差 2×），中间是黑盒——不知道该优化 webview、前端产物还是组合根。
// 前端那半段由 frontend/src/main.ts 记录，两半靠 app.log 的绝对时间戳拼起来
// （见 openspec/changes/perf-instrumentation/design.md D1）。
//
// 该打点由 internal/guard/wiring_test.go 核对数量：删掉没有任何症状，
// 只是启动优化重新变成盲调。
type bootTimer struct {
	start time.Time // 进程起点
	last  time.Time // 上一阶段结束点
}

// newBootTimer 以调用时刻为进程起点。
func newBootTimer() *bootTimer {
	now := time.Now()
	return &bootTimer{start: now, last: now}
}

// stage 记录一个阶段的耗时与该阶段结束时的累计耗时，并推进计时起点。
func (b *bootTimer) stage(name string) {
	now := time.Now()
	slog.Info("启动阶段",
		"stage", name,
		"ms", now.Sub(b.last).Milliseconds(),
		"total_ms", now.Sub(b.start).Milliseconds(),
	)
	b.last = now
}

func main() {
	boot := newBootTimer()

	// 日志：开发态 Debug，生产态 Info（构建模式判定见 buildmode_*.go）。
	logLevel := slog.LevelDebug
	if !debugBuild {
		logLevel = slog.LevelInfo
	}
	closeLog, err := logging.Init(logging.Options{
		AppName: appName,
		Level:   logLevel,
		Version: appinfo.Resolve(),
	})
	if err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}
	defer closeLog()
	boot.stage("日志初始化")

	// 组合根：数据库 → 服务 → 配置 → App
	db, err := database.Init(appName)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	boot.stage("数据库初始化")

	cfg, err := config.Load(appName)
	if cfg == nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	if err != nil {
		// 配置是可选项：丢失/损坏时用默认值继续启动，避免用户被一个坏文件挡在门外。
		slog.Warn("配置不可用，已回退默认值", "err", err)
	}
	svc := service.New(db, cfg)
	// 暂存区（ADR-0007）：处理结果先进应用数据目录的 history/，下载是唯一落盘出口。
	// 启动时修剪历史（保留最近 N 次的可下载暂存，D-56），失败不阻断启动。
	if historyDir, err := appdir.Dir(appName); err == nil {
		svc.SetHistoryRoot(filepath.Join(historyDir, "history"))
		if err := svc.PruneHistory(); err != nil {
			slog.Warn("历史修剪失败（不影响使用）", "err", err)
		}
	}
	app := NewApp(svc, cfg)
	boot.stage("配置加载与组装")

	// 窗口几何：在【创建期】给定尺寸与最大化状态，首帧即正确。
	// 不能在运行期设——OnStartup 在 goroutine 中执行，紧接着窗口就显示，二者存在竞态，
	// 用户会看到窗口跳一下（见 openspec/changes/runtime-pipeline/design.md D1）。
	winWidth, winHeight := cfg.WindowSize()
	startState := options.Normal
	if cfg.WindowMaximised {
		startState = options.Maximised
	}

	// 启动底色与前端 tokens 的 --bg-base 对齐：不一致时首帧前会闪一下错色
	// （浅色主题用户看到暗底启动，正是此前硬编码深色留下的症状）。
	// 这仍是一份【刻意保留的跨端复制】——webview 起来之前 Go 必须先给一个值；
	// 由护栏 TestStartupBackgroundMatchesThemeTokens 与 theme.css 双向核对，漂移即红。
	// theme 为空（跟随系统）时取 :root 的深色档，与 CSS 默认一致。
	bgR, bgG, bgB := startupBackground(cfg.Theme)

	// Create application with options
	err = wails.Run(&options.App{
		Title:            appDisplayName,
		Width:            winWidth,
		Height:           winHeight,
		WindowStartState: startState,
		// Wails 的日志接口接到 slog：不接的话，前端的 LogError/LogInfo 与 Wails 自身的
		// 内部错误【只写 stdout】——打包后无人可见，且不接不会有任何症状。
		// 故这条接线由 internal/guard/wiring_test.go 强制。
		Logger:             logging.WailsAdapter{},
		LogLevel:           logger.DEBUG,
		LogLevelProduction: logger.INFO,
		// 错误协议：把 Go 的 error 序列化为 JSON 字符串，前端 JSON.parse 还原为 {code,message}。
		// 注意：ErrorFormatter 签名是 func(error) any，但【必须】返回 string——
		// 返回对象会被前端 new Error(payload) 强转成 "[object Object]"（Wails v2.15.0 实测）。
		ErrorFormatter: apperr.Format,
		// 无边框按平台分叉：Windows/Linux 自绘标题栏（三按钮），macOS 用原生交通灯。
		// macOS 必须 false，否则 Wails 跳过 NSWindowStyleMaskTitled，交通灯消失（见 design D3）。
		Frameless: frameless(),
		// 拖拽导入必须显式开启：不设置时 OnFileDrop 静默不收任何事件（001 接线修正）。
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		// 单实例锁：二次启动唤起已有窗口。
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ImageWorkshop-single-instance",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				slog.Info("检测到二次启动，唤起已有窗口", "args", data.Args)
				runtime.Show(app.ctx)
				runtime.WindowUnminimise(app.ctx)
				// 把二次启动的启动参数推给前端（复用事件契约）。
				// 首次启动的参数不经事件——那时前端还没订阅，必然丢；用 GetLaunchArgs 查询。
				app.notifySecondInstance(data.Args, data.WorkingDirectory)
			},
		},
		// 关闭语义在运行期决定：托盘可用时隐藏到托盘，否则真退出（见 App.beforeClose）。
		// 不能用 HideWindowOnClose 静态开启——麒麟等环境可能根本没有托盘。
		HideWindowOnClose: false,
		OnBeforeClose:     app.beforeClose,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: bgR, G: bgG, B: bgB, A: 1}, // 与 tokens 的 --bg-base 一致，避免启动闪色
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		// macOS：透明标题栏 + 交通灯 inset + 内容顶到顶，不自绘三按钮。
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		// Windows：保留无边框窗口的圆角/阴影装饰，跟随系统明暗主题。
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// trayIcon 返回托盘图标字节。
func trayIcon() []byte {
	return trayIconBytes
}

// startupBackground 返回与主题一致的启动底色（R/G/B，0–255）。
//
// 取值必须与 frontend/src/styles/theme.css 的 --bg-base 一一对应：
//   - 深色（:root）  #0e0f12 → 14, 15, 18
//   - 浅色（light） #f7f8fa → 247, 248, 250
//
// 对应关系由 internal/guard/wiring_test.go 的护栏双向核对——改了 CSS token 而没改这里
// 会立刻变红，因为这份复制无法避免（webview 起来之前 Go 就得给出颜色）。
func startupBackground(theme string) (r, g, b uint8) {
	if theme == "light" {
		return 247, 248, 250
	}
	return 14, 15, 18
}
