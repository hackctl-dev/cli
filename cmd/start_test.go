package cmd

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestWaitForServiceReadyReachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not listen: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	waitCh := make(chan error)

	err = waitForServiceReady(context.Background(), port, waitCh, 2*time.Second)
	if err != nil {
		t.Fatalf("expected ready service, got error: %v", err)
	}
}

func TestWaitForServiceReadyTimeout(t *testing.T) {
	unusedPort := claimUnusedPort(t)
	waitCh := make(chan error)

	err := waitForServiceReady(context.Background(), unusedPort, waitCh, 300*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	if !strings.Contains(err.Error(), "readiness timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForServiceReadyExitBeforeReady(t *testing.T) {
	unusedPort := claimUnusedPort(t)
	waitCh := make(chan error, 1)
	waitCh <- errors.New("exit status 1")

	err := waitForServiceReady(context.Background(), unusedPort, waitCh, 2*time.Second)
	if err == nil {
		t.Fatalf("expected exit-before-ready error")
	}

	if !strings.Contains(err.Error(), "service exited before ready") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func claimUnusedPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not claim port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}
