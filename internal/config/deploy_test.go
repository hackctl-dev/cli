package config

import (
	"errors"
	"testing"
)

func TestValidateDeployConfigRequiresRuntime(t *testing.T) {
	_, err := ValidateDeployConfig(ProjectConfig{})
	if err == nil {
		t.Fatalf("expected deploy runtime validation error")
	}

	if err.Error() != "deploy.runtime required in hackctl.config.json" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDeployConfigDefaultsModeToDev(t *testing.T) {
	deploy, err := ValidateDeployConfig(ProjectConfig{
		Deploy: &DeployConfig{Runtime: DeployRuntimePM2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deploy.Mode != DeployModeDev {
		t.Fatalf("expected mode %q, got %q", DeployModeDev, deploy.Mode)
	}
}

func TestValidateDeployConfigRejectsUnsupportedRuntime(t *testing.T) {
	_, err := ValidateDeployConfig(ProjectConfig{
		Deploy: &DeployConfig{Runtime: "docker", Mode: DeployModeDev},
	})
	if err == nil {
		t.Fatalf("expected unsupported runtime error")
	}

	if err.Error() != "unsupported deploy.runtime docker" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDeployConfigRejectsProdModeForNow(t *testing.T) {
	_, err := ValidateDeployConfig(ProjectConfig{
		Deploy: &DeployConfig{Runtime: DeployRuntimePM2, Mode: DeployModeProd},
	})
	if err == nil {
		t.Fatalf("expected unsupported mode error")
	}

	if err.Error() != "deploy.mode prod is not supported yet" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveLoadAndDeleteDeployState(t *testing.T) {
	root := t.TempDir()

	original := DeployState{
		Target:         "root@example.com",
		KeyPath:        "/tmp/id_ed25519",
		Runtime:        DeployRuntimePM2,
		Mode:           DeployModeDev,
		RemoteRoot:     "~/hackctl/my-app",
		RemoteStateDir: "~/.hackctl/deploy/my-app",
		PublicService:  "frontend",
		PublicPort:     3000,
		PublicURL:      "https://example.trycloudflare.com",
		Services: []DeployServiceState{{
			Name:        "frontend",
			RemoteDir:   "~/hackctl/my-app/frontend",
			Port:        3000,
			ProcessName: "hackctl-my-app-frontend",
		}},
	}

	if err := SaveDeployState(root, original); err != nil {
		t.Fatalf("could not save deploy state: %v", err)
	}

	loaded, err := LoadDeployState(root)
	if err != nil {
		t.Fatalf("could not load deploy state: %v", err)
	}

	if loaded.Target != original.Target {
		t.Fatalf("expected target %q, got %q", original.Target, loaded.Target)
	}
	if loaded.Runtime != original.Runtime {
		t.Fatalf("expected runtime %q, got %q", original.Runtime, loaded.Runtime)
	}
	if len(loaded.Services) != 1 || loaded.Services[0].ProcessName != original.Services[0].ProcessName {
		t.Fatalf("unexpected loaded services: %#v", loaded.Services)
	}
	if loaded.LastDeployedAt == "" {
		t.Fatalf("expected last deployed timestamp to be set")
	}

	if err := DeleteDeployState(root); err != nil {
		t.Fatalf("could not delete deploy state: %v", err)
	}

	_, err = LoadDeployState(root)
	if !errors.Is(err, ErrDeployStateNotFound) {
		t.Fatalf("expected deploy state not found error, got %v", err)
	}
}
