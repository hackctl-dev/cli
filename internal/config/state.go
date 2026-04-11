package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	StateDirname  = ".hackctl"
	StateFilename = "state.json"
)

type RuntimeState struct {
	Mode           string                  `json:"mode"`
	LiveURL        string                  `json:"liveUrl,omitempty"`
	TunnelProvider string                  `json:"tunnelProvider,omitempty"`
	TunnelPID      int                     `json:"tunnelPid,omitempty"`
	Services       map[string]ServiceState `json:"services"`
	LastUpdated    string                  `json:"lastUpdated"`
}

type ServiceState struct {
	PID    int    `json:"pid"`
	Status string `json:"status"`
}

func LoadRuntimeState(rootPath string) (RuntimeState, error) {
	statePath := filepath.Join(rootPath, StateDirname, StateFilename)
	body, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultRuntimeState(), nil
		}
		return RuntimeState{}, err
	}

	var state RuntimeState
	if err := json.Unmarshal(body, &state); err != nil {
		return RuntimeState{}, err
	}

	if state.Services == nil {
		state.Services = make(map[string]ServiceState)
	}

	if state.Mode == "" {
		state.Mode = "local"
	}

	return state, nil
}

func SaveRuntimeState(rootPath string, state RuntimeState) error {
	if state.Services == nil {
		state.Services = make(map[string]ServiceState)
	}

	state.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	stateDir := filepath.Join(rootPath, StateDirname)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(stateDir, StateFilename)
	return os.WriteFile(statePath, append(body, '\n'), 0o644)
}

func defaultRuntimeState() RuntimeState {
	return RuntimeState{
		Mode:     "local",
		Services: make(map[string]ServiceState),
	}
}
