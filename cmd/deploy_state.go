package cmd

import (
	"errors"

	"github.com/hackctl/hackctl/cli/internal/config"
)

func loadRequiredDeployState(rootPath string) (config.DeployState, error) {
	state, err := config.LoadDeployState(rootPath)
	if errors.Is(err, config.ErrDeployStateNotFound) {
		return config.DeployState{}, errors.New("No services are deployed")
	}

	return state, err
}

func deployPlanFromState(state config.DeployState) (deployPlan, error) {
	parsedTarget, err := parseDeployTarget(state.Target)
	if err != nil {
		return deployPlan{}, err
	}

	services := make([]deployServicePlan, 0, len(state.Services))
	for _, service := range state.Services {
		services = append(services, deployServicePlan{
			Name:        service.Name,
			RemoteDir:   service.RemoteDir,
			Port:        service.Port,
			ProcessName: service.ProcessName,
		})
	}

	return deployPlan{
		Target:         parsedTarget,
		KeyPath:        state.KeyPath,
		Runtime:        state.Runtime,
		Mode:           state.Mode,
		RemoteRoot:     state.RemoteRoot,
		RemoteStateDir: state.RemoteStateDir,
		PublicService:  state.PublicService,
		PublicPort:     state.PublicPort,
		Services:       services,
	}, nil
}
