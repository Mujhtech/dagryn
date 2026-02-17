package handlers

import (
	"testing"

	"github.com/mujhtech/dagryn/pkg/database/models"
)

func TestCollectPluginSpecsFromWorkflows(t *testing.T) {
	raw := `
[workflow]
name = "test"

[plugins]
setup-go = "dagryn/setup-go@v1"
setup-node = "local:./plugins/setup-node"

[tasks.sample]
command = "echo test"
`

	workflows := []models.WorkflowWithTasks{
		{
			ProjectWorkflow: models.ProjectWorkflow{
				RawConfig: &raw,
			},
			Tasks: []models.WorkflowTask{
				{
					Plugins: []string{
						"dagryn/setup-go@v1",
						"github:owner/custom-plugin@v2.0.0",
					},
				},
				{
					Plugins: []string{
						"local:./plugins/setup-node",
					},
				},
			},
		},
	}

	specs := collectPluginSpecsFromWorkflows(workflows)

	if len(specs) != 3 {
		t.Fatalf("expected 3 unique specs, got %d: %#v", len(specs), specs)
	}

	want := map[string]bool{
		"dagryn/setup-go@v1":                false,
		"local:./plugins/setup-node":        false,
		"github:owner/custom-plugin@v2.0.0": false,
	}

	for _, spec := range specs {
		if _, ok := want[spec]; !ok {
			t.Fatalf("unexpected spec: %s", spec)
		}
		want[spec] = true
	}
	for spec, seen := range want {
		if !seen {
			t.Fatalf("missing spec: %s", spec)
		}
	}
}

func TestBuildPluginInfoFromSpec_OfficialGitHub(t *testing.T) {
	official := map[string]PluginInfo{
		"setup-go": {
			Name:        "setup-go",
			Source:      "official",
			Version:     "1.0.0",
			Description: "Install Go",
			Type:        "composite",
			Author:      "dagryn",
		},
	}

	got := buildPluginInfoFromSpec("dagryn/setup-go@v1", official)

	if got.Name != "setup-go" {
		t.Fatalf("expected name setup-go, got %s", got.Name)
	}
	if got.Source != "github" {
		t.Fatalf("expected source github, got %s", got.Source)
	}
	if got.Version != "v1" {
		t.Fatalf("expected version v1, got %s", got.Version)
	}
	if got.Description != "Install Go" {
		t.Fatalf("expected official description to be used")
	}
	if !got.Installed {
		t.Fatalf("expected installed true")
	}
}

func TestBuildPluginInfoFromSpec_InvalidSpecFallback(t *testing.T) {
	got := buildPluginInfoFromSpec("not-a-plugin-spec", nil)

	if got.Name != "not-a-plugin-spec" {
		t.Fatalf("expected fallback name from raw spec, got %s", got.Name)
	}
	if got.Source != "unknown" {
		t.Fatalf("expected unknown source, got %s", got.Source)
	}
	if !got.Installed {
		t.Fatalf("expected installed true")
	}
}
