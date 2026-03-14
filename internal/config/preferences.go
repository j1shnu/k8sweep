package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AllNamespacesSentinel is stored in Namespace when all-namespaces mode was active.
// The k8s sentinel is "" which omitempty would suppress, so we use a distinct value.
const AllNamespacesSentinel = "*"

// Preferences holds lightweight UI state persisted across runs.
type Preferences struct {
	SortColumn          string `json:"sort_column"`
	SortOrder           string `json:"sort_order"`
	DirtyFilter         bool   `json:"dirty_filter"`
	Namespace           string `json:"namespace,omitempty"`
	AllCollapsed        bool   `json:"all_collapsed"`
	SearchQuery         string `json:"search_query,omitempty"`
	ControllerDrillDown string `json:"controller_drill_down,omitempty"`
}

// validSortColumns lists recognized sort column labels.
var validSortColumns = map[string]struct{}{
	"NAME": {}, "STATUS": {}, "AGE": {}, "RESTARTS": {},
	"OWNER": {}, "CPU": {}, "MEM": {},
}

// Validate clamps invalid fields to safe defaults and returns a new Preferences.
func Validate(p Preferences) Preferences {
	result := p
	if _, ok := validSortColumns[result.SortColumn]; !ok {
		result.SortColumn = "NAME"
	}
	if result.SortOrder != "asc" && result.SortOrder != "desc" {
		result.SortOrder = "asc"
	}
	return result
}

// DefaultPath returns the platform-appropriate preferences file path.
// Uses os.UserConfigDir (~/.config on Linux, ~/Library/Application Support on macOS).
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "k8sweep", "preferences.json")
}

// Load reads and parses preferences from the given path.
// Returns zero-value Preferences on any error (missing file, corrupt JSON, etc.).
func Load(path string) Preferences {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preferences{}
	}
	var p Preferences
	if err := json.Unmarshal(data, &p); err != nil {
		return Preferences{}
	}
	return Validate(p)
}

// Save writes preferences to the given path atomically (write temp, then rename).
// Creates parent directories if needed.
func Save(path string, prefs Preferences) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		os.Remove(tmp) // best-effort cleanup of partial write
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
