package cmd

import (
	"strings"
	"sync"
)

type lineTail struct {
	mu    sync.Mutex
	max   int
	lines []string
	carry string
}

func newLineTail(max int) *lineTail {
	if max < 1 {
		max = 1
	}

	return &lineTail{max: max}
}

func (t *lineTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	chunk := strings.ReplaceAll(string(p), "\r", "\n")
	joined := t.carry + chunk
	parts := strings.Split(joined, "\n")

	if len(parts) == 1 {
		t.carry = parts[0]
		return len(p), nil
	}

	for _, part := range parts[:len(parts)-1] {
		t.push(part)
	}
	t.carry = parts[len(parts)-1]

	return len(p), nil
}

func (t *lineTail) LastLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if line := conciseText(t.carry); line != "" {
		return line
	}

	for i := len(t.lines) - 1; i >= 0; i-- {
		if line := conciseText(t.lines[i]); line != "" {
			return line
		}
	}

	return ""
}

func (t *lineTail) push(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}

	t.lines = append(t.lines, trimmed)
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}
