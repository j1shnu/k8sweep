package help

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/stretchr/testify/assert"
)

func makeBindings(n int) []key.Binding {
	bindings := make([]key.Binding, 0, n)
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("k%d", i)
		d := fmt.Sprintf("desc-%02d", i)
		bindings = append(bindings, key.NewBinding(key.WithKeys(k), key.WithHelp(k, d)))
	}
	return bindings
}

func TestHelpScrollDownAndUp(t *testing.T) {
	m := New([][]key.Binding{makeBindings(30)}).SetSize(100, 14)

	viewTop := m.View()
	assert.Contains(t, viewTop, "desc-00")

	m = m.ScrollDown()
	m = m.ScrollDown()
	viewScrolled := m.View()
	assert.NotEqual(t, viewTop, viewScrolled)

	m = m.ScrollUp()
	m = m.ScrollUp()
	assert.Equal(t, viewTop, m.View())
}
