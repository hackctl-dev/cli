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
}

type serviceExit struct {
	name string
	err  error
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
	rows        []startServiceRow
	indexByName map[string]int
	footer      string
	stopping    bool
	cancel      context.CancelFunc
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start project services",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(output.ASCIIBanner())

		rootPath, err := os.Getwd()
		if err != nil {
			return err
		}

		projectConfig, err := config.LoadProjectConfig(rootPath)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		model := newStartModel(projectConfig.Services, cancel)
		program := tea.NewProgram(model, tea.WithoutSignalHandler())

		go runStartWorkflow(ctx, rootPath, projectConfig, program.Send)

		if _, err := program.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func newStartModel(services []config.ServiceConfig, cancel context.CancelFunc) startModel {
	rows := make([]startServiceRow, 0, len(services))
	indexByName := make(map[string]int, len(services))

	for i, svc := range services {
		rows = append(rows, startServiceRow{
			Name:   svc.Name,
			Status: statusPending,
			Detail: "waiting",
		})
		indexByName[svc.Name] = i
	}

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))

	return startModel{
		spinner:     spin,
		rows:        rows,
		indexByName: indexByName,
		footer:      "Starting services...",
		cancel:      cancel,
	}
}

func (m startModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case startServiceUpdateMsg:
		idx, ok := m.indexByName[msg.Name]
		if ok {
			m.rows[idx].Status = msg.Status
			m.rows[idx].Detail = msg.Detail
		}
		return m, nil
	case startFooterMsg:
		m.footer = msg.Text
		return m, nil
	case startStoppedMsg:
		m.footer = "Services stopped"
		return m, tea.Quit
	case startFailedMsg:
		if msg.Message == "" {
			m.footer = "Start failed"
		} else {
			m.footer = "Start failed: " + msg.Message
		}
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

	builder.WriteString("\n")
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

func runStartWorkflow(ctx context.Context, rootPath string, projectConfig config.ProjectConfig, send func(msg tea.Msg)) {
	if err := ensureNodeAndNPM(); err != nil {
		send(startFailedMsg{Message: "node and npm are required"})
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

	running := make([]runningService, 0, len(projectConfig.Services))
	exitCh := make(chan serviceExit, len(projectConfig.Services))

	for _, svc := range projectConfig.Services {
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
		default:
		}

		svcDir := filepath.Join(rootPath, svc.CWD)
		if _, err := os.Stat(svcDir); err != nil {
			gracefulStop(running)
			state.Services[svc.Name] = config.ServiceState{PID: 0, Status: "failed"}
			markStopped(&state, running)
			_ = config.SaveRuntimeState(rootPath, state)
			send(startServiceUpdateMsg{Name: svc.Name, Status: statusFailed, Detail: "folder missing"})
			send(startFailedMsg{})
			return
		}

		installed, err := installDependenciesIfNeeded(ctx, svcDir)
		if err != nil {
			gracefulStop(running)
			state.Services[svc.Name] = config.ServiceState{PID: 0, Status: "failed"}
			markStopped(&state, running)
			_ = config.SaveRuntimeState(rootPath, state)
			send(startServiceUpdateMsg{Name: svc.Name, Status: statusFailed, Detail: "install failed"})
			send(startFailedMsg{})
			return
		}
		if installed {
			send(startServiceUpdateMsg{Name: svc.Name, Status: statusInstalling, Detail: "dependencies installed"})
		} else {
			send(startServiceUpdateMsg{Name: svc.Name, Status: statusInstalling, Detail: "dependencies ready"})
		}

		send(startServiceUpdateMsg{Name: svc.Name, Status: statusStarting, Detail: "starting"})

		serviceCmd := serviceCommand(svc.Run)
		serviceCmd.Dir = svcDir
		serviceCmd.Stdout = io.Discard
		serviceCmd.Stderr = io.Discard
		serviceCmd.Env = os.Environ()

		if svc.EnvFile != "" {
			envPath := filepath.Join(rootPath, svc.EnvFile)
			envMap, err := config.LoadEnvFile(envPath)
			if err != nil {
				gracefulStop(running)
				state.Services[svc.Name] = config.ServiceState{PID: 0, Status: "failed"}
				markStopped(&state, running)
				_ = config.SaveRuntimeState(rootPath, state)
				send(startServiceUpdateMsg{Name: svc.Name, Status: statusFailed, Detail: "env file invalid"})
				send(startFailedMsg{})
				return
			}
			serviceCmd.Env = append(serviceCmd.Env, envPairs(envMap)...)
		}

		if err := serviceCmd.Start(); err != nil {
			gracefulStop(running)
			state.Services[svc.Name] = config.ServiceState{PID: 0, Status: "failed"}
			markStopped(&state, running)
			_ = config.SaveRuntimeState(rootPath, state)
			send(startServiceUpdateMsg{Name: svc.Name, Status: statusFailed, Detail: "start failed"})
			send(startFailedMsg{})
			return
		}

		running = append(running, runningService{name: svc.Name, cmd: serviceCmd})
		state.Services[svc.Name] = config.ServiceState{PID: serviceCmd.Process.Pid, Status: "running"}
		if err := config.SaveRuntimeState(rootPath, state); err != nil {
			gracefulStop(running)
			markStopped(&state, running)
			_ = config.SaveRuntimeState(rootPath, state)
			send(startFailedMsg{Message: "could not save runtime state"})
			return
		}

		detail := fmt.Sprintf("running on http://localhost:%d", svc.Port)
		send(startServiceUpdateMsg{Name: svc.Name, Status: statusRunning, Detail: detail})

		go func(name string, cmd *exec.Cmd) {
			exitCh <- serviceExit{name: name, err: cmd.Wait()}
		}(svc.Name, serviceCmd)
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
		send(startFailedMsg{})
		return
	}
}

func ensureNodeAndNPM() error {
	if _, err := exec.LookPath("node"); err != nil {
		return err
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return err
	}
	return nil
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
	installCmd.Stdout = io.Discard
	installCmd.Stderr = io.Discard

	if err := installCmd.Run(); err != nil {
		return true, err
	}

	return true, nil
}

func serviceCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}

	return exec.Command("sh", "-c", command)
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
