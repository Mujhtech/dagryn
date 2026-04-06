package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Cluster represents a logical grouping of workers.
type Cluster struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	Description   string          `json:"description"`
	Labels        json.RawMessage `json:"labels"`
	ScopeType     string          `json:"scope_type"`
	TeamID        *uuid.UUID      `json:"team_id,omitempty"`
	OwnerUserID   *uuid.UUID      `json:"owner_user_id,omitempty"`
	SystemDefault bool            `json:"system_default"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// WorkerStatus represents the lifecycle status of a worker.
type WorkerStatus string

const (
	WorkerStatusOnline   WorkerStatus = "online"
	WorkerStatusDraining WorkerStatus = "draining"
	WorkerStatusOffline  WorkerStatus = "offline"
)

// Worker represents a registered agent node.
type Worker struct {
	ID                 uuid.UUID       `json:"id"`
	Hostname           string          `json:"hostname"`
	OS                 string          `json:"os"`
	Arch               string          `json:"arch"`
	Environment        string          `json:"environment"`
	Labels             json.RawMessage `json:"labels"`
	Capabilities       []string        `json:"capabilities"`
	MaxConcurrentTasks int             `json:"max_concurrent_tasks"`
	Version            string          `json:"version"`
	Status             WorkerStatus    `json:"status"`
	LastHeartbeatAt    time.Time       `json:"last_heartbeat_at"`
	RegisteredAt       time.Time       `json:"registered_at"`
	AuthTokenHash      string          `json:"-"`
	ClusterID          *uuid.UUID      `json:"cluster_id,omitempty"`
	CPUMillicoresAvail int64           `json:"cpu_millicores_available"`
	MemoryBytesAvail   int64           `json:"memory_bytes_available"`
	DiskBytesAvail     int64           `json:"disk_bytes_available"`
	CPUUsagePercent    float64         `json:"cpu_usage_percent"`
	MemoryUsagePercent float64         `json:"memory_usage_percent"`
	ActiveTasks        int             `json:"active_tasks"`
	ResourcesUpdatedAt time.Time       `json:"resources_updated_at"`
}

// TaskAssignmentStatus represents the status of a task assignment.
type TaskAssignmentStatus string

const (
	TaskAssignmentPending    TaskAssignmentStatus = "pending"
	TaskAssignmentAssigned   TaskAssignmentStatus = "assigned"
	TaskAssignmentRunning    TaskAssignmentStatus = "running"
	TaskAssignmentCompleted  TaskAssignmentStatus = "completed"
	TaskAssignmentFailed     TaskAssignmentStatus = "failed"
	TaskAssignmentReassigned TaskAssignmentStatus = "reassigned"
)

// TaskAssignment tracks distributed task dispatch.
type TaskAssignment struct {
	ID          uuid.UUID            `json:"id"`
	RunID       uuid.UUID            `json:"run_id"`
	TaskName    string               `json:"task_name"`
	WorkerID    *uuid.UUID           `json:"worker_id,omitempty"`
	ClusterID   *uuid.UUID           `json:"cluster_id,omitempty"`
	Status      TaskAssignmentStatus `json:"status"`
	AssignedAt  *time.Time           `json:"assigned_at,omitempty"`
	StartedAt   *time.Time           `json:"started_at,omitempty"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
	Result      json.RawMessage      `json:"result,omitempty"`
	RetryCount  int                  `json:"retry_count"`
	MaxRetries  int                  `json:"max_retries"`
	CreatedAt   time.Time            `json:"created_at"`
}
