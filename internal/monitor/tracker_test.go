package monitor

import (
	"testing"

	"github.com/huang/game-hdr-manager/internal/domain"
)

func TestTrackerUsesAnyConfiguredProcess(t *testing.T) {
	game := domain.Game{ID: "game-1", Name: "Example", Enabled: true, Processes: []string{"launcher.exe", "game.exe"}}
	tracker := NewTracker()
	started := tracker.Observe([]domain.Game{game}, Snapshot{"game.exe": {}})
	if len(started) != 1 || started[0].Type != GameStarted {
		t.Fatalf("start transitions = %#v", started)
	}
	// Launcher stays alive while game exits, so the game must remain active.
	stillRunning := tracker.Observe([]domain.Game{game}, Snapshot{"launcher.exe": {}})
	if len(stillRunning) != 0 {
		t.Fatalf("unexpected transitions = %#v", stillRunning)
	}
	stopped := tracker.Observe([]domain.Game{game}, Snapshot{})
	if len(stopped) != 1 || stopped[0].Type != GameStopped {
		t.Fatalf("stop transitions = %#v", stopped)
	}
}

func TestTrackerIgnoresDisabledGames(t *testing.T) {
	tracker := NewTracker()
	game := domain.Game{ID: "off", Name: "Disabled", Enabled: false, Processes: []string{"game.exe"}}
	if transitions := tracker.Observe([]domain.Game{game}, Snapshot{"game.exe": {}}); len(transitions) != 0 {
		t.Fatalf("disabled game transition = %#v", transitions)
	}
}
