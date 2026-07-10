package application

import (
	"context"
	"testing"
	"time"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/hdr"
	"github.com/huang/game-hdr-manager/internal/monitor"
)

func TestMonitorRuntimeStartStop(t *testing.T) {
	controller := &fakeHDR{state: hdr.On}
	runtime := NewMonitorRuntime(&snapshots{values: []monitor.Snapshot{{}}}, NewSessionCoordinator(controller), time.Second)
	events := make(chan RuntimeEvent, 2)
	if err := runtime.Start([]domain.Game{}, func(event RuntimeEvent) { events <- event }); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.Type != RuntimeStarted {
			t.Fatalf("first event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not start")
	}
	runtime.Stop()
	select {
	case event := <-events:
		if event.Type != RuntimeStopped {
			t.Fatalf("stop event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not stop")
	}
}

type snapshots struct {
	values []monitor.Snapshot
	index  int
}

func (s *snapshots) Running(_ context.Context) (monitor.Snapshot, error) {
	value := s.values[s.index]
	if s.index < len(s.values)-1 {
		s.index++
	}
	return value, nil
}
