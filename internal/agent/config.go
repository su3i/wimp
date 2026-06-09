package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ControlPlaneUrl   string `json:"control_plane_url"`
	RegistrationToken string `json:"registration_token"`
	MachineId         uint   `json:"machine_id"`
}

func LoadConfig() (*Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not resolve executable path: %w", err)
	}

	path := filepath.Join(filepath.Dir(exe), "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config.json at %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config.json: %w", err)
	}

	if cfg.ControlPlaneUrl == "" {
		return nil, fmt.Errorf("config.json: control_plane_url is required")
	}
	if cfg.RegistrationToken == "" {
		return nil, fmt.Errorf("config.json: registration_token is required")
	}
	if cfg.MachineId == 0 {
		return nil, fmt.Errorf("config.json: machine_id is required")
	}

	return &cfg, nil
}
