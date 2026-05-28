package tui

import "github.com/charmbracelet/lipgloss"

// Styles for colourful CLI output.
var (
	KeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	StatusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	PriorityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	DateStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	SummaryStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	SuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	ErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	HeadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
)
