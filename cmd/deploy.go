package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hackctl/hackctl/cli/internal/config"
	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	deployTarget string
	deployKey    string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the current project to a VPS",
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath, err := os.Getwd()
		if err != nil {
			return err
		}

		projectConfig, err := config.LoadProjectConfig(rootPath)
		if err != nil {
			return err
		}

		deployConfig, err := config.ValidateDeployConfig(projectConfig)
		if err != nil {
			return err
		}

		target, keyPath, err := resolveDeployInputs(rootPath, deployTarget, deployKey)
		if err != nil {
			return err
		}

		publicService, publicPort, err := resolveShareTarget(projectConfig)
		if err != nil {
			return err
		}

		plan, err := newDeployPlan(projectConfig, deployConfig, target, keyPath, publicService, publicPort)
		if err != nil {
			return err
		}

		publicURL := ""
		tunnelPID := 0

		if err := output.RunSteps("Deploying project", func(addStep func(string) int, completeStep func(int)) error {
			stepID := addStep("Validating local prerequisites")
			if err := ensureDependencies(depSSH, depSCP); err != nil {
				return err
			}
			if err := validateDeployKeyPath(plan.KeyPath); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep(fmt.Sprintf("Connecting to %s", plan.Target.String()))
			if err := checkRemoteConnection(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Provisioning pm2 runtime")
			if err := provisionRemoteRuntime(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Preparing remote project directory")
			if err := prepareRemoteProject(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Uploading project files")
			if err := syncProjectFiles(rootPath, plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Installing dependencies")
			if err := installRemoteDependencies(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Starting services")
			if err := startRemoteServices(plan); err != nil {
				return err
			}
			completeStep(stepID)

			stepID = addStep("Creating Cloudflare tunnel")
			url, pid, err := startRemoteTunnel(plan)
			if err != nil {
				return err
			}
			publicURL = url
			tunnelPID = pid
			completeStep(stepID)

			stepID = addStep("Saving deploy state")
			state := config.DeployState{
				Target:         plan.Target.String(),
				KeyPath:        plan.KeyPath,
				Runtime:        plan.Runtime,
				Mode:           plan.Mode,
				RemoteRoot:     plan.RemoteRoot,
				RemoteStateDir: plan.RemoteStateDir,
				PublicService:  plan.PublicService,
				PublicPort:     plan.PublicPort,
				PublicURL:      publicURL,
				TunnelPID:      tunnelPID,
				Services:       plan.deployServices(),
			}
			if err := config.SaveDeployState(rootPath, state); err != nil {
				return withDetail("deploy state write failed", err.Error())
			}
			completeStep(stepID)

			return nil
		}); err != nil {
			return silent(err)
		}

		fmt.Println("Project successfully deployed")
		fmt.Println(output.Field("Live URL", output.URL(publicURL)))
		fmt.Println()
		fmt.Println(output.Footer("Run hackctl status to view deploy details"))

		return nil
	},
}

type parsedDeployTarget struct {
	User string
	Host string
}

func (t parsedDeployTarget) String() string {
	return t.User + "@" + t.Host
}

type deployPlan struct {
	Target         parsedDeployTarget
	KeyPath        string
	Runtime        string
	Mode           string
	ProjectName    string
	ProjectSlug    string
	RemoteParent   string
	RemoteRoot     string
	RemoteStateDir string
	PublicService  string
	PublicPort     int
	Services       []deployServicePlan
}

type deployServicePlan struct {
	Name        string
	RemoteDir   string
	Port        int
	ProcessName string
	RunArgs     []string
}

func init() {
	deployCmd.Flags().StringVarP(&deployTarget, "target", "t", "", "SSH target in the form user@host")
	deployCmd.Flags().StringVarP(&deployKey, "key", "k", "", "Path to the SSH private key")

	rootCmd.AddCommand(deployCmd)
}

func parseDeployTarget(value string) (parsedDeployTarget, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return parsedDeployTarget{}, errors.New("missing deploy target")
	}

	user, host, ok := strings.Cut(trimmed, "@")
	if !ok || strings.TrimSpace(user) == "" || strings.TrimSpace(host) == "" {
		return parsedDeployTarget{}, errors.New("target must use the form user@host")
	}

	if strings.ContainsAny(trimmed, " \t\r\n") {
		return parsedDeployTarget{}, errors.New("target must not contain spaces")
	}

	return parsedDeployTarget{User: strings.TrimSpace(user), Host: strings.TrimSpace(host)}, nil
}

func resolveDeployInputs(rootPath string, targetFlag string, keyFlag string) (string, string, error) {
	target := strings.TrimSpace(targetFlag)
	keyPath := strings.TrimSpace(keyFlag)

	savedState, err := config.LoadDeployState(rootPath)
	if err != nil && !errors.Is(err, config.ErrDeployStateNotFound) {
		if target == "" || keyPath == "" {
			return "", "", err
		}
	} else if err == nil {
		if target == "" {
			target = strings.TrimSpace(savedState.Target)
		}
		if keyPath == "" {
			keyPath = strings.TrimSpace(savedState.KeyPath)
		}
	}

	if target == "" {
		return "", "", errors.New("missing deploy target (use --target user@host)")
	}

	if keyPath == "" {
		return "", "", errors.New("missing deploy key (use --key <path>)")
	}

	absoluteKeyPath, err := filepath.Abs(filepath.Clean(keyPath))
	if err != nil {
		return "", "", err
	}

	return target, absoluteKeyPath, nil
}

func validateDeployKeyPath(keyPath string) error {
	info, err := os.Stat(keyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("SSH key file not found")
		}
		return err
	}

	if info.IsDir() {
		return errors.New("SSH key path must be a file")
	}

	return nil
}

func newDeployPlan(projectConfig config.ProjectConfig, deployConfig config.DeployConfig, target string, keyPath string, publicService string, publicPort int) (deployPlan, error) {
	parsedTarget, err := parseDeployTarget(target)
	if err != nil {
		return deployPlan{}, err
	}

	projectSlug := safeDeploySlug(projectConfig.Name)
	if projectSlug == "" {
		projectSlug = "hackctl-app"
	}

	services := make([]deployServicePlan, 0, len(projectConfig.Services))
	for _, service := range projectConfig.Services {
		runArgs, err := parseDeployRunArgs(service.Run)
		if err != nil {
			return deployPlan{}, fmt.Errorf("service %s: %w", service.Name, err)
		}

		remoteDir := "~/hackctl/" + projectSlug
		if cleaned := strings.TrimSpace(filepath.ToSlash(service.CWD)); cleaned != "" && cleaned != "." {
			remoteDir += "/" + cleaned
		}

		services = append(services, deployServicePlan{
			Name:        service.Name,
			RemoteDir:   remoteDir,
			Port:        service.Port,
			ProcessName: fmt.Sprintf("hackctl-%s-%s", projectSlug, safeDeploySlug(service.Name)),
			RunArgs:     runArgs,
		})
	}

	return deployPlan{
		Target:         parsedTarget,
		KeyPath:        keyPath,
		Runtime:        deployConfig.Runtime,
		Mode:           deployConfig.Mode,
		ProjectName:    projectConfig.Name,
		ProjectSlug:    projectSlug,
		RemoteParent:   "~/hackctl",
		RemoteRoot:     "~/hackctl/" + projectSlug,
		RemoteStateDir: "~/.hackctl/deploy/" + projectSlug,
		PublicService:  publicService,
		PublicPort:     publicPort,
		Services:       services,
	}, nil
}

func (p deployPlan) deployServices() []config.DeployServiceState {
	services := make([]config.DeployServiceState, 0, len(p.Services))
	for _, service := range p.Services {
		services = append(services, config.DeployServiceState{
			Name:        service.Name,
			RemoteDir:   service.RemoteDir,
			Port:        service.Port,
			ProcessName: service.ProcessName,
		})
	}

	return services
}

func parseDeployRunArgs(command string) ([]string, error) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) < 3 {
		return nil, errors.New("deploy requires npm run commands")
	}

	if !strings.EqualFold(fields[0], "npm") && !strings.EqualFold(fields[0], "npm.cmd") {
		return nil, errors.New("deploy requires npm run commands")
	}

	if !strings.EqualFold(fields[1], "run") {
		return nil, errors.New("deploy requires npm run commands")
	}

	return fields[1:], nil
}

func safeDeploySlug(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-_")
}

func checkRemoteConnection(plan deployPlan) error {
	output, err := runRemoteScript(plan, "printf 'connected\\n'\n")
	if err != nil {
		return commandError("connection failed", err, output)
	}

	return nil
}

func provisionRemoteRuntime(plan deployPlan) error {
	output, err := runRemoteScript(plan, buildProvisionScript(plan))
	if err != nil {
		return commandError("provisioning failed", err, output)
	}

	return nil
}

func prepareRemoteProject(plan deployPlan) error {
	output, err := runRemoteScript(plan, buildRemotePrepareScript(plan))
	if err != nil {
		return commandError("project preparation failed", err, output)
	}

	return nil
}

func syncProjectFiles(rootPath string, plan deployPlan) error {
	packageDir, cleanup, err := prepareDeployPackage(rootPath, plan.ProjectSlug)
	if err != nil {
		return err
	}
	defer cleanup()

	args := append(sshTransportArgs(plan.KeyPath), "-r", packageDir, fmt.Sprintf("%s:%s/", plan.Target.String(), plan.RemoteParent))
	scpCmd := exec.Command("scp", args...)
	output, err := scpCmd.CombinedOutput()
	if err != nil {
		return commandError("upload failed", err, output)
	}

	return nil
}

func installRemoteDependencies(plan deployPlan) error {
	output, err := runRemoteScript(plan, buildDependencyInstallScript(plan))
	if err != nil {
		return commandError("dependency install failed", err, output)
	}

	return nil
}

func startRemoteServices(plan deployPlan) error {
	output, err := runRemoteScript(plan, buildServiceStartScript(plan))
	if err != nil {
		return commandError("service startup failed", err, output)
	}

	return nil
}

func startRemoteTunnel(plan deployPlan) (string, int, error) {
	output, err := runRemoteScript(plan, buildTunnelStartScript(plan))
	if err != nil {
		return "", 0, commandError("tunnel start failed", err, output)
	}

	tunnelPID := 0
	if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
		pid, parseErr := strconv.Atoi(trimmed)
		if parseErr == nil {
			tunnelPID = pid
		}
	}

	publicURL, err := waitForRemotePublicURL(plan, 25*time.Second)
	if err != nil {
		return "", 0, err
	}

	return publicURL, tunnelPID, nil
}

func waitForRemotePublicURL(plan deployPlan, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, err := runRemoteScript(plan, buildTunnelLogScript(plan))
		if err == nil {
			if match := quickTunnelURL.FindString(string(output)); match != "" {
				return match, nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return "", errors.New("cloudflare tunnel did not produce a public URL")
}

func prepareDeployPackage(rootPath string, projectSlug string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "hackctl-deploy-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	packageDir := filepath.Join(tempDir, projectSlug)
	if err := copyProjectForDeploy(rootPath, packageDir); err != nil {
		cleanup()
		return "", nil, err
	}

	return packageDir, cleanup, nil
}

func copyProjectForDeploy(src string, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	skippedDirs := map[string]struct{}{
		".git":         {},
		".hackctl":     {},
		"node_modules": {},
		"dist":         {},
		".next":        {},
		".nuxt":        {},
		".output":      {},
		".svelte-kit":  {},
		"coverage":     {},
		".turbo":       {},
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

		name := strings.ToLower(strings.TrimSpace(d.Name()))
		if d.IsDir() {
			if _, shouldSkip := skippedDirs[name]; shouldSkip {
				return filepath.SkipDir
			}
		}

		if name == ".ds_store" || name == "thumbs.db" || strings.HasSuffix(name, ".log") {
			return nil
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

func buildProvisionScript(plan deployPlan) string {
	var script strings.Builder
	script.WriteString("set -eu\n")
	script.WriteString("if [ \"$(uname -s)\" != \"Linux\" ]; then\n")
	script.WriteString("  echo 'remote host must be Linux'\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n")
	script.WriteString("if ! command -v apt-get >/dev/null 2>&1; then\n")
	script.WriteString("  echo 'remote host must use apt-get (Ubuntu or Debian)'\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n")
	script.WriteString("if [ \"$(id -u)\" -eq 0 ]; then\n")
	script.WriteString("  SUDO=''\n")
	script.WriteString("elif command -v sudo >/dev/null 2>&1; then\n")
	script.WriteString("  SUDO='sudo'\n")
	script.WriteString("else\n")
	script.WriteString("  echo 'remote host must provide sudo for package installation'\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n")
	script.WriteString("run_root() {\n")
	script.WriteString("  if [ -n \"$SUDO\" ]; then\n")
	script.WriteString("    \"$SUDO\" \"$@\"\n")
	script.WriteString("  else\n")
	script.WriteString("    \"$@\"\n")
	script.WriteString("  fi\n")
	script.WriteString("}\n")
	script.WriteString("missing_packages() {\n")
	script.WriteString("  MISSING=''\n")
	script.WriteString("  for PKG in \"$@\"; do\n")
	script.WriteString("    if ! dpkg -s \"$PKG\" >/dev/null 2>&1; then\n")
	script.WriteString("      MISSING=\"$MISSING $PKG\"\n")
	script.WriteString("    fi\n")
	script.WriteString("  done\n")
	script.WriteString("  printf '%s\\n' \"${MISSING# }\"\n")
	script.WriteString("}\n")
	script.WriteString("export DEBIAN_FRONTEND=noninteractive\n")
	script.WriteString("BASE_PACKAGES=$(missing_packages ca-certificates curl gnupg)\n")
	script.WriteString("if [ -n \"$BASE_PACKAGES\" ]; then\n")
	script.WriteString("  run_root apt-get update >/dev/null\n")
	script.WriteString("  run_root apt-get install -y $BASE_PACKAGES >/dev/null\n")
	script.WriteString("fi\n")
	script.WriteString("if ! command -v node >/dev/null 2>&1 || ! node -e \"process.exit(Number(process.versions.node.split('.')[0]) >= 20 ? 0 : 1)\"; then\n")
	script.WriteString("  run_root mkdir -p /etc/apt/keyrings\n")
	script.WriteString("  curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | run_root gpg --dearmor --yes -o /etc/apt/keyrings/nodesource.gpg\n")
	script.WriteString("  printf '%s\\n' 'deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main' | run_root tee /etc/apt/sources.list.d/nodesource.list >/dev/null\n")
	script.WriteString("  run_root apt-get update >/dev/null\n")
	script.WriteString("  run_root apt-get install -y nodejs >/dev/null\n")
	script.WriteString("fi\n")
	script.WriteString("if ! node -e \"const {execSync}=require('child_process'); const major=Number(execSync('npm --version').toString().trim().split('.')[0]); process.exit(major >= 10 ? 0 : 1)\"; then\n")
	script.WriteString("  run_root npm install -g npm@10 >/dev/null\n")
	script.WriteString("fi\n")
	script.WriteString("if ! command -v pm2 >/dev/null 2>&1; then\n")
	script.WriteString("  run_root npm install -g pm2 >/dev/null\n")
	script.WriteString("fi\n")
	script.WriteString("if ! command -v cloudflared >/dev/null 2>&1; then\n")
	script.WriteString("  ARCH=$(uname -m)\n")
	script.WriteString("  case \"$ARCH\" in\n")
	script.WriteString("    x86_64|amd64) DOWNLOAD_URL='https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64' ;;\n")
	script.WriteString("    aarch64|arm64) DOWNLOAD_URL='https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64' ;;\n")
	script.WriteString("    *) echo \"unsupported remote architecture $ARCH\"; exit 1 ;;\n")
	script.WriteString("  esac\n")
	script.WriteString("  TMP_PATH=$(mktemp)\n")
	script.WriteString("  curl -fsSL \"$DOWNLOAD_URL\" -o \"$TMP_PATH\"\n")
	script.WriteString("  chmod +x \"$TMP_PATH\"\n")
	script.WriteString("  run_root mv \"$TMP_PATH\" /usr/local/bin/cloudflared\n")
	script.WriteString("fi\n")
	script.WriteString("mkdir -p \"$HOME/hackctl\" \"$HOME/.hackctl/deploy\"\n")
	_ = plan
	return script.String()
}

func buildRemotePrepareScript(plan deployPlan) string {
	var script strings.Builder
	script.WriteString("set -eu\n")
	for _, service := range plan.Services {
		script.WriteString("pm2 delete ")
		script.WriteString(shellQuote(service.ProcessName))
		script.WriteString(" >/dev/null 2>&1 || true\n")
	}
	script.WriteString("if [ -f ")
	script.WriteString(shellPath(remotePath(plan.RemoteStateDir, "cloudflared.pid")))
	script.WriteString(" ]; then\n")
	script.WriteString("  kill \"$(cat ")
	script.WriteString(shellPath(remotePath(plan.RemoteStateDir, "cloudflared.pid")))
	script.WriteString(")\" >/dev/null 2>&1 || true\n")
	script.WriteString("fi\n")
	script.WriteString("rm -rf ")
	script.WriteString(shellPath(plan.RemoteRoot))
	script.WriteString(" ")
	script.WriteString(shellPath(plan.RemoteStateDir))
	script.WriteString("\n")
	script.WriteString("mkdir -p ")
	script.WriteString(shellPath(plan.RemoteParent))
	script.WriteString(" ")
	script.WriteString(shellPath(plan.RemoteStateDir))
	script.WriteString("\n")
	return script.String()
}

func buildDependencyInstallScript(plan deployPlan) string {
	var script strings.Builder
	script.WriteString("set -eu\n")
	for _, service := range plan.Services {
		script.WriteString("cd ")
		script.WriteString(shellPath(service.RemoteDir))
		script.WriteString("\n")
		script.WriteString("if [ -f package-lock.json ]; then\n")
		script.WriteString("  npm ci --silent --no-audit --no-fund >/dev/null\n")
		script.WriteString("else\n")
		script.WriteString("  npm install --silent --no-audit --no-fund >/dev/null\n")
		script.WriteString("fi\n")
	}
	return script.String()
}

func buildServiceStartScript(plan deployPlan) string {
	var script strings.Builder
	script.WriteString("set -eu\n")
	script.WriteString("wait_for_port() {\n")
	script.WriteString("  PORT=\"$1\"\n")
	script.WriteString("  ATTEMPTS=20\n")
	script.WriteString("  while [ \"$ATTEMPTS\" -gt 0 ]; do\n")
	script.WriteString("    if node -e \"const net=require('net'); const port=Number(process.argv[1]); const socket=net.connect({host:'127.0.0.1', port, timeout:500}, ()=>{socket.end(); process.exit(0)}); socket.on('error', ()=>process.exit(1)); socket.on('timeout', ()=>{socket.destroy(); process.exit(1)});\" \"$PORT\"; then\n")
	script.WriteString("      return 0\n")
	script.WriteString("    fi\n")
	script.WriteString("    ATTEMPTS=$((ATTEMPTS - 1))\n")
	script.WriteString("    sleep 1\n")
	script.WriteString("  done\n")
	script.WriteString("  echo \"service on port $PORT did not become ready\"\n")
	script.WriteString("  return 1\n")
	script.WriteString("}\n")
	for _, service := range plan.Services {
		script.WriteString("pm2 delete ")
		script.WriteString(shellQuote(service.ProcessName))
		script.WriteString(" >/dev/null 2>&1 || true\n")
		script.WriteString("pm2 start npm --name ")
		script.WriteString(shellQuote(service.ProcessName))
		script.WriteString(" --cwd ")
		script.WriteString(shellPath(service.RemoteDir))
		script.WriteString(" --")
		for _, arg := range service.RunArgs {
			script.WriteString(" ")
			script.WriteString(shellQuote(arg))
		}
		script.WriteString(" >/dev/null\n")
		script.WriteString("wait_for_port ")
		script.WriteString(strconv.Itoa(service.Port))
		script.WriteString("\n")
	}
	return script.String()
}

func buildTunnelStartScript(plan deployPlan) string {
	logPath := remotePath(plan.RemoteStateDir, "cloudflared.log")
	pidPath := remotePath(plan.RemoteStateDir, "cloudflared.pid")

	var script strings.Builder
	script.WriteString("set -eu\n")
	script.WriteString("rm -f ")
	script.WriteString(shellPath(logPath))
	script.WriteString(" ")
	script.WriteString(shellPath(pidPath))
	script.WriteString("\n")
	script.WriteString("nohup cloudflared tunnel --url ")
	script.WriteString(shellQuote(fmt.Sprintf("http://127.0.0.1:%d", plan.PublicPort)))
	script.WriteString(" --no-autoupdate >")
	script.WriteString(shellPath(logPath))
	script.WriteString(" 2>&1 &\n")
	script.WriteString("echo $! > ")
	script.WriteString(shellPath(pidPath))
	script.WriteString("\n")
	script.WriteString("cat ")
	script.WriteString(shellPath(pidPath))
	script.WriteString("\n")
	return script.String()
}

func buildTunnelLogScript(plan deployPlan) string {
	logPath := remotePath(plan.RemoteStateDir, "cloudflared.log")
	return "set -eu\nif [ -f " + shellPath(logPath) + " ]; then\n  cat " + shellPath(logPath) + "\nfi\n"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellPath(value string) string {
	expanded := expandRemoteHome(value)
	if strings.HasPrefix(expanded, "$HOME/") {
		return "\"$HOME/" + strings.TrimPrefix(expanded, "$HOME/") + "\""
	}

	return shellQuote(expanded)
}

func expandRemoteHome(value string) string {
	if strings.HasPrefix(value, "~/") {
		return "$HOME/" + strings.TrimPrefix(value, "~/")
	}
	return value
}

func remotePath(base string, name string) string {
	trimmed := strings.TrimRight(expandRemoteHome(base), "/")
	return trimmed + "/" + name
}

func sshTransportArgs(keyPath string) []string {
	return []string{
		"-i", keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
}

func runRemoteScript(plan deployPlan, script string) ([]byte, error) {
	args := append(sshTransportArgs(plan.KeyPath), plan.Target.String(), "sh", "-se")
	sshCmd := exec.Command("ssh", args...)
	sshCmd.Stdin = strings.NewReader(script)
	return sshCmd.CombinedOutput()
}
