package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Summary() string {
	return fmt.Sprintf("hackctl %s", Version)
}
