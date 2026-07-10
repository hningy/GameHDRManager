//go:build windows

package discovery

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func ScanInstalled() ([]Candidate, error) {
	steam, _ := findSteamPath()
	steamCandidates := []Candidate{}
	if steam != "" {
		steamCandidates, _ = (SteamScanner{SteamPath: steam}).Scan()
	}
	epicCandidates, err := (EpicScanner{ManifestPath: filepath.Join(os.Getenv("PROGRAMDATA"), "Epic", "EpicGamesLauncher", "Data", "Manifests")}).Scan()
	if err != nil {
		return MergeCandidates(steamCandidates), nil
	}
	return MergeCandidates(steamCandidates, epicCandidates), nil
}

func findSteamPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if path, _, valueErr := key.GetStringValue("SteamPath"); valueErr == nil {
			if _, statErr := os.Stat(path); statErr == nil {
				return path, nil
			}
		}
	}
	for _, path := range []string{`C:\Program Files (x86)\Steam`, `D:\Steam`, `E:\Steam`} {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}
	return "", err
}
