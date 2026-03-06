package plugin

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// Valid hook names for integration plugins.
const (
	HookOnRunStart   = "on_run_start"
	HookOnRunEnd     = "on_run_end"
	HookOnTaskStart  = "on_task_start"
	HookOnTaskEnd    = "on_task_end"
	HookOnRunSuccess = "on_run_success"
	HookOnRunFailure = "on_run_failure"
)

// ValidHookNames is the set of valid hook names for integration plugins.
var ValidHookNames = map[string]bool{
	HookOnRunStart:   true,
	HookOnRunEnd:     true,
	HookOnTaskStart:  true,
	HookOnTaskEnd:    true,
	HookOnRunSuccess: true,
	HookOnRunFailure: true,
}

// HookDef defines a lifecycle hook for an integration plugin.
type HookDef struct {
	Command string            `toml:"command"`
	If      string            `toml:"if"`
	Env     map[string]string `toml:"env"`
}

// Manifest represents a plugin.toml manifest file.
type Manifest struct {
	Plugin    ManifestPlugin       `toml:"plugin"`
	Tool      ManifestTool         `toml:"tool"`
	Platforms map[string]string    `toml:"platforms"`
	Inputs    map[string]InputDef  `toml:"inputs"`
	Outputs   map[string]OutputDef `toml:"outputs"`
	Steps     []CompositeStep      `toml:"step"`
	Cleanup   []CompositeStep      `toml:"cleanup"` // Cleanup steps run in reverse order after main steps
	Hooks     map[string]HookDef   `toml:"hooks"`   // Lifecycle hooks for integration plugins
}

// ManifestPlugin holds plugin metadata.
type ManifestPlugin struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Version     string `toml:"version"`
	Type        string `toml:"type"` // "tool", "composite", or "integration"
	Author      string `toml:"author"`
	License     string `toml:"license"`
	Homepage    string `toml:"homepage"`
}

// ManifestTool holds tool-specific configuration.
type ManifestTool struct {
	Binary string `toml:"binary"`
}

// InputDef defines an input parameter for the plugin.
type InputDef struct {
	Required    bool   `toml:"required"`
	Description string `toml:"description"`
	Default     string `toml:"default"`
}

// OutputDef defines an output of the plugin.
type OutputDef struct {
	Description string `toml:"description"`
}

// CompositeStep defines a step in a composite plugin.
type CompositeStep struct {
	Name    string            `toml:"name"`
	Command string            `toml:"command"`
	If      string            `toml:"if"`
	Env     map[string]string `toml:"env"`
}

// ParseManifest parses a plugin.toml manifest from raw bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse plugin manifest: %w", err)
	}
	return &m, nil
}

// ValidateManifest validates the manifest for required fields and consistency.
func ValidateManifest(m *Manifest) error {
	if m.Plugin.Name == "" {
		return fmt.Errorf("plugin.name is required")
	}
	if m.Plugin.Version == "" {
		return fmt.Errorf("plugin.version is required")
	}

	switch m.Plugin.Type {
	case "", "tool":
		// tool type is valid
	case "composite":
		if len(m.Steps) == 0 {
			return fmt.Errorf("composite plugin must have at least one step")
		}
		for i, step := range m.Steps {
			if step.Command == "" {
				return fmt.Errorf("step %d (%s) must have a command", i, step.Name)
			}
		}
	case "integration":
		if len(m.Hooks) == 0 {
			return fmt.Errorf("integration plugin must have at least one hook")
		}
		for name, hook := range m.Hooks {
			if !ValidHookNames[name] {
				return fmt.Errorf("invalid hook name %q", name)
			}
			if hook.Command == "" {
				return fmt.Errorf("hook %q must have a command", name)
			}
		}
	default:
		return fmt.Errorf("unsupported plugin type %q: must be \"tool\", \"composite\", or \"integration\"", m.Plugin.Type)
	}

	return nil
}

// IsComposite returns true if the plugin type is "composite".
func (m *Manifest) IsComposite() bool {
	return m.Plugin.Type == "composite"
}

// IsTool returns true if the plugin type is "tool" or empty (default).
func (m *Manifest) IsTool() bool {
	return m.Plugin.Type == "" || m.Plugin.Type == "tool"
}

// IsIntegration returns true if the plugin type is "integration".
func (m *Manifest) IsIntegration() bool {
	return m.Plugin.Type == "integration"
}

// PlatformAsset returns the asset filename for the given platform key (e.g. "darwin-arm64").
func (m *Manifest) PlatformAsset(key string) string {
	if m.Platforms == nil {
		return ""
	}
	// Try exact match first
	if asset, ok := m.Platforms[key]; ok {
		return asset
	}
	// Try normalized key (lowercase)
	lower := strings.ToLower(key)
	for k, v := range m.Platforms {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}
