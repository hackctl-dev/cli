package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hackctl/hackctl/cli/internal/config"
)

func TestCopyDirectorySkipsSecretsAndGeneratedFiles(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatalf("could not write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".env.example"), []byte("SECRET=\n"), 0o644); err != nil {
		t.Fatalf("could not write .env.example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatalf("could not write .DS_Store: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "frontend", "node_modules"), 0o755); err != nil {
		t.Fatalf("could not create node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "frontend", "node_modules", "pkg.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("could not write node_modules file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "frontend", "app.js"), []byte("console.log('ok')\n"), 0o644); err != nil {
		t.Fatalf("could not write app file: %v", err)
	}

	if err := copyDirectory(sourceDir, destinationDir); err != nil {
		t.Fatalf("copyDirectory failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destinationDir, ".env")); err == nil {
		t.Fatalf("expected .env to be excluded")
	}
	if _, err := os.Stat(filepath.Join(destinationDir, ".DS_Store")); err == nil {
		t.Fatalf("expected .DS_Store to be excluded")
	}
	if _, err := os.Stat(filepath.Join(destinationDir, "frontend", "node_modules")); err == nil {
		t.Fatalf("expected node_modules to be excluded")
	}

	if _, err := os.Stat(filepath.Join(destinationDir, ".env.example")); err != nil {
		t.Fatalf("expected .env.example to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destinationDir, "frontend", "app.js")); err != nil {
		t.Fatalf("expected app file to be copied: %v", err)
	}
}

func TestDependencyInstallTargetsRequiresValidConfig(t *testing.T) {
	root := t.TempDir()

	cfg := config.ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []config.ServiceConfig{
			{Name: "frontend", CWD: "frontend", Run: "npm run dev", Port: 3000},
		},
		Share: config.ShareConfig{DefaultService: "frontend", DefaultPort: 3000},
	}

	writeProjectConfigForCreateTest(t, root, cfg)

	_, err := dependencyInstallTargets(root)
	if err == nil {
		t.Fatalf("expected validation error for missing frontend folder")
	}
}

func TestDependencyInstallTargetsFromConfig(t *testing.T) {
	root := t.TempDir()
	frontendPath := filepath.Join(root, "frontend")
	if err := os.MkdirAll(frontendPath, 0o755); err != nil {
		t.Fatalf("could not create frontend folder: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendPath, "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("could not write package.json: %v", err)
	}

	cfg := config.ProjectConfig{
		Name:     "my-app",
		Template: "mern",
		Services: []config.ServiceConfig{
			{Name: "frontend", CWD: "frontend", Run: "npm run dev", Port: 3000},
		},
		Share: config.ShareConfig{DefaultService: "frontend", DefaultPort: 3000},
	}

	writeProjectConfigForCreateTest(t, root, cfg)

	targets, err := dependencyInstallTargets(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected one install target, got %d", len(targets))
	}

	if targets[0].name != "frontend" {
		t.Fatalf("unexpected target name: %q", targets[0].name)
	}
}

func writeProjectConfigForCreateTest(t *testing.T, root string, cfg config.ProjectConfig) {
	t.Helper()

	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, config.ProjectConfigFilename), append(body, '\n'), 0o644); err != nil {
		t.Fatalf("could not write config: %v", err)
	}
}
