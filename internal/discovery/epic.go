package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huang/game-hdr-manager/internal/domain"
)

// EpicScanner reads launcher manifests. Unlike directory heuristics, Epic's
// LaunchExecutable field is a useful first candidate and remains editable in
// the game editor.
type EpicScanner struct{ ManifestPath string }

type epicManifest struct {
	DisplayName      string `json:"DisplayName"`
	InstallLocation  string `json:"InstallLocation"`
	LaunchExecutable string `json:"LaunchExecutable"`
	CatalogItemID    string `json:"CatalogItemId"`
	AppName          string `json:"AppName"`
}

func (s EpicScanner) Scan() ([]Candidate, error) {
	entries, err := os.ReadDir(s.ManifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Candidate{}, nil
		}
		return nil, fmt.Errorf("读取 Epic 清单目录: %w", err)
	}
	candidates := make([]Candidate, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".item") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.ManifestPath, entry.Name()))
		if err != nil {
			continue
		}
		var manifest epicManifest
		if json.Unmarshal(data, &manifest) != nil || manifest.DisplayName == "" || manifest.InstallLocation == "" {
			continue
		}
		executable := manifest.LaunchExecutable
		if executable != "" && !filepath.IsAbs(executable) {
			executable = filepath.Join(manifest.InstallLocation, executable)
		}
		processes := []string{}
		if executable != "" {
			processes = append(processes, filepath.Base(executable))
		}
		// Epic does not consistently expose one stable public protocol URI for
		// every title. Use the declared EXE; users can edit it if a launcher
		// wrapper is required for a particular game.
		candidates = append(candidates, Candidate{Name: manifest.DisplayName, Source: "epic", InstallPath: manifest.InstallLocation,
			Launch: domain.LaunchConfig{Type: domain.LaunchTypeExe, Value: executable}, Processes: processes})
	}
	return candidates, nil
}
