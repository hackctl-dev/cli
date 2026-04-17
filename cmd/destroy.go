package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hackctl/hackctl/cli/internal/config"
	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the deployed project on the VPS",
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath, err := os.Getwd()
		if err != nil {
			return err
		}

		state, err := loadRequiredDeployState(rootPath)
		if err != nil {
			return err
		}

		plan, err := deployPlanFromState(state)
		if err != nil {
			return err
		}

		if err := output.RunSteps("Destroying deployment", func(addStep func(string) int, completeStep func(int)) error {
			stepID := addStep("Validating saved deploy state")
			if err := validateDeployKeyPath(plan.KeyPath); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Connecting to remote host")
			if err := checkRemoteConnection(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Removing deployed services")
			if err := destroyRemoteDeployment(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Clearing deploy state")
			if err := config.DeleteDeployState(rootPath); err != nil {
				return withDetail("deploy state cleanup failed", err.Error())
			}
			completeStep(stepID)

			return nil
		}); err != nil {
			return silent(err)
		}

		fmt.Println("Deployment successfully destroyed")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

func destroyRemoteDeployment(plan deployPlan) error {
	output, err := runRemoteScript(plan, buildDestroyScript(plan))
	if err != nil {
		return commandError("destroy failed", err, output)
	}

	return nil
}

func buildDestroyScript(plan deployPlan) string {
	var script strings.Builder
	script.WriteString("set -eu\n")
	for _, service := range plan.Services {
		script.WriteString("pm2 delete ")
		script.WriteString(shellQuote(service.ProcessName))
		script.WriteString(" >/dev/null 2>&1 || true\n")
	}
	pidPath := remotePath(plan.RemoteStateDir, "cloudflared.pid")
	script.WriteString("if [ -f ")
	script.WriteString(shellPath(pidPath))
	script.WriteString(" ]; then\n")
	script.WriteString("  kill \"$(cat ")
	script.WriteString(shellPath(pidPath))
	script.WriteString(")\" >/dev/null 2>&1 || true\n")
	script.WriteString("fi\n")
	script.WriteString("rm -rf ")
	script.WriteString(shellPath(plan.RemoteRoot))
	script.WriteString(" ")
	script.WriteString(shellPath(plan.RemoteStateDir))
	script.WriteString("\n")
	return script.String()
}
