package main

import (
	"errors"
	"fmt"
	"os"

	rootcmd "github.com/hackctl/hackctl/cli/cmd"
	"github.com/hackctl/hackctl/cli/internal/output"
)

func main() {
	if err := rootcmd.Execute(); err != nil {
		var silenced interface{ Silent() bool }
		if errors.As(err, &silenced) && silenced.Silent() {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, output.Error(err.Error()))
		os.Exit(1)
	}
}
