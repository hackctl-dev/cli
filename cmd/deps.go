package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type requiredDependency struct {
	name     string
	url      string
	minMajor int
}

var (
	nodeDownloadURL = "https://nodejs.org/en/download"
	versionPattern  = regexp.MustCompile(`\d+`)

	depGit = requiredDependency{
		name:     "git",
		url:      "https://git-scm.com/downloads",
		minMajor: 0,
	}
	depNode = requiredDependency{
		name:     "node",
		url:      nodeDownloadURL,
		minMajor: 20,
	}
	depNPM = requiredDependency{
		name:     "npm",
		url:      nodeDownloadURL,
		minMajor: 10,
	}
	depSSH = requiredDependency{
		name:     "ssh",
		url:      "https://www.openssh.com/portable.html",
		minMajor: 0,
	}
	depSCP = requiredDependency{
		name:     "scp",
		url:      "https://www.openssh.com/portable.html",
		minMajor: 0,
	}
)

func ensureDependencies(deps ...requiredDependency) error {
	for _, dep := range deps {
		path, err := exec.LookPath(dep.name)
		if err != nil {
			return fmt.Errorf("Missing dependency: %s (%s)", dep.name, dep.url)
		}

		if dep.minMajor == 0 {
			continue
		}

		major, err := dependencyMajorVersion(path)
		if err != nil || major < dep.minMajor {
			return fmt.Errorf("Unsupported dependency: %s >=%d (%s)", dep.name, dep.minMajor, dep.url)
		}
	}

	return nil
}

func dependencyMajorVersion(binaryPath string) (int, error) {
	versionCmd := exec.Command(binaryPath, "--version")
	output, err := versionCmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	return parseMajorVersion(string(output))
}

func parseMajorVersion(versionOutput string) (int, error) {
	match := versionPattern.FindString(strings.TrimSpace(versionOutput))
	if match == "" {
		return 0, errors.New("could not parse version")
	}

	major, err := strconv.Atoi(match)
	if err != nil {
		return 0, err
	}

	return major, nil
}
