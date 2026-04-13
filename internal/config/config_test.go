package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfigValid(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatalf("could not create frontend dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatalf("could not create backend dir: %v", err)
	}

	cfg := ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []ServiceConfig{
			{Name: "frontend", CWD: "frontend", Run: "npm run dev", Port: 3000},
			{Name: "backend", CWD: "backend", Run: "npm run dev", Port: 5000, EnvFile: "backend/.env"},
		},
		Share: ShareConfig{DefaultService: "frontend", DefaultPort: 3000},
	}

	writeProjectConfig(t, root, cfg)

	loaded, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(loaded.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(loaded.Services))
	}
}

func TestLoadProjectConfigMissingServiceFolder(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatalf("could not create frontend dir: %v", err)
	}

	cfg := ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []ServiceConfig{
			{Name: "frontend", CWD: "frontend", Run: "npm run dev", Port: 3000},
			{Name: "backend", CWD: "backend", Run: "npm run dev", Port: 5000},
		},
		Share: ShareConfig{DefaultService: "frontend", DefaultPort: 3000},
	}

	writeProjectConfig(t, root, cfg)

	_, err := LoadProjectConfig(root)
	if err == nil {
		t.Fatalf("expected error for missing service cwd")
	}

	if !strings.Contains(err.Error(), "service backend: cwd missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProjectConfigInvalidShareDefaultService(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatalf("could not create frontend dir: %v", err)
	}

	cfg := ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []ServiceConfig{
			{Name: "frontend", CWD: "frontend", Run: "npm run dev", Port: 3000},
		},
		Share: ShareConfig{DefaultService: "api", DefaultPort: 3000},
	}

	writeProjectConfig(t, root, cfg)

	_, err := LoadProjectConfig(root)
	if err == nil {
		t.Fatalf("expected share.defaultService validation error")
	}

	if !strings.Contains(err.Error(), "share.defaultService invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadProjectConfigInvalidPathEscape(t *testing.T) {
	root := t.TempDir()

	cfg := ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []ServiceConfig{
			{Name: "frontend", CWD: "../frontend", Run: "npm run dev", Port: 3000},
		},
		Share: ShareConfig{DefaultPort: 3000},
	}

	writeProjectConfig(t, root, cfg)

	_, err := LoadProjectConfig(root)
	if err == nil {
		t.Fatalf("expected path validation error")
	}

	if !strings.Contains(err.Error(), "service frontend: invalid cwd") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeProjectConfig(t *testing.T, root string, cfg ProjectConfig) {
	t.Helper()

	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal config: %v", err)
	}

	configPath := filepath.Join(root, ProjectConfigFilename)
	if err := os.WriteFile(configPath, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("could not write config: %v", err)
	}
}
