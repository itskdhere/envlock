package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SuccessColor   = lipgloss.Color("#04B575")
	WarningColor   = lipgloss.Color("#FFB800")
	ErrorColor     = lipgloss.Color("#FF4B4B")
	SubtleColor    = lipgloss.Color("#737373")
	HighlightColor = lipgloss.Color("#00E5FF")

	BoldStyle      = lipgloss.NewStyle().Bold(true)
	SubtleStyle    = lipgloss.NewStyle().Foreground(SubtleColor)
	PrimaryStyle   = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	HighlightStyle = lipgloss.NewStyle().Foreground(HighlightColor).Bold(true)

	SuccessIcon = lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).SetString("✔")
	WarningIcon = lipgloss.NewStyle().Bold(true).Foreground(WarningColor).SetString("▲")
	ErrorIcon   = lipgloss.NewStyle().Bold(true).Foreground(ErrorColor).SetString("✖")
	InfoIcon    = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).SetString("ℹ")

	SuccessBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(SuccessColor).
			Padding(0, 1)

	WarningBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(WarningColor).
			Padding(0, 1)

	ErrorBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ErrorColor).
			Padding(0, 1)

	BannerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			Margin(0, 0)

	ErrorCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Padding(1, 2).
			Margin(0, 0)
)

func IsColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}

func Success(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	if IsColorDisabled() {
		return fmt.Sprintf("[OK] %s", msg)
	}
	return fmt.Sprintf("%s %s", SuccessIcon.String(), msg)
}

func Warning(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	if IsColorDisabled() {
		return fmt.Sprintf("[WARNING] %s", msg)
	}
	return fmt.Sprintf("%s %s", WarningIcon.String(), msg)
}

func Error(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	if IsColorDisabled() {
		return fmt.Sprintf("[ERROR] %s", msg)
	}
	return fmt.Sprintf("%s %s", ErrorIcon.String(), msg)
}

func Info(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	if IsColorDisabled() {
		return fmt.Sprintf("[INFO] %s", msg)
	}
	return fmt.Sprintf("%s %s", InfoIcon.String(), msg)
}

func Subtle(text string) string {
	if IsColorDisabled() {
		return text
	}
	return SubtleStyle.Render(text)
}

func Bold(text string) string {
	if IsColorDisabled() {
		return text
	}
	return BoldStyle.Render(text)
}

func Highlight(text string) string {
	if IsColorDisabled() {
		return text
	}
	return HighlightStyle.Render(text)
}

func Primary(text string) string {
	if IsColorDisabled() {
		return text
	}
	return PrimaryStyle.Render(text)
}

func Banner(title string, lines []string) string {
	header := Primary(title)
	content := append([]string{header, ""}, lines...)
	return BannerStyle.Render(strings.Join(content, "\n"))
}

func ErrorCard(title string, message string, hint string) string {
	var parts []string
	if IsColorDisabled() {
		parts = append(parts, "[ERROR] "+title)
		if message != "" {
			parts = append(parts, message)
		}
		if hint != "" {
			parts = append(parts, "", "How to fix:")
			for _, line := range strings.Split(hint, "\n") {
				parts = append(parts, "  "+line)
			}
		}
		return strings.Join(parts, "\n")
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(ErrorColor).Render("❌ " + title)
	parts = append(parts, header)
	if message != "" {
		parts = append(parts, message)
	}
	if hint != "" {
		parts = append(parts, "")
		hintHeader := lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("💡 How to fix:")
		parts = append(parts, hintHeader)
		for _, line := range strings.Split(hint, "\n") {
			parts = append(parts, "  "+line)
		}
	}
	return ErrorCardStyle.Render(strings.Join(parts, "\n"))
}

func CopyToClipboardOSC52(text string) bool {
	fd := os.Stdout.Fd()
	if !isatty.IsTerminal(fd) && !isatty.IsCygwinTerminal(fd) {
		return false
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\x07", encoded)
	return true
}
