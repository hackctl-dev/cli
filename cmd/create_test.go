package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hackctl/hackctl/cli/internal/config"
)

func TestEnsureGitignoreEntryCreatesFile(t *testing.T) {
	dir := t.TempDir()

	if err := ensureGitignoreEntry(dir, ".hackctl/"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("could not read .gitignore: %v", err)
	}

	if string(body) != ".hackctl/\n" {
		t.Fatalf("unexpected .gitignore content: %q", string(body))
	}
}

func TestEnsureGitignoreEntryAppendsWithoutDuplicates(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")

	if err := os.WriteFile(gitignorePath, []byte("backend/.env\n"), 0o644); err != nil {
		t.Fatalf("could not write seed .gitignore: %v", err)
	}

	if err := ensureGitignoreEntry(dir, ".hackctl/"); err != nil {
		t.Fatalf("unexpected error on first write: %v", err)
	}
	if err := ensureGitignoreEntry(dir, ".hackctl/"); err != nil {
		t.Fatalf("unexpected error on second write: %v", err)
	}

	body, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("could not read .gitignore: %v", err)
	}

	expected := "backend/.env\n.hackctl/\n"
	if string(body) != expected {
		t.Fatalf("unexpected .gitignore content: %q", string(body))
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
