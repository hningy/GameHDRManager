package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/huang/game-hdr-manager/internal/domain"
	"github.com/huang/game-hdr-manager/internal/discovery"
)

const currentVersion = 2

type Settings struct {
	ExitConfirmSeconds      int  `json:"exitConfirmSeconds"`
	StartMonitoringOnLaunch bool `json:"startMonitoringOnLaunch"`
	MinimizeToTray          bool `json:"minimizeToTray"`
	NotificationsEnabled    bool `json:"notificationsEnabled"`
}

type Config struct {
	Version  int           `json:"version"`
	Games    []domain.Game `json:"games"`
	Settings Settings      `json:"settings"`
}

func Default() Config {
	return Config{Version: currentVersion, Games: []domain.Game{}, Settings: Settings{
		ExitConfirmSeconds: 15, NotificationsEnabled: true,
	}}
}

type Store struct {
	path   string
	logger *log.Logger
}

func DefaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(base, "GameHDRManager", "config.json")
}

func NewStore(path string, logger *log.Logger) *Store { return &Store{path: path, logger: logger} }
func (s *Store) Path() string                         { return s.path }

func (s *Store) Load() (Config, error) {
	if data, err := os.ReadFile(s.path); err == nil {
		cfg, err := decode(data)
		return cfg, err
	} else if !errors.Is(err, os.ErrNotExist) {
		return Default(), fmt.Errorf("读取配置 %s: %w", s.path, err)
	}

	legacyPath := filepath.Join(".", "games_config.json")
	if legacy, err := os.ReadFile(legacyPath); err == nil {
		cfg, err := migrateLegacy(legacy)
		if err != nil {
			return Default(), fmt.Errorf("迁移旧配置: %w", err)
		}
		if err := s.Save(cfg); err != nil {
			return cfg, fmt.Errorf("保存迁移配置: %w", err)
		}
		if err := os.WriteFile(filepath.Join(".", "games_config.legacy.json"), legacy, 0o600); err != nil {
			s.logger.Printf("旧配置备份失败: %v", err)
		}
		s.logger.Printf("已从 %s 迁移 %d 个游戏", legacyPath, len(cfg.Games))
		return cfg, nil
	}

	cfg := Default()
	if err := s.Save(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	cfg.Version = currentVersion
	normalize(&cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("编码配置: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时配置: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时配置: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时配置: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("替换配置: %w", err)
	}
	return nil
}

func decode(data []byte) (Config, error) {
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	if cfg.Version > currentVersion {
		return Default(), fmt.Errorf("配置版本 %d 高于当前版本", cfg.Version)
	}
	normalize(&cfg)
	return cfg, nil
}

func normalize(cfg *Config) {
	cfg.Version = currentVersion
	if cfg.Games == nil {
		cfg.Games = []domain.Game{}
	}
	if cfg.Settings.ExitConfirmSeconds < 5 || cfg.Settings.ExitConfirmSeconds > 60 {
		cfg.Settings.ExitConfirmSeconds = 15
	}
	for i := range cfg.Games {
		g := &cfg.Games[i]
		if g.ExitConfirmSeconds < 5 || g.ExitConfirmSeconds > 60 {
			g.ExitConfirmSeconds = cfg.Settings.ExitConfirmSeconds
		}
		if g.Source == "" {
			g.Source = "manual"
		}
		if g.Launch.Type == "" && g.Launch.Value != "" {
			g.Launch.Type = domain.LaunchTypeExe
		}
		if len(g.Processes) == 0 && g.InstallPath != "" {
			g.Processes = discovery.ExecutableNames(g.InstallPath)
		}
	}
}

type legacyConfig struct {
	Games []struct {
		Name    string `json:"name"`
		Process string `json:"process"`
		Enabled *bool  `json:"enabled"`
		Source  string `json:"source"`
		Path    string `json:"path"`
	} `json:"games"`
	Settings struct {
		RestoreHDR     *bool `json:"restore_hdr_on_exit"`
		StartOnLaunch  bool  `json:"start_monitoring_on_launch"`
		MinimizeToTray bool  `json:"minimize_to_tray"`
	} `json:"settings"`
}

func migrateLegacy(data []byte) (Config, error) {
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Default(), err
	}
	cfg := Default()
	cfg.Settings.StartMonitoringOnLaunch = legacy.Settings.StartOnLaunch
	cfg.Settings.MinimizeToTray = legacy.Settings.MinimizeToTray
	for i, old := range legacy.Games {
		if strings.TrimSpace(old.Name) == "" || strings.TrimSpace(old.Process) == "" {
			continue
		}
		enabled := true
		if old.Enabled != nil {
			enabled = *old.Enabled
		}
		source := strings.ToLower(strings.TrimSpace(old.Source))
		if source == "" {
			source = "manual"
		}
		g := domain.Game{ID: fmt.Sprintf("legacy-%03d", i+1), Name: old.Name, Enabled: enabled, Source: source,
			Processes: []string{old.Process}, HDR: domain.HDRRule{EnableBeforeLaunch: true, RestoreOnExit: true}, ExitConfirmSeconds: 15}
		if legacy.Settings.RestoreHDR != nil {
			g.HDR.RestoreOnExit = *legacy.Settings.RestoreHDR
		}
		if old.Path != "" {
			g.Launch = domain.LaunchConfig{Type: domain.LaunchTypeExe, Value: old.Path}
		}
		cfg.Games = append(cfg.Games, g)
	}
	normalize(&cfg)
	return cfg, nil
}
