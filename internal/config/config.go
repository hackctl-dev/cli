package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	if err := validateProjectConfig(rootPath, cfg); err != nil {
		return ProjectConfig{}, err
	}

	return cfg, nil
}

func validateProjectConfig(rootPath string, cfg ProjectConfig) error {
	if len(cfg.Services) == 0 {
		return errors.New("config has no services")
	}

	serviceNames := make(map[string]struct{}, len(cfg.Services))

	for _, svc := range cfg.Services {
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			return errors.New("service name required")
		}
		if _, exists := serviceNames[name]; exists {
			return fmt.Errorf("duplicate service: %s", name)
		}
		serviceNames[name] = struct{}{}

		if strings.TrimSpace(svc.CWD) == "" {
			return fmt.Errorf("service %s: cwd required", name)
		}

		servicePath, err := resolveProjectPath(rootPath, svc.CWD)
		if err != nil {
			return fmt.Errorf("service %s: invalid cwd", name)
		}

		info, err := os.Stat(servicePath)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("service %s: cwd missing", name)
		}

		if strings.TrimSpace(svc.Run) == "" {
			return fmt.Errorf("service %s: run required", name)
		}

		if svc.Port <= 0 || svc.Port > 65535 {
			return fmt.Errorf("service %s: port invalid", name)
		}

		if strings.TrimSpace(svc.EnvFile) != "" {
			if _, err := resolveProjectPath(rootPath, svc.EnvFile); err != nil {
				return fmt.Errorf("service %s: envFile invalid", name)
			}
		}
	}

	if cfg.Share.DefaultPort < 0 || cfg.Share.DefaultPort > 65535 {
		return errors.New("share.defaultPort invalid")
	}

	defaultService := strings.TrimSpace(cfg.Share.DefaultService)
	if defaultService != "" {
		if _, ok := serviceNames[defaultService]; !ok {
			return errors.New("share.defaultService invalid")
		}
	}

	return nil
}

func resolveProjectPath(rootPath string, value string) (string, error) {
	normalized := filepath.Clean(filepath.FromSlash(strings.TrimSpace(value)))
	if filepath.IsAbs(normalized) {
		return "", errors.New("absolute path")
	}

	resolved := filepath.Join(rootPath, normalized)
	relative, err := filepath.Rel(rootPath, resolved)
	if err != nil {
		return "", err
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes project")
	}

	return resolved, nil
}
