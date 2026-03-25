package ui

import "github.com/charmbracelet/lipgloss"

// Theme colors (Aesthetically professional, high-contrast)
const (
	ColorPrimary   = "#00ffd5" // Neo-Cyan
	ColorSecondary = "#ffcc00" // Amber
	ColorSuccess   = "#00ff00" // Emerald
	ColorError     = "#ff0000" // Ruby
	ColorDim       = "#555555" // Slate
	ColorMuted     = "#333333" // Charcoal
	ColorText      = "#ffffff" // White
	ColorBackground = "#000000" // Black (standard terminal)
)

var (
	// Panel Styles
	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorMuted)).
			Padding(0, 1)

	// Typography
	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Padding(0, 2)

	StylePrice = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondary)).
			Bold(true)

	// Status Styles
	StyleBuy = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorSuccess))

	StyleSell = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorError))

	StyleWait = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDim))

	StyleMuted = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDim)).
			Italic(true)

	// Tab Styles
	StyleTabActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPrimary)).
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color(ColorText))

	StyleTabInactive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorMuted)).
			Padding(0, 2).
			Foreground(lipgloss.Color(ColorDim))
)
