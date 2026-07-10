package application

import (
	"context"
	"errors"
	"testing"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
)

type fakeHDR struct {
	state hdr.State
	sets  []hdr.State
	err   error
}

func (f *fakeHDR) State(context.Context) (hdr.State, error) { return f.state, f.err }
func (f *fakeHDR) Set(_ context.Context, target hdr.State) (hdr.State, error) {
	f.sets = append(f.sets, target)
	if f.err != nil {
		return hdr.Unknown, f.err
	}
	f.state = target
	return target, nil
}

type fakeLauncher struct {
	called bool
	err    error
}

func (f *fakeLauncher) Launch(domain.LaunchConfig) error { f.called = true; return f.err }

func launchableGame() domain.Game {
	return domain.Game{Name: "Game", Enabled: true, Launch: domain.LaunchConfig{Type: domain.LaunchTypeExe, Value: "game.exe"}, HDR: domain.HDRRule{EnableBeforeLaunch: true}}
}

func TestStartEnablesHDRBeforeLaunching(t *testing.T) {
	h := &fakeHDR{state: hdr.Off}
	l := &fakeLauncher{}
	result, err := (StartService{HDR: h, Launcher: l}).Start(context.Background(), launchableGame())
	if err != nil {
		t.Fatal(err)
	}
	if !l.called || len(h.sets) != 1 || h.sets[0] != hdr.On {
		t.Fatalf("unexpected launch order: launcher=%v sets=%#v", l.called, h.sets)
	}
	if !result.Session.ChangedByApp || result.Session.State != domain.SessionLaunching {
		t.Fatalf("unexpected session: %#v", result.Session)
	}
}

func TestStartDoesNotChangeAlreadyEnabledHDR(t *testing.T) {
	h := &fakeHDR{state: hdr.On}
	l := &fakeLauncher{}
	result, err := (StartService{HDR: h, Launcher: l}).Start(context.Background(), launchableGame())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.sets) != 0 || result.Session.ChangedByApp {
		t.Fatalf("HDR must remain user-owned: %#v", result.Session)
	}
}

func TestStartRestoresHDRWhenLaunchFails(t *testing.T) {
	h := &fakeHDR{state: hdr.Off}
	l := &fakeLauncher{err: errors.New("boom")}
	if _, err := (StartService{HDR: h, Launcher: l}).Start(context.Background(), launchableGame()); err == nil {
		t.Fatal("launch failure expected")
	}
	if len(h.sets) != 2 || h.sets[0] != hdr.On || h.sets[1] != hdr.Off {
		t.Fatalf("must restore after failed launch: %#v", h.sets)
	}
}
