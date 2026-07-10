package domain

import "time"

// SessionState contains only business state. It has no Windows or UI calls,
// making HDR restoration rules deterministic and testable.
type SessionState string

const (
	SessionIdle           SessionState = "idle"
	SessionPreparing      SessionState = "preparing"
	SessionLaunching      SessionState = "launching"
	SessionMonitoring     SessionState = "monitoring"
	SessionExitConfirming SessionState = "exit_confirming"
	SessionUserOverride   SessionState = "user_override"
	SessionFailed         SessionState = "failed"
)

type HDRSession struct {
	State             SessionState
	InitialHDREnabled *bool
	ChangedByApp      bool
	UserOverrode      bool
	ActiveGameIDs     map[string]struct{}
	ExitConfirmUntil  time.Time
}

func NewHDRSession() HDRSession {
	return HDRSession{State: SessionIdle, ActiveGameIDs: make(map[string]struct{})}
}

func (s *HDRSession) Begin(initialHDR bool) {
	s.State = SessionPreparing
	s.InitialHDREnabled = &initialHDR
	s.ChangedByApp = false
	s.UserOverrode = false
	s.ExitConfirmUntil = time.Time{}
}

func (s *HDRSession) HDRReady(changedByApp bool) {
	s.ChangedByApp = changedByApp
	s.State = SessionLaunching
}

func (s *HDRSession) LaunchFailed() { s.State = SessionFailed }

func (s *HDRSession) GameStarted(gameID string) {
	if s.ActiveGameIDs == nil {
		s.ActiveGameIDs = make(map[string]struct{})
	}
	s.ActiveGameIDs[gameID] = struct{}{}
	s.ExitConfirmUntil = time.Time{}
	if !s.UserOverrode {
		s.State = SessionMonitoring
	}
}

func (s *HDRSession) GameStopped(gameID string, now time.Time, confirmFor time.Duration) {
	delete(s.ActiveGameIDs, gameID)
	if len(s.ActiveGameIDs) == 0 && !s.UserOverrode {
		s.State = SessionExitConfirming
		s.ExitConfirmUntil = now.Add(confirmFor)
	}
}

func (s *HDRSession) UserChangedHDR() {
	s.UserOverrode = true
	s.State = SessionUserOverride
}

func (s *HDRSession) ShouldRestore(now time.Time) bool {
	return s.State == SessionExitConfirming && !s.UserOverrode && s.ChangedByApp &&
		len(s.ActiveGameIDs) == 0 && !s.ExitConfirmUntil.IsZero() && !now.Before(s.ExitConfirmUntil)
}

func (s *HDRSession) Finish() {
	s.State = SessionIdle
	s.InitialHDREnabled = nil
	s.ChangedByApp = false
	s.UserOverrode = false
	s.ActiveGameIDs = make(map[string]struct{})
	s.ExitConfirmUntil = time.Time{}
}
