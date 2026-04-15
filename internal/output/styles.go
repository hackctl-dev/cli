package output

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	commandStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	urlStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	iconSuccess  = "●"
	iconFailure  = "●"
	iconInfo     = "i"
	iconFooter   = "?"
)

func Command(name string) string {
	return commandStyle.Render(name)
}

func Field(key string, value string) string {
	return fmt.Sprintf("%s %s", keyStyle.Render(key+":"), value)
}

func OK(message string) string {
	return okStyle.Render(message)
}

func Warn(message string) string {
	return warnStyle.Render(message)
}

func Info(message string) string {
	return infoStyle.Render(message)
}

func Error(message string) string {
	return errStyle.Render(message)
}

func URL(value string) string {
	return urlStyle.Render(value)
}

func Footer(text string) string {
	if text == "" {
		return footerStyle.Render(iconFooter)
	}

	return footerStyle.Render(fmt.Sprintf("%s %s", iconFooter, text))
}

func SuccessIcon() string {
	return iconSuccess
}

func FailureIcon() string {
	return iconFailure
}

func InfoIcon() string {
	return iconInfo
}

func Section(title string) string {
	return commandStyle.Render(title)
}
