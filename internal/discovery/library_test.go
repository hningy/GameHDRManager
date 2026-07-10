package discovery

import "testing"

func TestGameFromCandidateCreatesEditableDefaults(t *testing.T) {
	game, err := GameFromCandidate(Candidate{Name: "Game", Source: "steam"})
	if err != nil {
		t.Fatal(err)
	}
	if game.ID == "" || !game.Enabled || !game.HDR.EnableBeforeLaunch || !game.HDR.RestoreOnExit {
		t.Fatalf("unexpected game: %#v", game)
	}
}
