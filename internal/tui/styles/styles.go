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
	ColorLabel     = lipgloss.Color("#c1b895")
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

	LabelText = lipgloss.NewStyle().
			Foreground(ColorLabel)

	SelectedRow = lipgloss.NewStyle().
			Background(lipgloss.Color("#5A513C")).
			Foreground(lipgloss.Color("#FFFFFF")).
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

	ControllerRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8899AA"))

	ControllerRowDirty = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF6600"))

	LoadingSpinner = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	LoadingPrefix = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFFF"))

	LoadingFact = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFAA00"))

	CritSummary = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFailed)

	WarnSummary = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorEvicted)

	OKSummary = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorRunning)

	// Confirm button styles — destructive (Yes) vs safe (No)
	ButtonDanger = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#CC3333"))

	ButtonDangerDim = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CC3333"))

	ButtonSafe = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#338833"))

	ButtonSafeDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#338833"))
)

// Pre-built status styles to avoid per-render allocations.
var (
	statusRunning      = lipgloss.NewStyle().Foreground(ColorRunning)
	statusCompleted    = lipgloss.NewStyle().Foreground(ColorCompleted)
	statusFailed       = lipgloss.NewStyle().Foreground(ColorFailed)
	statusEvicted      = lipgloss.NewStyle().Foreground(ColorEvicted)
	statusCrashLoop    = lipgloss.NewStyle().Foreground(ColorCrashLoop)
	statusOOMKilled    = lipgloss.NewStyle().Foreground(ColorOOMKilled)
	statusPending      = lipgloss.NewStyle().Foreground(ColorPending)
	statusUnknown      = lipgloss.NewStyle().Foreground(ColorUnknown)
)

// StyleForStatus returns the appropriate lipgloss style for a pod status.
func StyleForStatus(status string) lipgloss.Style {
	switch status {
	case "Running":
		return statusRunning
	case "Completed":
		return statusCompleted
	case "Failed":
		return statusFailed
	case "Evicted":
		return statusEvicted
	case "CrashLoopBackOff":
		return statusCrashLoop
	case "ImagePullError":
		return statusFailed
	case "OOMKilled":
		return statusOOMKilled
	case "Pending":
		return statusPending
	default:
		return statusUnknown
	}
}
