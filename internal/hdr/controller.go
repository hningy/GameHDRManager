package hdr

import "context"

type State uint8

const (
	Unknown State = iota
	Off
	On
)

func (s State) String() string {
	switch s {
	case Off:
		return "off"
	case On:
		return "on"
	default:
		return "unknown"
	}
}

// Controller is deliberately independent from the UI and monitor. A future
// Windows implementation can be replaced without changing HDR session rules.
type Controller interface {
	State(ctx context.Context) (State, error)
	Set(ctx context.Context, target State) (State, error)
}
