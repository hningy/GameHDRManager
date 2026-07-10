package ui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/huang/game-hdr-manager/internal/application"
	"github.com/huang/game-hdr-manager/internal/config"
	"github.com/huang/game-hdr-manager/internal/discovery"
	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
)

type App struct {
	app        fyne.App
	window     fyne.Window
	store      *config.Store
	config     config.Config
	starter    application.StartService
	runtime    *application.MonitorRuntime
	controller hdr.Controller
	logger     *log.Logger

	content       *fyne.Container
	status        *widget.Label
	activity      *widget.List
	events        []string
	hdrDetail     *widget.Label
	monitorDetail *widget.Label
	monitorButton *widget.Button
	animations    []*fyne.Animation
}

func NewApp(a fyne.App, window fyne.Window, store *config.Store, cfg config.Config, starter application.StartService, runtime *application.MonitorRuntime, controller hdr.Controller, logger *log.Logger) *App {
	return &App{app: a, window: window, store: store, config: cfg, starter: starter, runtime: runtime, controller: controller, logger: logger,
		events: []string{"应用已启动：等待监控", "配置已加载：" + store.Path()}}
}

func (a *App) Run() {
	a.app.Settings().SetTheme(darkTheme{})
	a.window.Resize(fyne.NewSize(1060, 720))
	a.window.SetFixedSize(false)
	a.window.SetCloseIntercept(func() {
		if a.config.Settings.MinimizeToTray {
			a.window.Hide()
		} else {
			a.app.Quit()
		}
	})
	a.installTray()
	a.showDashboard()
	go a.pollHDRStatus()
	if a.config.Settings.StartMonitoringOnLaunch {
		go func() { time.Sleep(500 * time.Millisecond); fyne.Do(a.ensureMonitoring) }()
	}
	a.window.ShowAndRun()
}

func (a *App) pollHDRStatus() {
	// Fyne callbacks queued before ShowAndRun may not execute. Start polling
	// after the event loop is alive, then keep the status card current.
	time.Sleep(500 * time.Millisecond)
	for {
		fyne.Do(a.refreshHDR)
		time.Sleep(1 * time.Second)
	}
}

func (a *App) installTray() {
	desktopApp, ok := a.app.(desktop.App)
	if !ok {
		return
	}
	desktopApp.SetSystemTrayMenu(fyne.NewMenu("Game HDR Manager",
		fyne.NewMenuItem("显示主窗口", func() { a.window.Show(); a.window.RequestFocus() }),
		fyne.NewMenuItem("开始/停止监控", a.toggleMonitoring),
		fyne.NewMenuItem("退出", func() { a.app.Quit() }),
	))
}

func (a *App) shell(view fyne.CanvasObject, title string) {
	for _, animation := range a.animations {
		animation.Stop()
	}
	a.animations = nil
	a.status.SetText("就绪")
	navigation := container.NewVBox(
		canvas.NewText("游戏 HDR 管理器", text),
		widget.NewSeparator(),
		a.navButton("概览", "概览", title, a.showDashboard),
		a.navButton("游戏库", "游戏库", title, a.showGames),
		a.navButton("活动日志", "活动日志", title, a.showActivity),
		a.navButton("设置", "设置", title, a.showSettings),
		layoutSpacer(),
		widget.NewLabelWithStyle("版本 0.1.0", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)
	leftBackground := canvas.NewRectangle(panel)
	leftBackground.CornerRadius = 0
	left := container.NewStack(leftBackground, container.NewPadded(navigation))
	left.Resize(fyne.NewSize(180, 0))
	top := container.NewBorder(nil, widget.NewSeparator(), nil, a.status,
		container.NewPadded(canvas.NewText(title, text)))
	a.content = container.NewBorder(top, nil, left, nil, container.NewPadded(view))
	overlay := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 150})
	a.window.SetContent(container.NewStack(a.content, overlay))
	fade := fyne.NewAnimation(180*time.Millisecond, func(value float32) {
		overlay.FillColor = color.NRGBA{R: 0, G: 0, B: 0, A: uint8(150 * (1 - value))}
		overlay.Refresh()
	})
	a.animations = append(a.animations, fade)
	fade.Start()
}

func (a *App) navButton(label, page, current string, handler func()) *widget.Button {
	b := widget.NewButton(label, handler)
	b.Alignment = widget.ButtonAlignLeading
	b.Importance = widget.LowImportance
	if page == current {
		b.Importance = widget.HighImportance
	}
	b.Resize(fyne.NewSize(180, 38))
	return b
}

func layoutSpacer() fyne.CanvasObject { return canvas.NewRectangle(colorTransparent{}) }

// colorTransparent is used only to let a VBox retain a compact visual spacer.
type colorTransparent struct{}

func (colorTransparent) RGBA() (uint32, uint32, uint32, uint32) { return 0, 0, 0, 0 }

func (a *App) showDashboard() {
	a.status = widget.NewLabelWithStyle("就绪", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
	manual := widget.NewButton("切换 HDR", a.toggleHDR)
	manual.Importance = widget.HighImportance
	hdrCard := a.statusCardWithAction("HDR 状态", "检测中…", warning, &a.hdrDetail, manual)
	monitorButton := widget.NewButton("启动监控", a.toggleMonitoring)
	monitorButton.Importance = widget.HighImportance
	a.monitorButton = monitorButton
	monCard := a.statusCardWithAction("监控状态", "未启动", accent, &a.monitorDetail, monitorButton)
	gameCard := a.statusCard("游戏库", fmt.Sprintf("已迁移/加载 %d 个游戏", len(a.config.Games)), success, nil)
	quickTitle := widget.NewLabelWithStyle("我的游戏", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	quick := a.dashboardGames()
	content := container.NewVBox(
		widget.NewLabelWithStyle("准备开始游戏？", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("选择一个游戏，GameHDR 会在启动前准备 HDR，并在退出后恢复。"),
		container.NewGridWithColumns(3, hdrCard, monCard, gameCard),
		widget.NewSeparator(),
		quickTitle,
		quick,
	)
	a.shell(content, "概览")
}

func (a *App) dashboardGames() fyne.CanvasObject {
	if len(a.config.Games) == 0 {
		add := widget.NewButton("添加第一个游戏", func() { a.showGames() })
		add.Importance = widget.HighImportance
		return container.NewVBox(widget.NewLabel("还没有游戏，先从 Steam / Epic 扫描或手动添加。"), add)
	}
	rows := make([]fyne.CanvasObject, 0, len(a.config.Games))
	limit := len(a.config.Games)
	if limit > 5 { limit = 5 }
	for i := 0; i < limit; i++ {
		game := a.config.Games[i]
		name := widget.NewLabelWithStyle(game.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		source := widget.NewLabel(game.Source)
		start := widget.NewButton("▶ 启动", func() { a.startGame(game) })
		start.Importance = widget.SuccessImportance
		start.Resize(fyne.NewSize(110, 36))
		rows = append(rows, container.NewBorder(nil, widget.NewSeparator(), container.NewVBox(name, source), start, nil))
	}
	return container.NewVBox(rows...)
}

func (a *App) toggleMonitoring() {
	if a.runtime == nil {
		a.addEvent("监控运行时未配置")
		return
	}
	if a.runtime.Running() {
		a.runtime.Stop()
		a.addEvent("正在停止监控")
		return
	}
	if err := a.runtime.Start(a.config.Games, a.handleRuntimeEvent); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.addEvent("监控启动请求已提交")
}

func (a *App) ensureMonitoring() {
	if a.runtime == nil || a.runtime.Running() {
		return
	}
	if err := a.runtime.Start(a.config.Games, a.handleRuntimeEvent); err != nil {
		a.addEvent("自动启动监控失败：" + err.Error())
	}
}

func (a *App) handleRuntimeEvent(event application.RuntimeEvent) {
	fyne.Do(func() {
		a.addEvent(event.Message)
		if event.Type == application.RuntimeError {
			a.status.SetText("监控：发生错误，请查看活动日志")
		}
		if a.monitorDetail != nil {
			if event.Type == application.RuntimeStarted {
				a.monitorDetail.SetText("监控中")
				if a.monitorButton != nil { a.monitorButton.SetText("停止监控") }
			}
			if event.Type == application.RuntimeStopped {
				a.monitorDetail.SetText("未启动")
				if a.monitorButton != nil { a.monitorButton.SetText("启动监控") }
			}
		}
	})
}

func (a *App) refreshHDR() {
	if a.controller == nil {
		return
	}
	go func() {
		state, err := a.controller.State(context.Background())
		fyne.Do(func() {
			if a.hdrDetail == nil {
				return
			}
			if err != nil || state == hdr.Unknown {
				a.hdrDetail.SetText("无法读取 Windows HDR 状态")
				return
			}
			if state == hdr.On {
				a.hdrDetail.SetText("Windows HDR 已开启")
			} else {
				a.hdrDetail.SetText("Windows HDR 已关闭")
			}
		})
	}()
}

func (a *App) toggleHDR() {
	if a.controller == nil {
		a.addEvent("HDR 控制器未配置")
		return
	}
	a.addEvent("正在读取 HDR 状态…")
	go func() {
		state, err := a.controller.State(context.Background())
		if err == nil && state != hdr.Unknown {
			target := hdr.On
			if state == hdr.On {
				target = hdr.Off
			}
			state, err = a.controller.Set(context.Background(), target)
		}
		fyne.Do(func() {
			if err != nil {
				a.addEvent("HDR 切换失败：" + err.Error())
				return
			}
			a.addEvent("HDR 已切换为：" + state.String())
			a.refreshHDR()
		})
	}()
}

func (a *App) statusCard(title, detail string, stateColor color.Color, detailTarget **widget.Label) fyne.CanvasObject {
	return a.statusCardWithAction(title, detail, stateColor, detailTarget, nil)
}

func (a *App) statusCardWithAction(title, detail string, stateColor color.Color, detailTarget **widget.Label, action fyne.CanvasObject) fyne.CanvasObject {
	dot := canvas.NewCircle(stateColor)
	dot.Resize(fyne.NewSize(12, 12))
	dotBox := container.NewGridWrap(fyne.NewSize(12, 12), dot)
	detailLabel := widget.NewLabel(detail)
	if detailTarget != nil {
		*detailTarget = detailLabel
	}
	if stateColor == success || stateColor == accent {
		pulse := fyne.NewAnimation(1400*time.Millisecond, func(value float32) {
			alpha := uint8(175 + 80*value)
			r, g, b, _ := stateColor.RGBA()
			dot.FillColor = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
			dot.Refresh()
		})
		pulse.AutoReverse = true
		pulse.RepeatCount = fyne.AnimationRepeatForever
		a.animations = append(a.animations, pulse)
		pulse.Start()
	}
	heading := container.NewHBox(dotBox, widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if action != nil {
		heading.Add(action)
	}
	inside := container.NewVBox(heading, detailLabel)
	back := canvas.NewRectangle(card)
	back.CornerRadius = 10
	return container.NewStack(back, container.NewPadded(inside))
}

func (a *App) showGames() {
	rows := make([]fyne.CanvasObject, 0, len(a.config.Games)+1)
	scan := widget.NewButton("扫描 Steam / Epic", a.scanGames)
	scan.Importance = widget.HighImportance
	manual := widget.NewButton("手动添加", func() { a.editGame(domain.Game{}) })
	manual.Importance = widget.MediumImportance
	rows = append(rows, container.NewBorder(nil, nil, widget.NewLabelWithStyle("游戏库", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewHBox(manual, scan), nil))
	if len(a.config.Games) == 0 {
		rows = append(rows, widget.NewLabel("还没有游戏，请扫描或手动添加。"))
	}
	for _, game := range a.config.Games {
		g := game
		state := "已启用"
		if !g.Enabled {
			state = "已禁用"
		}
		rows = append(rows, a.gameRow(g, state))
	}
	a.shell(container.NewVScroll(container.NewVBox(rows...)), "游戏库")
}

func (a *App) scanGames() {
	a.addEvent("开始扫描 Steam / Epic 游戏库")
	go func() {
		candidates, err := discovery.ScanInstalled()
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				a.addEvent("游戏扫描失败：" + err.Error())
				return
			}
			a.showScanResults(candidates)
		})
	}()
}

func (a *App) showScanResults(candidates []discovery.Candidate) {
	if len(candidates) == 0 {
		dialog.ShowInformation("扫描结果", "未发现 Steam 或 Epic 游戏。", a.window)
		a.addEvent("扫描完成：未发现游戏")
		return
	}
	selected := make([]*widget.Check, 0, len(candidates))
	rows := make([]fyne.CanvasObject, 0, len(candidates))
	existing := make(map[string]struct{})
	for _, game := range a.config.Games {
		if game.Launch.Value != "" {
			existing[strings.ToLower(game.Launch.Value)] = struct{}{}
		}
		for _, process := range game.Processes {
			existing[strings.ToLower(process)] = struct{}{}
		}
	}
	for _, candidate := range candidates {
		candidate := candidate
		label := candidate.Name + "  ·  " + candidate.Source
		if len(candidate.Processes) == 0 {
			label += "  （添加后需确认实际游戏进程）"
		}
		key := strings.ToLower(candidate.Launch.Value)
		_, alreadyAdded := existing[key]
		if alreadyAdded {
			label += "  （已添加）"
		}
		check := widget.NewCheck(label, nil)
		// Steam manifests often do not expose the real process. Still select
		// them by default so the user can add and edit the entry afterwards.
		check.SetChecked(!alreadyAdded)
		if alreadyAdded {
			check.Disable()
		}
		selected = append(selected, check)
		rows = append(rows, check)
	}
	content := container.NewVScroll(container.NewVBox(rows...))
	content.SetMinSize(fyne.NewSize(580, 360))
	dialog.NewCustomConfirm("发现 "+fmt.Sprint(len(candidates))+" 个游戏", "添加选中", "取消", content, func(confirm bool) {
		if !confirm {
			return
		}
		added := 0
		for index, check := range selected {
			if !check.Checked {
				continue
			}
			key := strings.ToLower(candidates[index].Launch.Value)
			if _, exists := existing[key]; exists {
				continue
			}
			game, err := discovery.GameFromCandidate(candidates[index])
			if err != nil {
				a.addEvent("添加游戏失败：" + err.Error())
				continue
			}
			a.config.Games = append(a.config.Games, game)
			existing[key] = struct{}{}
			added++
		}
		if added == 0 {
			return
		}
		if err := a.store.Save(a.config); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.addEvent(fmt.Sprintf("扫描完成：已添加 %d 个游戏", added))
		a.showGames()
	}, a.window).Show()
}

func (a *App) gameRow(game domain.Game, state string) fyne.CanvasObject {
	process := "自动监控"
	if len(game.Processes) == 0 { process = "未配置进程" }
	back := canvas.NewRectangle(card)
	back.CornerRadius = 8
	label := widget.NewLabel(fmt.Sprintf("%s\n%s  ·  %s  ·  %s", game.Name, game.Source, process, state))
	start := widget.NewButton("▶  启动", func() { a.startGame(game) })
	start.Importance = widget.SuccessImportance
	start.Resize(fyne.NewSize(92, 36))
	edit := widget.NewButton("✎  编辑", func() { a.editGame(game) })
	edit.Importance = widget.MediumImportance
	edit.Resize(fyne.NewSize(92, 36))
	buttons := container.NewHBox(edit, start)
	return container.NewStack(back, container.NewBorder(nil, nil, nil, container.NewPadded(buttons), container.NewPadded(label)))
}

func (a *App) editGame(game domain.Game) {
	name := widget.NewEntry()
	name.SetText(game.Name)
	processes := widget.NewEntry()
	processes.SetText(strings.Join(game.Processes, ", "))
	processes.SetPlaceHolder("可留空，但无法自动判断退出")
	launch := widget.NewEntry()
	launch.SetText(game.Launch.Value)
	delay := widget.NewEntry()
	delay.SetText(fmt.Sprint(game.ExitConfirmSeconds))
	enabled := widget.NewCheck("启用此游戏的 HDR 管理", nil)
	enabled.SetChecked(game.Enabled)
	restore := widget.NewCheck("退出后恢复 HDR", nil)
	restore.SetChecked(game.HDR.RestoreOnExit)
	items := []*widget.FormItem{
		widget.NewFormItem("游戏名称", name),
		widget.NewFormItem("监控进程（可选）", processes),
		widget.NewFormItem("启动命令 / URI", launch),
		widget.NewFormItem("退出确认（秒）", delay),
		widget.NewFormItem("", enabled),
		widget.NewFormItem("", restore),
	}
	dialog.NewForm("编辑游戏", "保存", "取消", items, func(confirm bool) {
		if !confirm {
			return
		}
		actualProcesses := strings.FieldsFunc(processes.Text, func(r rune) bool { return r == ',' || r == ';' || r == '，' })
		if strings.TrimSpace(name.Text) == "" || strings.TrimSpace(launch.Text) == "" {
			dialog.ShowError(fmt.Errorf("请填写游戏名称和启动方式"), a.window)
			return
		}
		confirmSeconds := game.ExitConfirmSeconds
		if _, err := fmt.Sscanf(delay.Text, "%d", &confirmSeconds); err != nil || confirmSeconds < 5 || confirmSeconds > 60 {
			dialog.ShowError(fmt.Errorf("退出确认时间应为 5 至 60 秒"), a.window)
			return
		}
		if game.ID == "" {
			created, err := discovery.GameFromCandidate(discovery.Candidate{Name: strings.TrimSpace(name.Text), Source: "manual", Launch: domain.LaunchConfig{Type: domain.LaunchTypeExe, Value: strings.TrimSpace(launch.Text)}, Processes: actualProcesses})
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			created.Enabled = enabled.Checked
			created.HDR.RestoreOnExit = restore.Checked
			created.ExitConfirmSeconds = confirmSeconds
			if strings.HasPrefix(strings.ToLower(created.Launch.Value), "steam://") {
				created.Launch.Type = domain.LaunchTypeSteamURI
			}
			if strings.HasPrefix(strings.ToLower(created.Launch.Value), "com.epicgames.launcher://") {
				created.Launch.Type = domain.LaunchTypeEpicURI
			}
			a.config.Games = append(a.config.Games, created)
		} else {
			for index := range a.config.Games {
				if a.config.Games[index].ID != game.ID {
					continue
				}
				updated := &a.config.Games[index]
				updated.Name = strings.TrimSpace(name.Text)
				updated.Processes = actualProcesses
				updated.Launch.Value = strings.TrimSpace(launch.Text)
				updated.Launch.Type = domain.LaunchTypeExe
				if strings.HasPrefix(strings.ToLower(updated.Launch.Value), "steam://") {
					updated.Launch.Type = domain.LaunchTypeSteamURI
				}
				if strings.HasPrefix(strings.ToLower(updated.Launch.Value), "com.epicgames.launcher://") {
					updated.Launch.Type = domain.LaunchTypeEpicURI
				}
				updated.Enabled = enabled.Checked
				updated.HDR.RestoreOnExit = restore.Checked
				updated.ExitConfirmSeconds = confirmSeconds
				break
			}
		}
		if err := a.store.Save(a.config); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.addEvent("已更新游戏配置：" + game.Name)
		a.showGames()
	}, a.window).Show()
}

func (a *App) startGame(game domain.Game) {
	a.addEvent("正在准备 HDR 并启动：" + game.Name)
	go func() {
		result, err := a.starter.Start(context.Background(), game)
		fyne.Do(func() {
			if err != nil {
				a.addEvent("启动失败：" + game.Name + " · " + err.Error())
				dialog.ShowError(err, a.window)
				return
			}
			if a.runtime != nil {
				a.runtime.AdoptLaunchedSession(result.Session, game)
			}
			a.ensureMonitoring()
			if result.Warning != "" {
				a.addEvent("警告：" + game.Name + " · " + result.Warning)
			}
			a.addEvent("已提交启动请求：" + game.Name)
		})
	}()
}

func (a *App) showActivity() {
	a.activity = widget.NewList(
		func() int { return len(a.events) },
		func() fyne.CanvasObject { return widget.NewLabel("事件") },
		func(id widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(a.events[id]) },
	)
	clear := widget.NewButton("清空", func() { a.events = nil; a.activity.Refresh() })
	export := widget.NewButton("导出", a.exportActivity)
	header := container.NewBorder(nil, nil, widget.NewLabelWithStyle("活动日志", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewHBox(clear, export), nil)
	a.shell(container.NewBorder(header, nil, nil, nil, a.activity), "活动日志")
}

func (a *App) exportActivity() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		for _, event := range a.events {
			_, _ = writer.Write([]byte(event + "\n"))
		}
		a.addEvent("活动日志已导出")
	}, a.window)
}

func (a *App) showSettings() {
	confirm := widget.NewSlider(5, 60)
	confirm.Step = 5
	confirm.SetValue(float64(a.config.Settings.ExitConfirmSeconds))
	confirm.OnChanged = func(value float64) { a.config.Settings.ExitConfirmSeconds = int(value); _ = a.store.Save(a.config) }
	content := container.NewVBox(
		widget.NewLabelWithStyle("设置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("退出确认时间（秒）"), confirm,
		widget.NewLabel("配置文件："+a.store.Path()),
	)
	startOnLaunch := widget.NewCheck("启动应用时自动监控", func(value bool) { a.config.Settings.StartMonitoringOnLaunch = value; _ = a.store.Save(a.config) })
	startOnLaunch.SetChecked(a.config.Settings.StartMonitoringOnLaunch)
	tray := widget.NewCheck("关闭窗口时最小化到托盘", func(value bool) { a.config.Settings.MinimizeToTray = value; _ = a.store.Save(a.config) })
	tray.SetChecked(a.config.Settings.MinimizeToTray)
	content.Add(startOnLaunch)
	content.Add(tray)
	content.Add(widget.NewLabel("手动 HDR、启动前 HDR 和退出恢复已经可用。"))
	a.shell(content, "设置")
}

func (a *App) addEvent(message string) {
	entry := time.Now().Format("15:04:05") + "  " + message
	a.events = append([]string{entry}, a.events...)
	a.logger.Print(message)
	if a.activity != nil {
		a.activity.Refresh()
	}
}
