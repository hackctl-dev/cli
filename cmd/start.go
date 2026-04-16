package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hackctl/hackctl/cli/internal/config"
	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/spf13/cobra"
)

type runningService struct {
	name string
	cmd  *exec.Cmd
	tail *lineTail
}

type serviceExit struct {
	name string
	err  error
}

type serviceStartResult struct {
	name      string
	port      int
	service   runningService
	waitCh    <-chan error
	err       error
	reason    string
	detail    string
	cancelled bool
}

type startServiceStatus string

const (
	statusPending    startServiceStatus = "pending"
	statusInstalling startServiceStatus = "installing"
	statusStarting   startServiceStatus = "starting"
	statusRunning    startServiceStatus = "running"
	statusStopping   startServiceStatus = "stopping"
	statusStopped    startServiceStatus = "stopped"
	statusFailed     startServiceStatus = "failed"
)

type startServiceRow struct {
	Name   string
	Status startServiceStatus
	Detail string
}

type startServiceUpdateMsg struct {
	Name   string
	Status startServiceStatus
	Detail string
}

type startFooterMsg struct {
	Text string
}

type startStoppedMsg struct{}

type startFailedMsg struct {
	Message string
}

type startModel struct {
	spinner     spinner.Model
	startup     output.StartupAnimation
	rows        []startServiceRow
	indexByName map[string]int
	footer      string
	stopping    bool
	done        bool
	result      error
	cancel      context.CancelFunc
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start project services",
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath, err := os.Getwd()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		model := newStartModel(cancel)
		program := tea.NewProgram(model, tea.WithoutSignalHandler())

		go runStartWorkflow(ctx, rootPath, program.Send)

		finalModel, err := program.Run()
		if err != nil {
			return err
		}

		final := finalModel.(startModel)
		if final.result != nil {
			return final.result
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func newStartModel(cancel context.CancelFunc) startModel {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))

	return startModel{
		spinner:     spin,
		startup:     output.NewStartupAnimation(),
		rows:        []startServiceRow{},
		indexByName: make(map[string]int),
		footer:      "Starting services...",
		cancel:      cancel,
	}
}

func (m startModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startup.Init())
}

func (m startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case output.StartupAnimationTickMsg:
		var cmd tea.Cmd
		m.startup, cmd = m.startup.Update(msg)
		return m, cmd
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case startServiceUpdateMsg:
		idx, ok := m.indexByName[msg.Name]
		if ok {
			m.rows[idx].Status = msg.Status
			m.rows[idx].Detail = msg.Detail
		} else {
			m.indexByName[msg.Name] = len(m.rows)
			m.rows = append(m.rows, startServiceRow{
				Name:   msg.Name,
				Status: msg.Status,
				Detail: msg.Detail,
			})
		}
		return m, nil
	case startFooterMsg:
		m.footer = msg.Text
		return m, nil
	case startStoppedMsg:
		m.footer = "Services stopped"
		m.done = true
		return m, tea.Quit
	case startFailedMsg:
		if msg.Message == "" {
			m.result = errors.New("start failed")
		} else {
			m.result = errors.New(msg.Message)
		}
		m.done = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if !m.stopping {
				m.stopping = true
				m.footer = "Stopping services..."
				if m.cancel != nil {
					m.cancel()
				}
			}
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m startModel) View() string {
	var builder strings.Builder

	if m.startup.Visible() {
		builder.WriteString("\n")
		if m.done {
			builder.WriteString(m.startup.ResolvedView())
		} else {
			builder.WriteString(m.startup.View())
		}
		builder.WriteString("\n\n")
	}

	for _, row := range m.rows {
		line := m.rowIcon(row.Status) + " " + displayServiceName(row.Name)
		if row.Detail != "" {
			line += ": " + row.Detail
		}

		if row.Status == statusFailed {
			builder.WriteString(output.Error(line))
		} else {
			builder.WriteString(line)
		}
		builder.WriteString("\n")
	}

	if m.result != nil {
		if len(m.rows) > 0 {
			builder.WriteString("\n")
		}
		return builder.String()
	}

	if len(m.rows) > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString(output.Footer(m.footer))
	builder.WriteString("\n")

	return builder.String()
}

func displayServiceName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Service"
	}

	runes := []rune(trimmed)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func (m startModel) rowIcon(status startServiceStatus) string {
	switch status {
	case statusInstalling, statusStarting, statusStopping:
		return m.spinner.View()
	case statusRunning:
		return output.OK(output.SuccessIcon())
	case statusFailed:
		return output.FailureIcon()
	case statusStopped:
		return output.Warn("•")
	default:
		return output.Warn("·")
	}
}

func runStartWorkflow(ctx context.Context, rootPath string, send func(msg tea.Msg)) {
	projectConfig, err := config.LoadProjectConfig(rootPath)
	if err != nil {
		send(startFailedMsg{Message: err.Error()})
		return
	}

	if err := ensureDependencies(depNode, depNPM); err != nil {
		send(startFailedMsg{Message: err.Error()})
		return
	}

	state, err := config.LoadRuntimeState(rootPath)
	if err != nil {
		send(startFailedMsg{Message: "could not load runtime state"})
		return
	}

	state.Mode = "local"
	for _, svc := range projectConfig.Services {
		state.Services[svc.Name] = config.ServiceState{PID: 0, Status: "starting"}
		send(startServiceUpdateMsg{Name: svc.Name, Status: statusPending, Detail: "waiting"})
	}
	if err := config.SaveRuntimeState(rootPath, state); err != nil {
		send(startFailedMsg{Message: "could not save runtime state"})
		return
	}

	startCtx, startCancel := context.WithCancel(context.Background())
	defer startCancel()

	serviceResults := make(chan serviceStartResult, len(projectConfig.Services))
	for _, svc := range projectConfig.Services {
		svc := svc
		go func() {
			serviceResults <- startService(startCtx, rootPath, svc, send)
		}()
	}

	type startupFailure struct {
		name    string
		reason  string
		detail  string
		message string
	}

	var startupErr *startupFailure
	running := make([]runningService, 0, len(projectConfig.Services))
	runningByName := make(map[string]struct{}, len(projectConfig.Services))
	readyWaiters := make(map[string]<-chan error, len(projectConfig.Services))

	cancelled := false
	ctxDone := ctx.Done()

	for completed := 0; completed < len(projectConfig.Services); {
		select {
		case <-ctxDone:
			cancelled = true
			startCancel()
			ctxDone = nil
		case result := <-serviceResults:
			completed++

			if ctx.Err() != nil && !cancelled {
				cancelled = true
				startCancel()
				ctxDone = nil
			}

			if result.service.cmd != nil {
				if _, seen := runningByName[result.name]; !seen {
					runningByName[result.name] = struct{}{}
					running = append(running, result.service)
				}
			}

			if cancelled {
				continue
			}

			if result.err != nil {
				if result.cancelled {
					continue
				}

				if startupErr == nil {
					startupErr = &startupFailure{name: result.name, reason: result.reason, detail: result.detail}
					state.Services[result.name] = config.ServiceState{PID: 0, Status: "failed"}
					send(startServiceUpdateMsg{Name: result.name, Status: statusFailed, Detail: result.reason})
					startCancel()
				}
				continue
			}

			if cancelled || startupErr != nil {
				continue
			}

			readyWaiters[result.name] = result.waitCh
			state.Services[result.name] = config.ServiceState{PID: result.service.cmd.Process.Pid, Status: "running"}
			if err := config.SaveRuntimeState(rootPath, state); err != nil {
				startupErr = &startupFailure{message: "could not save runtime state"}
				startCancel()
				continue
			}

			detail := fmt.Sprintf("running on http://localhost:%d", result.port)
			send(startServiceUpdateMsg{Name: result.name, Status: statusRunning, Detail: detail})
		}
	}

	if cancelled {
		for _, rs := range running {
			send(startServiceUpdateMsg{Name: rs.name, Status: statusStopping, Detail: "stopping"})
		}
		gracefulStop(running)
		markStopped(&state, running)
		_ = config.SaveRuntimeState(rootPath, state)
		send(startStoppedMsg{})
		return
	}

	if startupErr != nil {
		gracefulStop(running)
		markStopped(&state, running)
		_ = config.SaveRuntimeState(rootPath, state)

		if startupErr.message != "" {
			send(startFailedMsg{Message: startupErr.message})
			return
		}

		send(startFailedMsg{Message: serviceFailureMessage(startupErr.name, startupErr.reason, startupErr.detail)})
		return
	}

	exitCh := make(chan serviceExit, len(running))
	for _, rs := range running {
		waitCh, ok := readyWaiters[rs.name]
		if !ok || waitCh == nil {
			continue
		}

		go func(name string, waitCh <-chan error) {
			exitCh <- serviceExit{name: name, err: <-waitCh}
		}(rs.name, waitCh)
	}

	send(startFooterMsg{Text: "Press Ctrl+C to stop running."})

	select {
	case <-ctx.Done():
		for _, rs := range running {
			send(startServiceUpdateMsg{Name: rs.name, Status: statusStopping, Detail: "stopping"})
		}
		gracefulStop(running)
		markStopped(&state, running)
		_ = config.SaveRuntimeState(rootPath, state)
		send(startStoppedMsg{})
		return
	case exited := <-exitCh:
		gracefulStop(running)
		state.Services[exited.name] = config.ServiceState{PID: 0, Status: "failed"}
		markStopped(&state, running)
		_ = config.SaveRuntimeState(rootPath, state)
		send(startServiceUpdateMsg{Name: exited.name, Status: statusFailed, Detail: "exited unexpectedly"})
		failureDetail := conciseError(exited.err)
		if failureDetail == "" {
			if rs, ok := findRunningService(running, exited.name); ok && rs.tail != nil {
				failureDetail = rs.tail.LastLine()
			}
		}
		send(startFailedMsg{Message: serviceFailureMessage(exited.name, "exited unexpectedly", failureDetail)})
		return
	}
}

func startService(ctx context.Context, rootPath string, svc config.ServiceConfig, send func(msg tea.Msg)) serviceStartResult {
	result := serviceStartResult{
		name: svc.Name,
		port: svc.Port,
	}

	svcDir := filepath.Join(rootPath, svc.CWD)
	if _, err := os.Stat(svcDir); err != nil {
		result.err = err
		result.reason = "folder missing"
		return result
	}

	installed, err := installDependenciesIfNeeded(ctx, svcDir)
	if err != nil {
		result.err = err
		if ctx.Err() != nil {
			result.cancelled = true
			return result
		}

		result.reason = "install failed"
		result.detail = conciseError(err)
		return result
	}

	if installed {
		send(startServiceUpdateMsg{Name: svc.Name, Status: statusInstalling, Detail: "dependencies installed"})
	} else {
		send(startServiceUpdateMsg{Name: svc.Name, Status: statusInstalling, Detail: "dependencies ready"})
	}

	if ctx.Err() != nil {
		result.err = ctx.Err()
		result.cancelled = true
		return result
	}

	send(startServiceUpdateMsg{Name: svc.Name, Status: statusStarting, Detail: "starting"})

	serviceTail := newLineTail(10)
	serviceCmd := serviceCommand(ctx, svc.Run)
	serviceCmd.Dir = svcDir
	serviceCmd.Stdout = serviceTail
	serviceCmd.Stderr = serviceTail
	serviceCmd.Env = os.Environ()

	if svc.EnvFile != "" {
		envPath := filepath.Join(rootPath, svc.EnvFile)
		envMap, err := config.LoadEnvFile(envPath)
		if err != nil {
			result.err = err
			result.reason = "env file invalid"
			result.detail = conciseError(err)
			return result
		}
		serviceCmd.Env = append(serviceCmd.Env, envPairs(envMap)...)
	}

	if err := serviceCmd.Start(); err != nil {
		result.err = err
		if ctx.Err() != nil {
			result.cancelled = true
			return result
		}

		result.reason = "start failed"
		result.detail = conciseError(err)
		return result
	}

	result.service = runningService{name: svc.Name, cmd: serviceCmd, tail: serviceTail}

	serviceWaitCh := make(chan error, 1)
	go func(cmd *exec.Cmd, waitCh chan<- error) {
		waitCh <- cmd.Wait()
	}(serviceCmd, serviceWaitCh)

	if err := waitForServiceReady(ctx, svc.Port, serviceWaitCh, 20*time.Second); err != nil {
		result.err = err
		if ctx.Err() != nil {
			result.cancelled = true
			return result
		}

		result.reason = "not ready"
		result.detail = serviceTail.LastLine()
		if result.detail == "" {
			result.detail = conciseError(err)
		}
		return result
	}

	result.waitCh = serviceWaitCh
	return result
}

func installDependenciesIfNeeded(ctx context.Context, serviceDir string) (bool, error) {
	packageJSON := filepath.Join(serviceDir, "package.json")
	if _, err := os.Stat(packageJSON); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	nodeModules := filepath.Join(serviceDir, "node_modules")
	if _, err := os.Stat(nodeModules); err == nil {
		return false, nil
	}

	installCmd := exec.CommandContext(ctx, "npm", "install", "--silent", "--no-audit", "--no-fund")
	installCmd.Dir = serviceDir
	output, err := installCmd.CombinedOutput()
	if err != nil {
		return true, commandError("dependency install failed", err, output)
	}

	return true, nil
}

func waitForServiceReady(ctx context.Context, port int, waitCh <-chan error, timeout time.Duration) error {
	if port <= 0 {
		return errors.New("invalid port")
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if isPortReachable(port, 300*time.Millisecond) {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("start cancelled")
		case err := <-waitCh:
			if err != nil {
				return withDetail("service exited before ready", err.Error())
			}
			return errors.New("service exited before ready")
		case <-timer.C:
			return errors.New("readiness timeout")
		case <-ticker.C:
		}
	}
}

func serviceFailureMessage(service string, reason string, detail string) string {
	trimmedService := strings.TrimSpace(service)
	if trimmedService == "" {
		trimmedService = "service"
	}

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "failed"
	}

	message := trimmedService + " " + trimmedReason
	if shortDetail := conciseText(detail); shortDetail != "" {
		message += ": " + shortDetail
	}

	return message
}

func findRunningService(services []runningService, name string) (runningService, bool) {
	for _, service := range services {
		if service.name == name {
			return service, true
		}
	}

	return runningService{}, false
}

func serviceCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}

	return exec.CommandContext(ctx, "sh", "-c", command)
}

func gracefulStop(services []runningService) {
	if runtime.GOOS == "windows" {
		for _, svc := range services {
			if svc.cmd == nil || svc.cmd.Process == nil {
				continue
			}

			taskkillCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(svc.cmd.Process.Pid))
			taskkillCmd.Stdout = io.Discard
			taskkillCmd.Stderr = io.Discard
			if err := taskkillCmd.Run(); err != nil {
				_ = svc.cmd.Process.Kill()
			}
		}
		return
	}

	for _, svc := range services {
		if svc.cmd == nil || svc.cmd.Process == nil {
			continue
		}
		_ = svc.cmd.Process.Signal(os.Interrupt)
	}
	time.Sleep(700 * time.Millisecond)

	for _, svc := range services {
		if svc.cmd == nil || svc.cmd.Process == nil {
			continue
		}
		_ = svc.cmd.Process.Kill()
	}
}

func markStopped(state *config.RuntimeState, services []runningService) {
	for _, svc := range services {
		if current, ok := state.Services[svc.name]; ok {
			if current.Status == "failed" || current.Status == "exited" {
				continue
			}
		}
		state.Services[svc.name] = config.ServiceState{PID: 0, Status: "stopped"}
	}
}

func envPairs(values map[string]string) []string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return pairs
}
