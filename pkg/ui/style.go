package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Brand colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SuccessColor   = lipgloss.Color("#04B575")
	WarningColor   = lipgloss.Color("#FFB800")
	DangerColor    = lipgloss.Color("#FF4E64")
	SubtleColor    = lipgloss.Color("#6C7086")
	HighlightColor = lipgloss.Color("#38BDF8")

	// Base styles
	BoldText = lipgloss.NewStyle().Bold(true)
	DimText  = lipgloss.NewStyle().Foreground(SubtleColor)

	// Badge styles
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

	DangerBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(DangerColor).
			Padding(0, 1)

	InfoBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	// Card/Banner styles
	BannerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2).
			Margin(0, 0)
)

// Success formats a success message with a badge.
func Success(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return fmt.Sprintf("%s %s", SuccessBadge.Render("SUCCESS"), msg)
}

// Warning formats a warning message with a badge.
func Warning(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return fmt.Sprintf("%s %s", WarningBadge.Render("WARNING"), msg)
}

// Danger formats an error message with a badge.
func Danger(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return fmt.Sprintf("%s %s", DangerBadge.Render("ERROR"), msg)
}

// Info formats an informational message with a badge.
func Info(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return fmt.Sprintf("%s %s", InfoBadge.Render("INFO"), msg)
}

// Banner renders an emphasized box with a title and multi-line body.
func Banner(title string, lines []string) string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Render(title)

	content := append([]string{header, ""}, lines...)
	return BannerStyle.Render(strings.Join(content, "\n"))
}

