package models

import (
	"time"

	"github.com/google/uuid"
)

// ProjectWorkflow represents a synced workflow from dagryn.toml.
type ProjectWorkflow struct {
	ID         uuid.UUID `json:"id" db:"id"`
	ProjectID  uuid.UUID `json:"project_id" db:"project_id"`
	Name       string    `json:"name" db:"name"`
	Version    int       `json:"version" db:"version"`
	IsDefault  bool      `json:"is_default" db:"is_default"`
	ConfigHash *string   `json:"config_hash,omitempty" db:"config_hash"`
	RawConfig  *string   `json:"raw_config,omitempty" db:"raw_config"`
	SyncedAt   time.Time `json:"synced_at" db:"synced_at"`
}

// WorkflowTask represents a task within a workflow.
type WorkflowTask struct {
	ID             uuid.UUID         `json:"id" db:"id"`
	WorkflowID     uuid.UUID         `json:"workflow_id" db:"workflow_id"`
	Name           string            `json:"name" db:"name"`
	Command        string            `json:"command" db:"command"`
	Needs          []string          `json:"needs" db:"needs"`
	Inputs         []string          `json:"inputs" db:"inputs"`
	Outputs        []string          `json:"outputs" db:"outputs"`
	Plugins        []string          `json:"plugins" db:"plugins"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty" db:"timeout_seconds"`
	Workdir        *string           `json:"workdir,omitempty" db:"workdir"`
	Env            map[string]string `json:"env,omitempty" db:"env"`
	GroupName      *string           `json:"group,omitempty" db:"group_name"`
	ConditionExpr  *string           `json:"condition,omitempty" db:"condition_expr"`
}

// WorkflowWithTasks combines a workflow with its tasks.
type WorkflowWithTasks struct {
	ProjectWorkflow
	Tasks []WorkflowTask `json:"tasks"`
}
