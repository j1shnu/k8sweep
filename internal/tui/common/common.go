package common

// WindowSize tracks the terminal dimensions.
type WindowSize struct {
	Width  int
	Height int
}

const (
	MinWidth  = 60
	MinHeight = 15

	HeaderHeight = 3
	FooterHeight = 2
)
