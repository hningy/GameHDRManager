package application

import (
	"context"
	"fmt"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
)

type GameLauncher interface {
	Launch(domain.LaunchConfig) error
}

type StartService struct {
	HDR      hdr.Controller
	Launcher GameLauncher
}

type StartResult struct {
	Session domain.HDRSession
	// Warning is non-empty when the game was launched but a best-effort
	// preparation step (currently HDR enable) did not complete.
	Warning string
}

// Start performs the only supported launch order: inspect HDR, change and
// verify it if required, then hand the game to its platform/EXE launcher.
func (s StartService) Start(ctx context.Context, game domain.Game) (StartResult, error) {
	if s.HDR == nil || s.Launcher == nil {
		return StartResult{}, fmt.Errorf("启动服务未完整配置")
	}
	if !game.Enabled {
		return StartResult{}, fmt.Errorf("游戏已禁用: %s", game.Name)
	}
	if game.Launch.Value == "" {
		return StartResult{}, fmt.Errorf("游戏未配置启动方式: %s", game.Name)
	}

	current, err := s.HDR.State(ctx)
	if err != nil || current == hdr.Unknown {
		if err == nil {
			err = fmt.Errorf("HDR 状态未知")
		}
		return StartResult{}, fmt.Errorf("启动前无法确认 HDR 状态: %w", err)
	}
	session := domain.NewHDRSession()
	session.Begin(current == hdr.On)

	changedByApp := false
	warning := ""
	if game.HDR.EnableBeforeLaunch && current != hdr.On {
		state, setErr := s.HDR.Set(ctx, hdr.On)
		if setErr != nil || state != hdr.On {
			// Do not silently swallow the failure, but do not block the user's
			// game either. Some Windows builds expose the HDR registry state but
			// reject the shortcut/API toggle; the old client still launched in
			// this situation. The UI surfaces this as a warning.
			if state == hdr.On {
				changedByApp = true
			}
			if setErr == nil {
				setErr = fmt.Errorf("HDR 未达到开启状态")
			}
			warning = "启动前开启 HDR 失败：" + setErr.Error()
		} else {
			changedByApp = true
		}
	}
	session.HDRReady(changedByApp)
	if err := s.Launcher.Launch(game.Launch); err != nil {
		session.LaunchFailed()
		// A failed game launch must not leave the desktop in a state it did
		// not have before the user clicked start.
		if changedByApp {
			_, _ = s.HDR.Set(ctx, hdr.Off)
		}
		return StartResult{Session: session}, fmt.Errorf("启动游戏失败: %w", err)
	}
	return StartResult{Session: session, Warning: warning}, nil
}
