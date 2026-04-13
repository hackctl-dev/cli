package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hackctl/hackctl/cli/internal/config"
	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/hackctl/hackctl/cli/internal/templates"
	"github.com/spf13/cobra"
)

var (
	createTemplate string
)

var createCmd = &cobra.Command{
	Use:   "create [flags] <path>",
	Short: "Create a new hackctl project",
	Args:  validateCreateArgs,
	Example: "hackctl create --template mern .\n" +
		"hackctl create --template mern my-app",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(output.ASCIIBanner())

		createName := args[0]

		source, err := templates.Resolve(createTemplate)
		if err != nil {
			return err
		}

		targetPath, err := resolveTargetPath(createName)
		if err != nil {
			return err
		}

		if err := output.RunSteps("Creating project", func(addStep func(string) int, completeStep func(int)) error {
			stepID := addStep("Preparing project directory")
			if err := ensureWritableTarget(targetPath, createName); err != nil {
				return err
			}
			if err := ensureDependencies(depGit, depNode, depNPM); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Downloading template")
			if err := cloneTemplate(source, targetPath); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Updating ignore rules")
			if err := ensureGitignoreEntry(targetPath, ".hackctl/"); err != nil {
				return errors.New("could not update .gitignore")
			}
			completeStep(stepID)

			installTargets, err := dependencyInstallTargets(targetPath)
			if err != nil {
				return err
			}

			for _, target := range installTargets {
				stepID = addStep(fmt.Sprintf("Installing %s dependencies", target.name))
				if err := installDependencies(target); err != nil {
					return err
				}
				completeStep(stepID)
			}

			return nil
		}); err != nil {
			return silent(err)
		}

		fmt.Println("Project successfully created")
		fmt.Println()
		if createName == "." {
			fmt.Println(output.Footer("Next: hackctl start"))
		} else {
			fmt.Println(output.Footer(fmt.Sprintf("Next: cd %s && hackctl start", createName)))
		}

		return nil
	},
}

func init() {
	createCmd.Flags().StringVarP(&createTemplate, "template", "t", "mern", "Template to scaffold")
	createCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(createCmd)
}

func validateCreateArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		return nil
	}

	if len(args) == 0 {
		return errors.New("missing path argument (usage: hackctl create <path>)")
	}

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			return errors.New("invalid argument order: place flags before path")
		}
	}

	return errors.New("create accepts exactly one path argument")
}

func resolveTargetPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("path argument cannot be empty")
	}

	if filepath.IsAbs(name) {
		return "", errors.New("path argument must be relative to current working directory")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	targetPath := filepath.Clean(filepath.Join(cwd, name))
	return targetPath, nil
}

func ensureWritableTarget(targetPath string, name string) error {
	if name == "." {
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return err
		}

		if len(entries) > 0 {
			return errors.New("directory is not empty")
		}

		return nil
	}

	_, err := os.Stat(targetPath)
	if err == nil {
		return errors.New("directory already exists")
	}

	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	parentPath := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return err
	}

	return nil
}

func cloneTemplate(source templates.TemplateSource, targetPath string) error {
	if source.Subdir == "" {
		args := []string{"clone", "--depth", "1"}
		if source.Ref != "" {
			args = append(args, "--branch", source.Ref)
		}
		args = append(args, source.RepoURL, targetPath)

		cloneCmd := exec.Command("git", args...)
		output, err := cloneCmd.CombinedOutput()
		if err != nil {
			return commandError("template download failed", err, output)
		}

		return nil
	}

	tempDir, err := os.MkdirTemp("", "hackctl-template-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	args := []string{"clone", "--depth", "1"}
	if source.Ref != "" {
		args = append(args, "--branch", source.Ref)
	}
	args = append(args, source.RepoURL, tempDir)

	cloneCmd := exec.Command("git", args...)
	output, err := cloneCmd.CombinedOutput()
	if err != nil {
		return commandError("template download failed", err, output)
	}

	templatePath := filepath.Join(tempDir, filepath.FromSlash(source.Subdir))
	templateInfo, err := os.Stat(templatePath)
	if err != nil {
		return errors.New("invalid template source")
	}
	if !templateInfo.IsDir() {
		return errors.New("invalid template source")
	}

	if err := copyDirectory(templatePath, targetPath); err != nil {
		return errors.New("template copy failed")
	}

	return nil
}

type dependencyTarget struct {
	name string
	path string
}

func dependencyInstallTargets(targetPath string) ([]dependencyTarget, error) {
	cfg, err := config.LoadProjectConfig(targetPath)
	if err != nil {
		return nil, err
	}

	targets := make([]dependencyTarget, 0, len(cfg.Services))
	for _, service := range cfg.Services {
		servicePath := filepath.Join(targetPath, filepath.FromSlash(service.CWD))
		pkgJSON := filepath.Join(servicePath, "package.json")
		if _, statErr := os.Stat(pkgJSON); statErr != nil {
			continue
		}

		serviceName := strings.TrimSpace(service.Name)
		if serviceName == "" {
			serviceName = filepath.Base(servicePath)
		}

		targets = append(targets, dependencyTarget{name: serviceName, path: servicePath})
	}

	if len(targets) > 0 {
		return targets, nil
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	fallbackTargets := make([]dependencyTarget, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		servicePath := filepath.Join(targetPath, entry.Name())
		pkgJSON := filepath.Join(servicePath, "package.json")
		if _, statErr := os.Stat(pkgJSON); statErr != nil {
			continue
		}

		fallbackTargets = append(fallbackTargets, dependencyTarget{name: entry.Name(), path: servicePath})
	}

	return fallbackTargets, nil
}

func installDependencies(target dependencyTarget) error {
	installCmd := exec.Command("npm", "install", "--silent", "--no-audit", "--no-fund")
	installCmd.Dir = target.path
	output, err := installCmd.CombinedOutput()
	if err != nil {
		return commandError(fmt.Sprintf("dependency install failed for %s", target.name), err, output)
	}

	return nil
}

func ensureGitignoreEntry(projectPath string, entry string) error {
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	body, err := os.ReadFile(gitignorePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.WriteFile(gitignorePath, []byte(entry+"\n"), 0o644)
		}
		return err
	}

	content := string(body)
	if hasGitignoreEntry(content, entry) {
		return nil
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

func hasGitignoreEntry(content string, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}

	return false
}

func copyDirectory(src string, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileInfo.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		return nil
	})
}
