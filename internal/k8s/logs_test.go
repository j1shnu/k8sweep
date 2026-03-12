package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	lines, err := ParseLogLines(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestParseLogLinesEmpty(t *testing.T) {
	lines, err := ParseLogLines(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestParseLogLinesNoTrailingNewline(t *testing.T) {
	input := "line1\nline2"
	lines, err := ParseLogLines(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2"}, lines)
}

func TestParseLogLinesSingleLine(t *testing.T) {
	input := "single line output"
	lines, err := ParseLogLines(strings.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []string{"single line output"}, lines)
}
