package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/hackctl/hackctl/cli/internal/config"
	"github.com/hackctl/hackctl/cli/internal/output"
	"github.com/spf13/cobra"
)

var quickTunnelURL = regexp.MustCompile(`https://[a-zA-Z0-9.-]+\.trycloudflare\.com`)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Share a running project publicly",
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			rootPath string
			port     int
		)

		var cloudflaredPath string
		var tunnelCmd *exec.Cmd
		var waitCh chan error
		var publicURL string
		state := config.RuntimeState{}
		stateLoaded := false
		tunnelOutput := newLineTail(20)

		if err := output.RunSteps("Sharing project", func(addStep func(string) int, completeStep func(int)) error {
			var err error
			rootPath, err = os.Getwd()
			if err != nil {
				return err
			}

			projectConfig, err := config.LoadProjectConfig(rootPath)
			if err != nil {
				return err
			}

			port, err = resolveSharePort(projectConfig)
			if err != nil {
				return err
			}

			if !isPortReachable(port, 1200*time.Millisecond) {
				return fmt.Errorf("No services running on port %d", port)
			}

			stepID := addStep("Preparing tunnel client")
			var resolveErr error
			cloudflaredPath, resolveErr = resolveCloudflaredBinary()
			if resolveErr != nil {
				return withDetail("could not prepare cloudflared", resolveErr.Error())
			}
			completeStep(stepID)

			tunnelCmd = exec.Command(cloudflaredPath, "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port), "--no-autoupdate")
			tunnelCmd.Dir = rootPath

			stdoutPipe, err := tunnelCmd.StdoutPipe()
			if err != nil {
				return withDetail("could not start tunnel client", err.Error())
			}
			stderrPipe, err := tunnelCmd.StderrPipe()
			if err != nil {
				return withDetail("could not start tunnel client", err.Error())
			}

			if err := tunnelCmd.Start(); err != nil {
				return withDetail("could not start cloudflare quick tunnel", err.Error())
			}

			state, err = config.LoadRuntimeState(rootPath)
			if err != nil {
				return withDetail("could not prepare runtime state", err.Error())
			}
			stateLoaded = true
			state.Mode = "local"
			state.TunnelPID = tunnelCmd.Process.Pid
			state.TunnelProvider = "cloudflare"
			if err := config.SaveRuntimeState(rootPath, state); err != nil {
				return withDetail("could not write runtime state", err.Error())
			}

			urlCh := make(chan string, 1)
			go scanTunnelOutput(stdoutPipe, urlCh, tunnelOutput)
			go scanTunnelOutput(stderrPipe, urlCh, tunnelOutput)

			waitCh = make(chan error, 1)
			go func() {
				waitCh <- tunnelCmd.Wait()
			}()

			stepID = addStep("Opening public URL")
			url, waitErr := waitForPublicURL(urlCh, waitCh)
			if waitErr != nil {
				detail := tunnelOutput.LastLine()
				if detail == "" {
					detail = waitErr.Error()
				}
				return withDetail("could not establish a public URL", detail)
			}
			publicURL = url
			completeStep(stepID)

			state.LiveURL = publicURL
			if err := config.SaveRuntimeState(rootPath, state); err != nil {
				return withDetail("could not write runtime state", err.Error())
			}

			return nil
		}); err != nil {
			if tunnelCmd != nil {
				_ = stopProcess(tunnelCmd)
			}
			if stateLoaded {
				state.LiveURL = ""
				state.TunnelPID = 0
				state.TunnelProvider = ""
				_ = config.SaveRuntimeState(rootPath, state)
			}
			return err
		}

		fmt.Printf("Live URL: %s\n", output.URL(publicURL))
		fmt.Println()
		fmt.Print(output.Footer("Press Ctrl+C to stop sharing."))

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		unexpectedStop := false
		unexpectedDetail := ""

		select {
		case <-sigCh:
			_ = stopProcess(tunnelCmd)
			<-waitCh
		case err := <-waitCh:
			unexpectedStop = true
			unexpectedDetail = conciseError(err)
			if unexpectedDetail == "" {
				unexpectedDetail = tunnelOutput.LastLine()
			}
		}

		state.LiveURL = ""
		state.TunnelProvider = ""
		state.TunnelPID = 0
		if err := config.SaveRuntimeState(rootPath, state); err != nil {
			return withDetail("could not write runtime state", err.Error())
		}

		if unexpectedStop {
			return withDetail("Tunnel stopped unexpectedly", unexpectedDetail)
		}

		fmt.Print("\r\033[2K")
		fmt.Println(output.Footer("Sharing stopped"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shareCmd)
}

func resolveSharePort(cfg config.ProjectConfig) (int, error) {
	serviceName := cfg.Share.DefaultService
	if serviceName == "" {
		serviceName = "frontend"
	}

	for _, svc := range cfg.Services {
		if svc.Name == serviceName && svc.Port > 0 {
			return svc.Port, nil
		}
	}

	if cfg.Share.DefaultPort > 0 {
		return cfg.Share.DefaultPort, nil
	}

	if serviceName != "frontend" {
		for _, svc := range cfg.Services {
			if svc.Name == "frontend" && svc.Port > 0 {
				return svc.Port, nil
			}
		}
	}

	return 0, errors.New("frontend share port is missing in hackctl.config.json")
}

func waitForPublicURL(urlCh <-chan string, waitCh <-chan error) (string, error) {
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()

	for {
		select {
		case publicURL := <-urlCh:
			if publicURL != "" {
				return publicURL, nil
			}
		case <-timer.C:
			return "", errors.New("timeout")
		case err := <-waitCh:
			if err != nil {
				return "", err
			}
			return "", errors.New("tunnel stopped")
		}
	}
}

func isPortReachable(port int, timeout time.Duration) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func resolveCloudflaredBinary() (string, error) {
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}

	cachePath, err := cachedCloudflaredPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", err
	}

	downloadURL, err := cloudflaredDownloadURL()
	if err != nil {
		return "", err
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", errors.New("download failed")
	}

	if filepath.Ext(downloadURL) == ".tgz" {
		if err := extractCloudflaredFromTGZ(resp.Body, cachePath); err != nil {
			return "", err
		}
	} else {
		file, err := os.OpenFile(cachePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			return "", err
		}
	}

	return cachePath, nil
}

func cachedCloudflaredPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(home, ".hackctl", "bin", runtime.GOOS, runtime.GOARCH, name), nil
}

func cloudflaredDownloadURL() (string, error) {
	base := "https://github.com/cloudflare/cloudflared/releases/latest/download/"

	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return base + "cloudflared-windows-amd64.exe", nil
		case "arm64":
			return base + "cloudflared-windows-arm64.exe", nil
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return base + "cloudflared-darwin-amd64.tgz", nil
		case "arm64":
			return base + "cloudflared-darwin-arm64.tgz", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return base + "cloudflared-linux-amd64", nil
		case "arm64":
			return base + "cloudflared-linux-arm64", nil
		}
	}

	return "", fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

func scanTunnelOutput(reader io.Reader, urlCh chan<- string, tail *lineTail) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if tail != nil {
			_, _ = tail.Write([]byte(line + "\n"))
		}
		if match := quickTunnelURL.FindString(line); match != "" {
			select {
			case urlCh <- match:
			default:
			}
		}
	}
}

func extractCloudflaredFromTGZ(source io.Reader, targetPath string) error {
	gzipReader, err := gzip.NewReader(source)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if filepath.Base(header.Name) != "cloudflared" {
			continue
		}

		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(file, tarReader); err != nil {
			return err
		}

		return nil
	}

	return errors.New("cloudflared binary not found in archive")
}

func stopProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if runtime.GOOS != "windows" {
		_ = cmd.Process.Signal(os.Interrupt)
		time.Sleep(500 * time.Millisecond)
	}

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	return nil
}
