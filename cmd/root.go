package cmd

import (
	"fmt"

	"github.com/hackctl/hackctl/cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{if .HasAvailableSubCommands}} [command]{{end}}{{else if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var rootCmd = &cobra.Command{
	Use:   "hackctl",
	Short: "Launch hackathon projects faster.",
	Long:  "Launch hackathon projects faster.",
	RunE: func(cmd *cobra.Command, args []string) error {
		showVersion, err := cmd.Flags().GetBool("version")
		if err != nil {
			return err
		}

		if showVersion {
			fmt.Println(buildinfo.Summary())
			return nil
		}

		return cmd.Help()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
}

func Execute() error {
	return rootCmd.Execute()
}
