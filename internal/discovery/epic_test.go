package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEpicScannerReadsManifest(t *testing.T) {
	dir := t.TempDir()
	item := `{"DisplayName":"Alan Wake 2","InstallLocation":"C:\\Games\\Alan Wake 2","LaunchExecutable":"AlanWake2.exe"}`
	if err := os.WriteFile(filepath.Join(dir, "alan.item"), []byte(item), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := (EpicScanner{ManifestPath: dir}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d", len(candidates))
	}
	if candidates[0].Processes[0] != "AlanWake2.exe" {
		t.Fatalf("processes = %#v", candidates[0].Processes)
	}
}

func TestFindExecutableCandidatesFiltersInstallers(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"game.exe", "setup.exe", "bin/crashreport.exe"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := FindExecutableCandidates(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || filepath.Base(candidates[0]) != "game.exe" {
		t.Fatalf("candidates = %#v", candidates)
	}
}
