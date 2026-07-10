package domain

import "strings"

// LaunchType describes how a game is started after HDR is ready.
type LaunchType string

const (
	LaunchTypeExe      LaunchType = "exe"
	LaunchTypeSteamURI LaunchType = "steam_uri"
	LaunchTypeEpicURI  LaunchType = "epic_uri"
)

type LaunchConfig struct {
	Type  LaunchType `json:"type"`
	Value string     `json:"value"`
}

type HDRRule struct {
	EnableBeforeLaunch bool `json:"enableBeforeLaunch"`
	RestoreOnExit      bool `json:"restoreOnExit"`
}

// Game is the persisted definition of one game. Processes contain the real
// game executables, not merely a platform launcher executable.
type Game struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Enabled            bool         `json:"enabled"`
	Source             string       `json:"source"`
	Launch             LaunchConfig `json:"launch"`
	Processes          []string     `json:"processes"`
	InstallPath        string       `json:"installPath,omitempty"`
	HDR                HDRRule      `json:"hdr"`
	ExitConfirmSeconds int          `json:"exitConfirmSeconds"`
}

func (g Game) IsRunning(processName string) bool {
	for _, process := range g.Processes {
		if strings.EqualFold(process, processName) {
			return true
		}
	}
	return false
}
