package cmd

import (
	"errors"

	"github.com/hackctl/hackctl/cli/internal/config"
)

func resolveShareTarget(cfg config.ProjectConfig) (string, int, error) {
	serviceName := cfg.Share.DefaultService
	if serviceName == "" {
		serviceName = "frontend"
	}

	for _, svc := range cfg.Services {
		if svc.Name == serviceName && svc.Port > 0 {
			return serviceName, svc.Port, nil
		}
	}

	if cfg.Share.DefaultPort > 0 {
		return serviceName, cfg.Share.DefaultPort, nil
	}

	if serviceName != "frontend" {
		for _, svc := range cfg.Services {
			if svc.Name == "frontend" && svc.Port > 0 {
				return "frontend", svc.Port, nil
			}
		}
	}

	return "", 0, errors.New("frontend share port is missing in hackctl.config.json")
}

func resolveSharePort(cfg config.ProjectConfig) (int, error) {
	_, port, err := resolveShareTarget(cfg)
	return port, err
}
