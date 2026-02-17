package ghactions

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// TranslationResult contains the generated Dagryn TOML snippet plus metadata.
type TranslationResult struct {
	Plugins   map[string]string
	TasksToml string
	Workflows []WorkflowSummary
}

// WorkflowSummary provides a minimal overview of a translated workflow file.
type WorkflowSummary struct {
	File      string
	Name      string
	TaskCount int
}

// TranslateWorkflows converts GitHub Actions workflow files into a Dagryn TOML snippet.
// The input map is filename -> file content.
func TranslateWorkflows(files map[string][]byte) (TranslationResult, error) {
	result := TranslationResult{
		Plugins: make(map[string]string),
	}
	if len(files) == 0 {
		return result, nil
	}

	var tasksBuf strings.Builder

	for fname, content := range files {
		var wf githubWorkflow
		if err := yaml.Unmarshal(content, &wf); err != nil {
			return TranslationResult{}, fmt.Errorf("parse workflow %s: %w", fname, err)
		}

		workflowName := wf.Name
		if workflowName == "" {
			workflowName = strings.TrimSuffix(strings.TrimSuffix(fname, ".yml"), ".yaml")
		}
		workflowPrefix := sanitizeWorkflowName(workflowName)

		if len(wf.Jobs) == 0 {
			continue
		}

		tasksBuf.WriteString("\n")
		tasksBuf.WriteString(fmt.Sprintf("# Workflow: %s (from .github/workflows/%s)\n", workflowName, fname))

		taskCount := 0
		for jobID, job := range wf.Jobs {
			taskName := sanitizeWorkflowName(workflowPrefix + "_" + jobID)

			var commands []string
			var taskUses []string

			for _, step := range job.Steps {
				if strings.TrimSpace(step.Uses) != "" {
					if key, spec := pluginFromUses(step.Uses); key != "" && spec != "" {
						if _, exists := result.Plugins[key]; !exists {
							result.Plugins[key] = spec
						}
						if !containsString(taskUses, key) {
							taskUses = append(taskUses, key)
						}
					}
				}

				if strings.TrimSpace(step.Run) != "" {
					commands = append(commands, strings.TrimSpace(step.Run))
				}
			}

			if len(commands) == 0 {
				continue
			}

			joined := strings.Join(commands, " && ")
			escapedCmd := escapeForTomlString(joined)

			tasksBuf.WriteString(fmt.Sprintf("[tasks.%s]\n", taskName))
			if job.Name != "" {
				tasksBuf.WriteString(fmt.Sprintf("description = \"%s\"\n", escapeForTomlString(job.Name)))
			}
			if len(taskUses) > 0 {
				quoted := make([]string, len(taskUses))
				for i, u := range taskUses {
					quoted[i] = fmt.Sprintf("\"%s\"", u)
				}
				tasksBuf.WriteString(fmt.Sprintf("uses = [%s]\n", strings.Join(quoted, ", ")))
			}
			tasksBuf.WriteString(fmt.Sprintf("command = \"%s\"\n", escapedCmd))
			tasksBuf.WriteString("inputs = [\"**/*\"]\n\n")
			taskCount++
		}

		result.Workflows = append(result.Workflows, WorkflowSummary{
			File:      fname,
			Name:      workflowName,
			TaskCount: taskCount,
		})
	}

	if tasksBuf.Len() == 0 {
		return result, nil
	}

	var b strings.Builder
	b.WriteString("\n# -----------------------------------------------------------------------------\n")
	b.WriteString("# Tasks generated from existing GitHub Actions workflows\n")
	b.WriteString("# You can tweak commands / dependencies to better fit Dagryn.\n")
	b.WriteString("# -----------------------------------------------------------------------------\n")

	if len(result.Plugins) > 0 {
		b.WriteString("\n[plugins]\n")
		keys := make([]string, 0, len(result.Plugins))
		for k := range result.Plugins {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("%s = \"%s\"\n", k, result.Plugins[k]))
		}
		b.WriteString("\n")
	}

	b.WriteString(tasksBuf.String())
	result.TasksToml = b.String()
	return result, nil
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// githubWorkflow is a minimal subset of a GitHub Actions workflow used for translation.
type githubWorkflow struct {
	Name string                       `yaml:"name"`
	Jobs map[string]githubWorkflowJob `yaml:"jobs"`
}

type githubWorkflowJob struct {
	Name  string               `yaml:"name"`
	Steps []githubWorkflowStep `yaml:"steps"`
}

type githubWorkflowStep struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
	Uses string `yaml:"uses"`
}

// sanitizeWorkflowName converts a workflow file name into a safe task prefix.
func sanitizeWorkflowName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

// pluginFromUses converts a GitHub Actions "uses:" value into a Dagryn plugin
// key and specification (github:owner/repo@version).
func pluginFromUses(uses string) (string, string) {
	uses = strings.TrimSpace(uses)
	if uses == "" {
		return "", ""
	}

	parts := strings.Split(uses, "@")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", ""
	}
	version := "latest"
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		version = strings.TrimSpace(parts[1])
	}

	ownerRepo := name
	if !strings.Contains(ownerRepo, "/") {
		// Common shorthand like "checkout@v4"
		ownerRepo = "actions/" + ownerRepo
	}

	spec := fmt.Sprintf("github:%s@%s", ownerRepo, version)
	// Plugin key like actions_checkout
	key := sanitizeWorkflowName(strings.ReplaceAll(ownerRepo, "/", "_"))
	return key, spec
}

// escapeForTomlString escapes a string for use inside a double-quoted TOML string.
func escapeForTomlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
