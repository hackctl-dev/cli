package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hackctl/hackctl/cli/internal/templates"
)

const ProjectConfigFilename = "hackctl.config.json"

type ProjectConfig struct {
	Name     string          `json:"name"`
	Template string          `json:"template"`
	Services []ServiceConfig `json:"services"`
	Share    ShareConfig     `json:"share"`
	Deploy   *DeployConfig   `json:"deploy,omitempty"`
}

type ShareConfig struct {
	DefaultService string `json:"defaultService"`
	DefaultPort    int    `json:"defaultPort"`
}

type DeployConfig struct {
	Runtime string `json:"runtime"`
	Mode    string `json:"mode,omitempty"`
}

const (
	DeployRuntimePM2 = "pm2"
	DeployModeDev    = "dev"
	DeployModeProd   = "prod"
)

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
			return ProjectConfig{}, fmt.Errorf("%s not found", ProjectConfigFilename)
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
	officialTemplate := templates.IsOfficial(cfg.Template)

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

		if officialTemplate {
			if !isNPMRunCommand(svc.Run) {
				return fmt.Errorf("service %s: run must use npm run", name)
			}

			packageJSONPath := filepath.Join(servicePath, "package.json")
			info, err := os.Stat(packageJSONPath)
			if err != nil || info.IsDir() {
				return fmt.Errorf("service %s: package.json missing", name)
			}
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

	if cfg.Deploy != nil {
		deploy := normalizedDeployConfig(cfg.Deploy)
		if deploy.Runtime == "" {
			return errors.New("deploy.runtime required when deploy is configured")
		}
	}

	return nil
}

func ValidateDeployConfig(cfg ProjectConfig) (DeployConfig, error) {
	if cfg.Deploy == nil {
		return DeployConfig{}, errors.New("deploy.runtime required in hackctl.config.json")
	}

	deploy := normalizedDeployConfig(cfg.Deploy)
	if deploy.Runtime == "" {
		return DeployConfig{}, errors.New("deploy.runtime required in hackctl.config.json")
	}

	if deploy.Mode == "" {
		deploy.Mode = DeployModeDev
	}

	if deploy.Runtime != DeployRuntimePM2 {
		return DeployConfig{}, fmt.Errorf("unsupported deploy.runtime %s", deploy.Runtime)
	}

	switch deploy.Mode {
	case DeployModeDev:
		return deploy, nil
	case DeployModeProd:
		return DeployConfig{}, fmt.Errorf("deploy.mode %s is not supported yet", deploy.Mode)
	default:
		return DeployConfig{}, fmt.Errorf("unsupported deploy.mode %s", deploy.Mode)
	}
}

func normalizedDeployConfig(cfg *DeployConfig) DeployConfig {
	if cfg == nil {
		return DeployConfig{}
	}

	return DeployConfig{
		Runtime: strings.ToLower(strings.TrimSpace(cfg.Runtime)),
		Mode:    strings.ToLower(strings.TrimSpace(cfg.Mode)),
	}
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

func isNPMRunCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 {
		return false
	}

	if !strings.EqualFold(fields[0], "npm") && !strings.EqualFold(fields[0], "npm.cmd") {
		return false
	}

	return strings.EqualFold(fields[1], "run")
}
