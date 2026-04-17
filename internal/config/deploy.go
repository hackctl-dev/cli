package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const DeployStateFilename = "deploy.json"

var ErrDeployStateNotFound = errors.New("deploy state not found")

type DeployState struct {
	Target         string               `json:"target"`
	KeyPath        string               `json:"keyPath"`
	Runtime        string               `json:"runtime"`
	Mode           string               `json:"mode"`
	RemoteRoot     string               `json:"remoteRoot"`
	RemoteStateDir string               `json:"remoteStateDir"`
	PublicService  string               `json:"publicService"`
	PublicPort     int                  `json:"publicPort"`
	PublicURL      string               `json:"publicUrl,omitempty"`
	TunnelPID      int                  `json:"tunnelPid,omitempty"`
	Services       []DeployServiceState `json:"services"`
	LastDeployedAt string               `json:"lastDeployedAt"`
}

type DeployServiceState struct {
	Name        string `json:"name"`
	RemoteDir   string `json:"remoteDir"`
	Port        int    `json:"port"`
	ProcessName string `json:"processName"`
}

func LoadDeployState(rootPath string) (DeployState, error) {
	statePath := filepath.Join(rootPath, StateDirname, DeployStateFilename)
	body, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DeployState{}, ErrDeployStateNotFound
		}
		return DeployState{}, err
	}

	var state DeployState
	if err := json.Unmarshal(body, &state); err != nil {
		return DeployState{}, err
	}

	if state.Services == nil {
		state.Services = []DeployServiceState{}
	}

	return state, nil
}

func SaveDeployState(rootPath string, state DeployState) error {
	if state.Services == nil {
		state.Services = []DeployServiceState{}
	}

	state.LastDeployedAt = time.Now().UTC().Format(time.RFC3339)

	stateDir := filepath.Join(rootPath, StateDirname)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(stateDir, DeployStateFilename)
	return os.WriteFile(statePath, append(body, '\n'), 0o644)
}

func DeleteDeployState(rootPath string) error {
	statePath := filepath.Join(rootPath, StateDirname, DeployStateFilename)
	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}
