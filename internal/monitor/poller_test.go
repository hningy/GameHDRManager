package monitor

import (
	"context"
	"testing"

	"github.com/huang/game-hdr-manager/internal/domain"
)

type snapshots struct {
	values []Snapshot
	index  int
}

func (s *snapshots) Running(context.Context) (Snapshot, error) {
	value := s.values[s.index]
	if s.index < len(s.values)-1 {
		s.index++
	}
	return value, nil
}

func TestPollerObserveProducesTransitions(t *testing.T) {
	provider := &snapshots{values: []Snapshot{{"game.exe": {}}, {}}}
	poller := Poller{Provider: provider}
	game := domain.Game{ID: "id", Name: "Game", Enabled: true, Processes: []string{"game.exe"}}
	started, err := poller.Observe(context.Background(), []domain.Game{game})
	if err != nil || len(started) != 1 || started[0].Type != GameStarted {
		t.Fatalf("start = %#v, err = %v", started, err)
	}
	stopped, err := poller.Observe(context.Background(), []domain.Game{game})
	if err != nil || len(stopped) != 1 || stopped[0].Type != GameStopped {
		t.Fatalf("stop = %#v, err = %v", stopped, err)
	}
}
