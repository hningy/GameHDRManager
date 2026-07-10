package monitor

import (
	"sort"

	"github.com/huang/game-hdr-manager/internal/domain"
)

type TransitionType string

const (
	GameStarted TransitionType = "started"
	GameStopped TransitionType = "stopped"
)

type Transition struct {
	Type TransitionType
	Game domain.Game
}

// Tracker converts complete process snapshots into game transitions. It only
// decides whether a game is present; HDR timing and exit confirmation remain in
// the session coordinator.
type Tracker struct{ running map[string]bool }

func NewTracker() *Tracker { return &Tracker{running: make(map[string]bool)} }

func (t *Tracker) Observe(games []domain.Game, snapshot Snapshot) []Transition {
	transitions := make([]Transition, 0)
	seen := make(map[string]struct{}, len(games))
	for _, game := range games {
		if !game.Enabled || game.ID == "" {
			continue
		}
		seen[game.ID] = struct{}{}
		isRunning := false
		for _, process := range game.Processes {
			if snapshot.Has(process) {
				isRunning = true
				break
			}
		}
		wasRunning := t.running[game.ID]
		if isRunning && !wasRunning {
			transitions = append(transitions, Transition{Type: GameStarted, Game: game})
		}
		if !isRunning && wasRunning {
			transitions = append(transitions, Transition{Type: GameStopped, Game: game})
		}
		t.running[game.ID] = isRunning
	}
	// Drop deleted or disabled game IDs so a later re-add starts a fresh session.
	for id := range t.running {
		if _, exists := seen[id]; !exists {
			delete(t.running, id)
		}
	}
	sort.Slice(transitions, func(i, j int) bool { return transitions[i].Game.Name < transitions[j].Game.Name })
	return transitions
}
