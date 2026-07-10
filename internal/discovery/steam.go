package discovery

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/huang/game-hdr-manager/internal/domain"
)

type Candidate struct {
	Name        string
	Source      string
	InstallPath string
	Launch      domain.LaunchConfig
	Processes   []string
}

type SteamScanner struct{ SteamPath string }

func (s SteamScanner) Scan() ([]Candidate, error) {
	if strings.TrimSpace(s.SteamPath) == "" {
		return nil, fmt.Errorf("未找到 Steam 安装目录")
	}
	roots, err := steamLibraries(s.SteamPath)
	if err != nil {
		return nil, err
	}
	var candidates []Candidate
	for _, root := range roots {
		manifests, _ := filepath.Glob(filepath.Join(root, "appmanifest_*.acf"))
		for _, manifest := range manifests {
			candidate, ok := scanManifest(manifest, root)
			if ok {
				candidates = append(candidates, candidate)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name) })
	return candidates, nil
}

func steamLibraries(steamPath string) ([]string, error) {
	primary := filepath.Join(steamPath, "steamapps")
	if _, err := os.Stat(primary); err != nil {
		return nil, fmt.Errorf("读取 Steam 库: %w", err)
	}
	roots := []string{primary}
	data, err := os.ReadFile(filepath.Join(primary, "libraryfolders.vdf"))
	if err != nil {
		return roots, nil
	}
	for _, raw := range vdfValues(string(data), "path") {
		path := filepath.Join(strings.ReplaceAll(raw, "\\\\", "\\"), "steamapps")
		if _, err := os.Stat(path); err == nil {
			roots = append(roots, path)
		}
	}
	return uniquePaths(roots), nil
}

func scanManifest(filename, library string) (Candidate, bool) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Candidate{}, false
	}
	values := vdfMap(string(data))
	name, appID, installDir := values["name"], values["appid"], values["installdir"]
	if name == "" || appID == "" || installDir == "" {
		return Candidate{}, false
	}
	installPath := filepath.Join(library, "common", installDir)
	return Candidate{Name: name, Source: "steam", InstallPath: installPath, Processes: ExecutableNames(installPath),
		Launch: domain.LaunchConfig{Type: domain.LaunchTypeSteamURI, Value: "steam://rungameid/" + appID}}, true
}

// executableNames supplies a useful first pass for monitoring Steam games.
// Users can still edit this list when a game keeps its binary in a subfolder.
func ExecutableNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil { return nil }
	result := make([]string, 0, 4)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".exe") { continue }
		lower := strings.ToLower(entry.Name())
		if strings.Contains(lower, "unins") || strings.Contains(lower, "launcher") || strings.Contains(lower, "crash") || strings.Contains(lower, "redist") { continue }
		result = append(result, entry.Name())
	}
	sort.Strings(result)
	if len(result) > 12 { result = result[:12] }
	return result
}

var vdfPair = regexp.MustCompile(`(?m)^\s*"([^"]+)"\s+"([^"]*)"\s*$`)

func vdfMap(content string) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		matches := vdfPair.FindStringSubmatch(scanner.Text())
		if len(matches) == 3 {
			values[strings.ToLower(matches[1])] = matches[2]
		}
	}
	return values
}

func vdfValues(content, key string) []string {
	var values []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		matches := vdfPair.FindStringSubmatch(scanner.Text())
		if len(matches) == 3 && strings.EqualFold(matches[1], key) {
			values = append(values, matches[2])
		}
	}
	return values
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		key := strings.ToLower(filepath.Clean(path))
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, path)
		}
	}
	return result
}
