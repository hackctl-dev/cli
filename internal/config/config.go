package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const ProjectConfigFilename = "hackctl.config.json"

type ProjectConfig struct {
	Name     string          `json:"name"`
	Template string          `json:"template"`
	Services []ServiceConfig `json:"services"`
	Share    ShareConfig     `json:"share"`
}

type ShareConfig struct {
	DefaultService string `json:"defaultService"`
	DefaultPort    int    `json:"defaultPort"`
}

type ServiceConfig struct {
	Name    string `json:"name"`
	CWD     string `json:"cwd"`
	Run     string `json:"run"`
	Port    int    `json:"port"`
	EnvFile string `json:"envFile"`
}

func LoadProjectConfig(rootPath string) (ProjectConfig, error) {
	configPath := filepath.Join(rootPath, ProjectConfigFilename)
	body, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, fmt.Errorf("%s not found in %s", ProjectConfigFilename, rootPath)
		}
		return ProjectConfig{}, err
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return ProjectConfig{}, fmt.Errorf("invalid %s: %w", ProjectConfigFilename, err)
	}

	if len(cfg.Services) == 0 {
		return ProjectConfig{}, errors.New("hackctl.config.json has no services")
	}

	for _, svc := range cfg.Services {
		if svc.Name == "" {
			return ProjectConfig{}, errors.New("service name is required")
		}
		if svc.CWD == "" {
			return ProjectConfig{}, fmt.Errorf("service %q missing cwd", svc.Name)
		}
		if svc.Run == "" {
			return ProjectConfig{}, fmt.Errorf("service %q missing run command", svc.Name)
		}
	}

	return cfg, nil
}
