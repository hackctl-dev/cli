package cmd

import (
	"errors"
	"fmt"
	"strings"
)

const maxErrorDetailLen = 140

func conciseText(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(trimmed, "\r", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if len(line) > maxErrorDetailLen {
			return line[:maxErrorDetailLen-3] + "..."
		}

		return line
	}

	return ""
}

func conciseError(err error) string {
	if err == nil {
		return ""
	}

	return conciseText(err.Error())
}

func withDetail(base string, detail string) error {
	shortDetail := conciseText(detail)
	if shortDetail == "" {
		return errors.New(base)
	}

	return fmt.Errorf("%s: %s", base, shortDetail)
}

func commandError(base string, err error, output []byte) error {
	detail := conciseText(string(output))
	if detail == "" {
		detail = conciseError(err)
	}

	return withDetail(base, detail)
}
