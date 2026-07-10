package launcher

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/huang/game-hdr-manager/internal/domain"
)

// CommandStarter is deliberately small so that launch requests can be tested
// without starting a game or platform client.
type CommandStarter interface {
	Start(name string, args ...string) error
}

type OSStarter struct{}

func (OSStarter) Start(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

type Launcher struct{ starter CommandStarter }

func New(starter CommandStarter) Launcher { return Launcher{starter: starter} }

func (l Launcher) Launch(launch domain.LaunchConfig) error {
	value := strings.TrimSpace(launch.Value)
	if value == "" {
		return fmt.Errorf("未配置启动命令")
	}
	switch launch.Type {
	case domain.LaunchTypeExe:
		if strings.EqualFold(filepath.Ext(value), ".url") && isNativeStarter(l.starter) {
			if err := openWithShell(value); err == nil {
				return nil
			} else if fallbackErr := l.starter.Start("rundll32.exe", "url.dll,FileProtocolHandler", value); fallbackErr != nil {
				return fmt.Errorf("打开快捷方式失败：%v；兜底启动失败：%w", err, fallbackErr)
			} else {
				return nil
			}
		}
		return l.starter.Start(value)
	case domain.LaunchTypeSteamURI:
		if !strings.HasPrefix(strings.ToLower(value), "steam://") {
			return fmt.Errorf("Steam 启动地址无效")
		}
	case domain.LaunchTypeEpicURI:
		if !strings.HasPrefix(strings.ToLower(value), "com.epicgames.launcher://") {
			return fmt.Errorf("Epic 启动地址无效")
		}
	default:
		return fmt.Errorf("不支持的启动方式: %q", launch.Type)
	}
	if isNativeStarter(l.starter) {
		if err := openWithShell(value); err == nil {
			return nil
		} else if fallbackErr := l.starter.Start("rundll32.exe", "url.dll,FileProtocolHandler", value); fallbackErr != nil {
			return fmt.Errorf("打开启动地址失败：%v；兜底启动失败：%w", err, fallbackErr)
		}
		return nil
	}
	// Test doubles use the old command path; the real Windows starter uses
	// ShellExecuteW so protocol registration and .url files work reliably.
	return l.starter.Start("rundll32.exe", "url.dll,FileProtocolHandler", value)
}

func isNativeStarter(starter CommandStarter) bool { _, ok := starter.(OSStarter); return ok }
