package styles

import "github.com/charmbracelet/lipgloss"

// Status colors
var (
	ColorRunning   = lipgloss.Color("#00FFFF") // Cyan
	ColorCompleted = lipgloss.Color("#808080") // Gray
	ColorFailed    = lipgloss.Color("#FF0000") // Red
	ColorEvicted   = lipgloss.Color("#FFAA00") // Orange/Yellow
	ColorCrashLoop = lipgloss.Color("#FF6600") // Dark Orange
	ColorOOMKilled = lipgloss.Color("#FF3366") // Pink-Red
	ColorPending   = lipgloss.Color("#FFFF00") // Yellow
	ColorUnknown   = lipgloss.Color("#999999") // Dim Gray
	ColorSelected  = lipgloss.Color("#00FF00") // Green
	ColorPointer   = lipgloss.Color("#FFFFFF") // White
	ColorMuted     = lipgloss.Color("#666666") // Muted
	ColorAccent    = lipgloss.Color("#7D56F4") // Purple accent for borders/highlights
)

// Component styles
var (
	HeaderBox = lipgloss.NewStyle().
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent).
			Padding(0, 1)

	FooterHelp = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SelectedRow = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Bold(true)

	Pointer = lipgloss.NewStyle().
		Foreground(ColorPointer).
		Bold(true)

	OverlayBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6666")).
			Padding(1, 2).
			Bold(true)

	StatusMessage = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")).
			Bold(true)

	ErrorMessage = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent)

	FilterActiveHeaderBox = lipgloss.NewStyle().
				Bold(true).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFAA00")).
				Padding(0, 1)

	FilterBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFAA00"))

	LoadingSpinner = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	LoadingPrefix = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFFF"))

	LoadingFact = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFAA00"))
)

// StyleForStatus returns the appropriate lipgloss style for a pod status.
func StyleForStatus(status string) lipgloss.Style {
	switch status {
	case "Running":
		return lipgloss.NewStyle().Foreground(ColorRunning)
	case "Completed":
		return lipgloss.NewStyle().Foreground(ColorCompleted)
	case "Failed":
		return lipgloss.NewStyle().Foreground(ColorFailed)
	case "Evicted":
		return lipgloss.NewStyle().Foreground(ColorEvicted)
	case "CrashLoopBackOff":
		return lipgloss.NewStyle().Foreground(ColorCrashLoop)
	case "OOMKilled":
		return lipgloss.NewStyle().Foreground(ColorOOMKilled)
	case "Pending":
		return lipgloss.NewStyle().Foreground(ColorPending)
	default:
		return lipgloss.NewStyle().Foreground(ColorUnknown)
	}
}
