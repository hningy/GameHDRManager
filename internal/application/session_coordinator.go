package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
	"github.com/huang/game-hdr-manager/internal/monitor"
)

// SessionCoordinator owns HDR restoration decisions for all games. Callers
// feed it monitor transitions and periodic HDR observations; it never assumes
// a process exit alone means HDR should immediately be turned off.
type SessionCoordinator struct {
	controller hdr.Controller
	session    domain.HDRSession
	mu         sync.Mutex
}

func NewSessionCoordinator(controller hdr.Controller) *SessionCoordinator {
	return &SessionCoordinator{controller: controller, session: domain.NewHDRSession()}
}

func (c *SessionCoordinator) AdoptLaunchedSession(session domain.HDRSession, gameID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = session
	c.session.GameStarted(gameID)
}

func (c *SessionCoordinator) HandleTransition(ctx context.Context, transition monitor.Transition, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if transition.Type == monitor.GameStarted {
		return c.onGameStarted(ctx, transition.Game)
	}
	if transition.Type == monitor.GameStopped {
		confirm := transition.Game.ExitConfirmSeconds
		if confirm < 5 {
			confirm = 15
		}
		c.session.GameStopped(transition.Game.ID, now, time.Duration(confirm)*time.Second)
	}
	return nil
}

func (c *SessionCoordinator) onGameStarted(ctx context.Context, game domain.Game) error {
	if c.session.State == domain.SessionIdle || c.session.State == domain.SessionFailed {
		current, err := c.controller.State(ctx)
		if err != nil || current == hdr.Unknown {
			if err == nil {
				err = fmt.Errorf("HDR 状态未知")
			}
			return err
		}
		c.session.Begin(current == hdr.On)
		changed := false
		// External starts cannot be pre-warmed, but immediate action is still
		// preferable to waiting for a debounce interval.
		if game.HDR.EnableBeforeLaunch && current == hdr.Off {
			state, setErr := c.controller.Set(ctx, hdr.On)
			if setErr != nil || state != hdr.On {
				c.session.LaunchFailed()
				if setErr == nil {
					setErr = fmt.Errorf("HDR 未达到开启状态")
				}
				return setErr
			}
			changed = true
		}
		c.session.HDRReady(changed)
	}
	c.session.GameStarted(game.ID)
	return nil
}

// ObserveHDR records user intervention. It is intentionally conservative: if
// the app switched HDR on but sees it off while a game is active, the app will
// never attempt to restore it later.
func (c *SessionCoordinator) ObserveHDR(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session.State != domain.SessionMonitoring || !c.session.ChangedByApp {
		return nil
	}
	state, err := c.controller.State(ctx)
	if err != nil {
		return err
	}
	if state != hdr.On {
		c.session.UserChangedHDR()
	}
	return nil
}

func (c *SessionCoordinator) Tick(ctx context.Context, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.session.ShouldRestore(now) {
		return nil
	}
	target := hdr.Off
	if c.session.InitialHDREnabled != nil && *c.session.InitialHDREnabled {
		target = hdr.On
	}
	state, err := c.controller.Set(ctx, target)
	if err != nil {
		return err
	}
	if state != target {
		return fmt.Errorf("HDR 恢复验证失败：当前 %s，期望 %s", state, target)
	}
	c.session.Finish()
	return nil
}

func (c *SessionCoordinator) Session() domain.HDRSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := c.session
	copy.ActiveGameIDs = make(map[string]struct{}, len(c.session.ActiveGameIDs))
	for id := range c.session.ActiveGameIDs {
		copy.ActiveGameIDs[id] = struct{}{}
	}
	return copy
}
