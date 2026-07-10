package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ignoredExecutableNames = []string{
	"unins", "uninstall", "unitycrashhandler", "crashreport", "launcher",
	"installer", "setup", "vcredist", "dxwebsetup", "directx", "redist",
}

// FindExecutableCandidates is used after a user selects a game. It does not
// run during the initial library scan, keeping the scan fast even on large
// Steam libraries.
func FindExecutableCandidates(root string, maxDepth int) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		} // inaccessible subdirectories are skipped
		if path != root {
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && strings.Count(rel, string(os.PathSeparator)) > maxDepth {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".exe") {
			return nil
		}
		name := strings.TrimSuffix(strings.ToLower(entry.Name()), ".exe")
		for _, ignored := range ignoredExecutableNames {
			if strings.Contains(name, ignored) {
				return nil
			}
		}
		candidates = append(candidates, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, leftErr := os.Stat(candidates[i])
		right, rightErr := os.Stat(candidates[j])
		if leftErr != nil || rightErr != nil {
			return candidates[i] < candidates[j]
		}
		return left.Size() > right.Size()
	})
	return candidates, nil
}
