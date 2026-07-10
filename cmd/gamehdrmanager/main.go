package main

import (
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2/app"
	"github.com/huang/game-hdr-manager/internal/application"
	"github.com/huang/game-hdr-manager/internal/config"
	"github.com/huang/game-hdr-manager/internal/hdr"
	"github.com/huang/game-hdr-manager/internal/launcher"
	"github.com/huang/game-hdr-manager/internal/monitor"
	"github.com/huang/game-hdr-manager/ui"
)

func main() {
	logger := log.New(os.Stdout, "[gamehdr] ", log.LstdFlags|log.Lmicroseconds)
	store := config.NewStore(config.DefaultPath(), logger)

	cfg, err := store.Load()
	if err != nil {
		logger.Printf("加载配置失败，已使用安全默认值: %v", err)
	}

	a := app.NewWithID("io.github.huang.gamehdrmanager")
	window := a.NewWindow("Game HDR Manager")
	controller := hdr.NewWindowsController()
	starter := application.StartService{
		HDR:      controller,
		Launcher: launcher.New(launcher.OSStarter{}),
	}
	runtime := application.NewMonitorRuntime(monitor.TasklistSnapshot{}, application.NewSessionCoordinator(controller), 2*time.Second)
	ui.NewApp(a, window, store, cfg, starter, runtime, controller, logger).Run()
}
