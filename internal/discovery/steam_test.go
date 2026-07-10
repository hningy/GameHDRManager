package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSteamScannerReadsAppManifest(t *testing.T) {
	steam := t.TempDir()
	apps := filepath.Join(steam, "steamapps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `"AppState"
{
  "appid" "1091500"
  "name" "Cyberpunk 2077"
  "installdir" "Cyberpunk 2077"
}`
	if err := os.WriteFile(filepath.Join(apps, "appmanifest_1091500.acf"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := (SteamScanner{SteamPath: steam}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	if candidates[0].Launch.Value != "steam://rungameid/1091500" {
		t.Fatalf("launch = %s", candidates[0].Launch.Value)
	}
}
