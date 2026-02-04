package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("12")  // bright blue
	colorSecondary = lipgloss.Color("10")  // bright green
	colorDim       = lipgloss.Color("240") // gray
	colorHighlight = lipgloss.Color("11")  // bright yellow
	colorBorder    = lipgloss.Color("238") // dark gray

	// Input area
	styleInput = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleInputPrompt = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	// List items
	styleListSelected = lipgloss.NewStyle().
				Foreground(colorHighlight).
				Bold(true)

	styleListNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	styleListSource = lipgloss.NewStyle().
			Width(7)

	styleSourceClaude = lipgloss.NewStyle().
				Foreground(colorPrimary)

	styleSourceCodex = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Panels
	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	styleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary)

	// Status bar
	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)

	// Panel titles
	styleTitle = lipgloss.NewStyle().
			Foreground(colorDim).
			Bold(true)
)
