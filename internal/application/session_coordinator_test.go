package application

import (
	"context"
	"testing"
	"time"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
	"github.com/huang/game-hdr-manager/internal/monitor"
)

func monitoredGame() domain.Game {
	return domain.Game{ID: "game", Name: "Game", Enabled: true, HDR: domain.HDRRule{EnableBeforeLaunch: true}, ExitConfirmSeconds: 5}
}

func TestExternalStartThenConfirmedExitRestoresHDR(t *testing.T) {
	controller := &fakeHDR{state: hdr.Off}
	coordinator := NewSessionCoordinator(controller)
	game := monitoredGame()
	now := time.Now()
	if err := coordinator.HandleTransition(context.Background(), monitor.Transition{Type: monitor.GameStarted, Game: game}, now); err != nil {
		t.Fatal(err)
	}
	if len(controller.sets) != 1 || controller.sets[0] != hdr.On {
		t.Fatalf("expected immediate HDR enable: %#v", controller.sets)
	}
	if err := coordinator.HandleTransition(context.Background(), monitor.Transition{Type: monitor.GameStopped, Game: game}, now); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background(), now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(controller.sets) != 1 {
		t.Fatal("must not restore before confirmation")
	}
	if err := coordinator.Tick(context.Background(), now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(controller.sets) != 2 || controller.sets[1] != hdr.Off {
		t.Fatalf("expected HDR restore: %#v", controller.sets)
	}
}

func TestManualHDRChangePreventsRestore(t *testing.T) {
	controller := &fakeHDR{state: hdr.Off}
	coordinator := NewSessionCoordinator(controller)
	game := monitoredGame()
	now := time.Now()
	if err := coordinator.HandleTransition(context.Background(), monitor.Transition{Type: monitor.GameStarted, Game: game}, now); err != nil {
		t.Fatal(err)
	}
	controller.state = hdr.Off // user changes HDR while game is running
	if err := coordinator.ObserveHDR(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.HandleTransition(context.Background(), monitor.Transition{Type: monitor.GameStopped, Game: game}, now); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(controller.sets) != 1 {
		t.Fatalf("must not overwrite user action: %#v", controller.sets)
	}
}
