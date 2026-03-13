package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	p := Load("/nonexistent/path/prefs.json")
	assert.Equal(t, Preferences{}, p)
}

func TestLoad_CorruptJSON_ReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid json"), 0o644))

	p := Load(path)
	assert.Equal(t, Preferences{}, p)
}

func TestLoad_ValidJSON_ReturnsParsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	data := `{"sort_column":"STATUS","sort_order":"desc","dirty_filter":true,"namespace":"kube-system","all_collapsed":true}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	p := Load(path)
	assert.Equal(t, "STATUS", p.SortColumn)
	assert.Equal(t, "desc", p.SortOrder)
	assert.True(t, p.DirtyFilter)
	assert.Equal(t, "kube-system", p.Namespace)
	assert.True(t, p.AllCollapsed)
}

func TestLoad_InvalidSortColumn_ClampedToName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	data := `{"sort_column":"BOGUS","sort_order":"asc"}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	p := Load(path)
	assert.Equal(t, "NAME", p.SortColumn)
}

func TestLoad_InvalidSortOrder_ClampedToAsc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	data := `{"sort_column":"NAME","sort_order":"random"}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	p := Load(path)
	assert.Equal(t, "asc", p.SortOrder)
}

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "nested", "prefs.json")

	prefs := Preferences{
		SortColumn:  "AGE",
		SortOrder:   "desc",
		DirtyFilter: true,
		Namespace:   "prod",
	}
	require.NoError(t, Save(path, prefs))

	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestSave_WritesValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	prefs := Preferences{
		SortColumn:   "STATUS",
		SortOrder:    "desc",
		DirtyFilter:  true,
		AllCollapsed: true,
		SearchQuery:  "nginx",
	}
	require.NoError(t, Save(path, prefs))

	loaded := Load(path)
	assert.Equal(t, prefs.SortColumn, loaded.SortColumn)
	assert.Equal(t, prefs.SortOrder, loaded.SortOrder)
	assert.Equal(t, prefs.DirtyFilter, loaded.DirtyFilter)
	assert.Equal(t, prefs.AllCollapsed, loaded.AllCollapsed)
	assert.Equal(t, prefs.SearchQuery, loaded.SearchQuery)
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	original := Preferences{
		SortColumn:   "RESTARTS",
		SortOrder:    "desc",
		DirtyFilter:  true,
		Namespace:    "monitoring",
		AllCollapsed: true,
		SearchQuery:  "api-",
	}

	require.NoError(t, Save(path, original))
	loaded := Load(path)
	assert.Equal(t, original, loaded)
}

func TestRoundTrip_AllNamespacesSentinel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	original := Preferences{
		SortColumn:  "NAME",
		SortOrder:   "asc",
		Namespace:   AllNamespacesSentinel,
		DirtyFilter: true,
	}

	require.NoError(t, Save(path, original))
	loaded := Load(path)
	assert.Equal(t, AllNamespacesSentinel, loaded.Namespace)
	assert.Equal(t, original, loaded)
}

func TestValidate_ClampsUnknownSortColumn(t *testing.T) {
	p := Validate(Preferences{SortColumn: "INVALID", SortOrder: "asc"})
	assert.Equal(t, "NAME", p.SortColumn)
}

func TestValidate_ClampsUnknownSortOrder(t *testing.T) {
	p := Validate(Preferences{SortColumn: "NAME", SortOrder: "unknown"})
	assert.Equal(t, "asc", p.SortOrder)
}

func TestValidate_PreservesValidValues(t *testing.T) {
	p := Validate(Preferences{
		SortColumn:  "CPU",
		SortOrder:   "desc",
		DirtyFilter: true,
		Namespace:   "prod",
	})
	assert.Equal(t, "CPU", p.SortColumn)
	assert.Equal(t, "desc", p.SortOrder)
	assert.True(t, p.DirtyFilter)
	assert.Equal(t, "prod", p.Namespace)
}

func TestDefaultPath_ReturnsNonEmpty(t *testing.T) {
	path := DefaultPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "k8sweep")
	assert.Contains(t, path, "preferences.json")
}

func TestSave_AtomicWrite_DoesNotCorruptExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")

	// Write initial valid prefs
	original := Preferences{SortColumn: "NAME", SortOrder: "asc"}
	require.NoError(t, Save(path, original))

	// Write again (simulates overwrite)
	updated := Preferences{SortColumn: "STATUS", SortOrder: "desc"}
	require.NoError(t, Save(path, updated))

	loaded := Load(path)
	assert.Equal(t, "STATUS", loaded.SortColumn)
	assert.Equal(t, "desc", loaded.SortOrder)
}

func TestLoad_EmptyFile_ReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))

	p := Load(path)
	assert.Equal(t, Preferences{}, p)
}
