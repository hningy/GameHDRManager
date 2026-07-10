package config

import "testing"

func TestMigrateLegacyPreservesGameAndSettings(t *testing.T) {
	legacy := []byte(`{
  "games": [{"name":"Test","process":"test.exe","enabled":false,"source":"Steam","path":"C:/Games/test.exe"}],
  "settings":{"restore_hdr_on_exit":false,"start_monitoring_on_launch":true,"minimize_to_tray":true}
}`)
	cfg, err := migrateLegacy(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Games) != 1 {
		t.Fatalf("games = %d, want 1", len(cfg.Games))
	}
	g := cfg.Games[0]
	if g.Enabled || g.Source != "steam" || g.Launch.Value != "C:/Games/test.exe" || g.HDR.RestoreOnExit {
		t.Fatalf("legacy game was not preserved: %#v", g)
	}
	if !cfg.Settings.StartMonitoringOnLaunch || !cfg.Settings.MinimizeToTray {
		t.Fatalf("legacy settings were not preserved: %#v", cfg.Settings)
	}
}
