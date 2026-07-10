package domain

import (
	"testing"
	"time"
)

func TestSessionRestoresOnlyAfterExitConfirmation(t *testing.T) {
	s := NewHDRSession()
	s.Begin(false)
	s.HDRReady(true)
	s.GameStarted("cyberpunk")

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	s.GameStopped("cyberpunk", now, 15*time.Second)
	if s.ShouldRestore(now.Add(14 * time.Second)) {
		t.Fatal("must not restore before exit confirmation completes")
	}
	if !s.ShouldRestore(now.Add(15 * time.Second)) {
		t.Fatal("must restore after exit confirmation completes")
	}
}

func TestSessionDoesNotRestoreAfterUserOverride(t *testing.T) {
	s := NewHDRSession()
	s.Begin(false)
	s.HDRReady(true)
	s.GameStarted("eldenring")
	s.UserChangedHDR()
	s.GameStopped("eldenring", time.Now(), 5*time.Second)
	if s.ShouldRestore(time.Now().Add(time.Minute)) {
		t.Fatal("must never overwrite a user HDR change")
	}
}

func TestRestartDuringExitConfirmationCancelsRestore(t *testing.T) {
	s := NewHDRSession()
	s.Begin(false)
	s.HDRReady(true)
	s.GameStarted("rdr2")
	now := time.Now()
	s.GameStopped("rdr2", now, 15*time.Second)
	s.GameStarted("rdr2")
	if s.ShouldRestore(now.Add(time.Minute)) {
		t.Fatal("a restarted game must cancel restore")
	}
}
