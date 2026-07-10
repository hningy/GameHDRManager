package hdr

import "testing"

func TestStateString(t *testing.T) {
	tests := map[State]string{Unknown: "unknown", Off: "off", On: "on"}
	for state, want := range tests {
		if got := state.String(); got != want {
			t.Fatalf("State(%d) = %q, want %q", state, got, want)
		}
	}
}
