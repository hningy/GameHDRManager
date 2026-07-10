package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/monitor"
)

type RuntimeEventType string

const (
	RuntimeStarted   RuntimeEventType = "monitor_started"
	RuntimeStopped   RuntimeEventType = "monitor_stopped"
	RuntimeGameStart RuntimeEventType = "game_started"
	RuntimeGameStop  RuntimeEventType = "game_stopped"
	RuntimeError     RuntimeEventType = "error"
)

type RuntimeEvent struct {
	Type    RuntimeEventType
	Game    domain.Game
	Message string
}

// MonitorRuntime owns one monitoring goroutine. Its public methods are safe
// for the UI thread; all state-changing monitor work happens in that goroutine.
type MonitorRuntime struct {
	poller      monitor.Poller
	coordinator *SessionCoordinator
	interval    time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	runID  uint64
}

func NewMonitorRuntime(provider monitor.SnapshotProvider, coordinator *SessionCoordinator, interval time.Duration) *MonitorRuntime {
	return &MonitorRuntime{poller: monitor.Poller{Provider: provider}, coordinator: coordinator, interval: interval}
}

func (r *MonitorRuntime) Start(games []domain.Game, report func(RuntimeEvent)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return fmt.Errorf("监控已在运行")
	}
	if r.coordinator == nil {
		return fmt.Errorf("监控协调器未配置")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.runID++
	id := r.runID
	r.cancel = cancel
	go r.run(ctx, id, games, report)
	return nil
}

func (r *MonitorRuntime) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *MonitorRuntime) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

func (r *MonitorRuntime) AdoptLaunchedSession(session domain.HDRSession, game domain.Game) {
	r.coordinator.AdoptLaunchedSession(session, game.ID)
}

func (r *MonitorRuntime) run(ctx context.Context, id uint64, games []domain.Game, report func(RuntimeEvent)) {
	defer func() {
		r.mu.Lock()
		if r.runID == id {
			r.cancel = nil
		}
		r.mu.Unlock()
		report(RuntimeEvent{Type: RuntimeStopped, Message: "监控已停止"})
	}()
	interval := r.interval
	if interval < time.Second {
		interval = 2 * time.Second
	}
	pollTicker := time.NewTicker(interval)
	defer pollTicker.Stop()
	tickTicker := time.NewTicker(time.Second)
	defer tickTicker.Stop()
	report(RuntimeEvent{Type: RuntimeStarted, Message: "监控已启动"})
	r.observe(ctx, games, report) // Do not wait a full interval for existing games.
	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			r.observe(ctx, games, report)
		case now := <-tickTicker.C:
			if err := r.coordinator.ObserveHDR(ctx); err != nil {
				report(RuntimeEvent{Type: RuntimeError, Message: "HDR 状态检测失败：" + err.Error()})
			}
			if err := r.coordinator.Tick(ctx, now); err != nil {
				report(RuntimeEvent{Type: RuntimeError, Message: "HDR 恢复失败：" + err.Error()})
			}
		}
	}
}

func (r *MonitorRuntime) observe(ctx context.Context, games []domain.Game, report func(RuntimeEvent)) {
	transitions, err := r.poller.Observe(ctx, games)
	if err != nil {
		report(RuntimeEvent{Type: RuntimeError, Message: "进程扫描失败：" + err.Error()})
		return
	}
	for _, transition := range transitions {
		if err := r.coordinator.HandleTransition(ctx, transition, time.Now()); err != nil {
			report(RuntimeEvent{Type: RuntimeError, Game: transition.Game, Message: "处理游戏状态失败：" + err.Error()})
			continue
		}
		typeName := RuntimeGameStart
		message := "检测到游戏启动：" + transition.Game.Name
		if transition.Type == monitor.GameStopped {
			typeName = RuntimeGameStop
			message = "检测到游戏退出：" + transition.Game.Name
		}
		report(RuntimeEvent{Type: typeName, Game: transition.Game, Message: message})
	}
}
