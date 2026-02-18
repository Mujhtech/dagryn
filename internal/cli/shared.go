package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/client"
	"github.com/mujhtech/dagryn/pkg/dagryn/config"
	"github.com/mujhtech/dagryn/pkg/storage"
)

// ContextConfig represents the .dagryn/context.json file.
type ContextConfig struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name,omitempty"`
	SetAt       string `json:"set_at"`
}

// LoadContextProjectID reads the project ID from .dagryn/context.json.
// Returns empty string if the file doesn't exist.
func LoadContextProjectID(projectRoot string) string {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".dagryn", "context.json"))
	if err != nil {
		return ""
	}
	var ctx ContextConfig
	if err := json.Unmarshal(data, &ctx); err != nil {
		return ""
	}
	return ctx.ProjectID
}

// BuildBucket creates a storage.Bucket from the remote cache config.
func BuildBucket(rc config.RemoteCacheConfig) (storage.Bucket, error) {
	return storage.NewBucket(storage.Config{
		Provider:        storage.ProviderType(rc.Provider),
		BasePath:        rc.BasePath,
		Bucket:          rc.Bucket,
		Region:          rc.Region,
		Endpoint:        rc.Endpoint,
		AccessKeyID:     rc.AccessKeyID,
		SecretAccessKey: rc.SecretAccessKey,
		UsePathStyle:    rc.UsePathStyle,
		Prefix:          rc.Prefix,
	})
}

// SyncWorkflowToRemote parses dagryn.toml and syncs the workflow to the remote server.
func SyncWorkflowToRemote(ctx context.Context, apiClient *client.Client, projectRoot string, projectID uuid.UUID) error {
	configPath := filepath.Join(projectRoot, "dagryn.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("dagryn.toml not found")
	}

	cfg, err := config.Parse(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse dagryn.toml: %w", err)
	}

	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	syncReq := client.SyncWorkflowRequest{
		Name:       cfg.Workflow.Name,
		IsDefault:  cfg.Workflow.Default,
		ConfigHash: config.ComputeConfigHash(rawConfig),
		RawConfig:  string(rawConfig),
		Tasks:      make([]client.SyncWorkflowTaskData, 0, len(cfg.Tasks)),
	}

	if syncReq.Name == "" {
		syncReq.Name = "default"
	}

	for name, task := range cfg.Tasks {
		taskData := client.SyncWorkflowTaskData{
			Name:      name,
			Command:   task.Command,
			Needs:     task.Needs,
			Inputs:    task.Inputs,
			Outputs:   task.Outputs,
			Plugins:   task.GetPlugins(),
			Env:       task.Env,
			Group:     task.Group,
			Condition: task.If,
		}
		if task.Workdir != "" {
			taskData.Workdir = &task.Workdir
		}
		syncReq.Tasks = append(syncReq.Tasks, taskData)
	}

	resp, err := apiClient.SyncWorkflow(ctx, projectID, syncReq)
	if err != nil {
		return fmt.Errorf("failed to sync workflow: %w", err)
	}

	if resp.Data.Changed {
		fmt.Printf("  Synced: %s (%d tasks)\n", resp.Data.Name, resp.Data.TaskCount)
	}

	return nil
}

// GetGitBranch returns the current git branch.
func GetGitBranch() string {
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	return ""
}

// GetGitCommit returns the current git commit hash.
func GetGitCommit() string {
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))

	if !strings.HasPrefix(content, "ref:") {
		if len(content) >= 7 {
			return content[:7]
		}
		return content
	}

	refPath := strings.TrimPrefix(content, "ref: ")
	refData, err := os.ReadFile(".git/" + refPath)
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(refData))
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}
