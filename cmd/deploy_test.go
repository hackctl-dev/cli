package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hackctl/hackctl/cli/internal/config"
)

func TestParseDeployTarget(t *testing.T) {
	target, err := parseDeployTarget("root@203.0.113.10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target.User != "root" || target.Host != "203.0.113.10" {
		t.Fatalf("unexpected parsed target: %#v", target)
	}
}

func TestParseDeployTargetRejectsInvalidValue(t *testing.T) {
	_, err := parseDeployTarget("root")
	if err == nil {
		t.Fatalf("expected invalid target error")
	}
}

func TestResolveDeployInputsFallsBackToSavedState(t *testing.T) {
	root := t.TempDir()
	state := config.DeployState{
		Target:  "root@example.com",
		KeyPath: filepath.Join(root, "id_ed25519"),
	}
	if err := os.WriteFile(state.KeyPath, []byte("key\n"), 0o600); err != nil {
		t.Fatalf("could not write key file: %v", err)
	}
	writeDeployStateForTest(t, root, state)

	target, keyPath, err := resolveDeployInputs(root, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target != state.Target {
		t.Fatalf("expected target %q, got %q", state.Target, target)
	}
	if keyPath != state.KeyPath {
		t.Fatalf("expected key path %q, got %q", state.KeyPath, keyPath)
	}
}

func TestResolveDeployInputsRequiresTargetAndKey(t *testing.T) {
	root := t.TempDir()

	_, _, err := resolveDeployInputs(root, "", "")
	if err == nil {
		t.Fatalf("expected missing input error")
	}
}

func writeDeployStateForTest(t *testing.T, root string, state config.DeployState) {
	t.Helper()

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal deploy state: %v", err)
	}

	stateDir := filepath.Join(root, config.StateDirname)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("could not create state dir: %v", err)
	}

	statePath := filepath.Join(stateDir, config.DeployStateFilename)
	if err := os.WriteFile(statePath, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("could not write deploy state: %v", err)
	}
}
