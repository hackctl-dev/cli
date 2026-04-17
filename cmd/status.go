package cmd

import (
	"fmt"
	"os"

	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployed project details",
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath, err := os.Getwd()
		if err != nil {
			return err
		}

		state, err := loadRequiredDeployState(rootPath)
		if err != nil {
			return err
		}

		fmt.Println(output.Section("Deployment"))
		fmt.Println(output.Field("Target", state.Target))
		fmt.Println(output.Field("Runtime", state.Runtime))
		fmt.Println(output.Field("Mode", state.Mode))
		fmt.Println(output.Field("Remote root", state.RemoteRoot))
		fmt.Println(output.Field("State dir", state.RemoteStateDir))
		if state.PublicURL != "" {
			fmt.Println(output.Field("Live URL", output.URL(state.PublicURL)))
		}
		if state.LastDeployedAt != "" {
			fmt.Println(output.Field("Last deployed", state.LastDeployedAt))
		}

		if len(state.Services) == 0 {
			return nil
		}

		fmt.Println()
		fmt.Println(output.Section("Services"))
		for _, service := range state.Services {
			fmt.Printf("- %s (%s) on port %d\n", service.Name, service.ProcessName, service.Port)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
