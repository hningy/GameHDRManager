package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/huang/game-hdr-manager/internal/domain"
)

// Poller turns the single tasklist snapshot into transitions. It deliberately
// polls all processes at once, which remains cheap as the game library grows.
// WMI event delivery can later feed the same Tracker as an acceleration layer.
type Poller struct {
	Provider SnapshotProvider
	Tracker  *Tracker
	Interval time.Duration
}

func (p *Poller) Observe(ctx context.Context, games []domain.Game) ([]Transition, error) {
	if p.Provider == nil {
		return nil, fmt.Errorf("未配置进程快照提供器")
	}
	if p.Tracker == nil {
		p.Tracker = NewTracker()
	}
	snapshot, err := p.Provider.Running(ctx)
	if err != nil {
		return nil, err
	}
	return p.Tracker.Observe(games, snapshot), nil
}

func (p *Poller) Run(ctx context.Context, games []domain.Game, events chan<- Transition) error {
	interval := p.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		transitions, err := p.Observe(ctx, games)
		if err != nil {
			return err
		}
		for _, transition := range transitions {
			select {
			case events <- transition:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
