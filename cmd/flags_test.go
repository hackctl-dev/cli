package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAllNonHelpFlagsHaveShorthand(t *testing.T) {
	missing := make([]string, 0)

	var walk func(command *cobra.Command)
	walk = func(command *cobra.Command) {
		command.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Name == "help" {
				return
			}
			if flag.Shorthand == "" {
				missing = append(missing, fmt.Sprintf("%s --%s", command.CommandPath(), flag.Name))
			}
		})

		for _, child := range command.Commands() {
			if child.Name() == "help" {
				continue
			}
			walk(child)
		}
	}

	walk(rootCmd)

	if len(missing) > 0 {
		t.Fatalf("flags missing shorthand:\n- %s", joinLines(missing))
	}
}

func joinLines(items []string) string {
	if len(items) == 0 {
		return ""
	}

	result := items[0]
	for i := 1; i < len(items); i++ {
		result += "\n- " + items[i]
	}

	return result
}
