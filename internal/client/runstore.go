package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DagrynDir is the local configuration directory name.
	DagrynDir = ".dagryn"
	// RunsDir is the directory name for storing local runs.
	RunsDir = "runs"
	// RunMetadataFile is the filename for run metadata.
	RunMetadataFile = "run.json"
	// RunLogsFile is the filename for run logs (JSONL format).
	RunLogsFile = "logs.jsonl"
)

// RunLogEntry represents a single log line stored locally.
type RunLogEntry struct {
	Timestamp time.Time `json:"ts"`
	TaskName  string    `json:"task"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
	Line      string    `json:"line"`
}

// LocalRun represents metadata for a locally-stored run.
type LocalRun struct {
	RunID       uuid.UUID  `json:"run_id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ServerURL   string     `json:"server_url"`
	Targets     []string   `json:"targets"`
	GitBranch   string     `json:"git_branch,omitempty"`
	GitCommit   string     `json:"git_commit,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Status      string     `json:"status"` // running, success, failed, cancelled
	PendingSync bool       `json:"pending_sync"`
	ErrorMsg    string     `json:"error_message,omitempty"`
}

// RunStore manages local run data in .dagryn/runs/
type RunStore struct {
	root   string
	mu     sync.Mutex             // Protects file operations
	logMu  sync.Mutex             // Protects log file writes
	logFDs map[uuid.UUID]*os.File // Open log file descriptors
}

// NewRunStore creates a new run store.
func NewRunStore(projectRoot string) *RunStore {
	return &RunStore{
		root:   projectRoot,
		logFDs: make(map[uuid.UUID]*os.File),
	}
}

// --- Path helpers ---

// RunsPath returns the path to the runs directory.
func (s *RunStore) RunsPath() string {
	return filepath.Join(s.root, DagrynDir, RunsDir)
}

// RunPath returns the path to a specific run's directory.
func (s *RunStore) RunPath(runID uuid.UUID) string {
	return filepath.Join(s.RunsPath(), runID.String())
}

// MetadataPath returns the path to a run's metadata file.
func (s *RunStore) MetadataPath(runID uuid.UUID) string {
	return filepath.Join(s.RunPath(runID), RunMetadataFile)
}

// LogsPath returns the path to a run's logs file.
func (s *RunStore) LogsPath(runID uuid.UUID) string {
	return filepath.Join(s.RunPath(runID), RunLogsFile)
}

// --- CRUD operations ---

// CreateRun creates a new local run record.
func (s *RunStore) CreateRun(run *LocalRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create run directory
	runDir := s.RunPath(run.RunID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return fmt.Errorf("failed to create run directory: %w", err)
	}

	// Write metadata
	return s.writeMetadataLocked(run)
}

// UpdateRun updates an existing local run record.
func (s *RunStore) UpdateRun(run *LocalRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if run exists
	if _, err := os.Stat(s.RunPath(run.RunID)); os.IsNotExist(err) {
		return fmt.Errorf("run %s does not exist", run.RunID)
	}

	return s.writeMetadataLocked(run)
}

// GetRun retrieves a local run by ID.
func (s *RunStore) GetRun(runID uuid.UUID) (*LocalRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaPath := s.MetadataPath(runID)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to read run metadata: %w", err)
	}

	var run LocalRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("failed to parse run metadata: %w", err)
	}

	return &run, nil
}

// DeleteRun removes a local run and its logs.
func (s *RunStore) DeleteRun(runID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close any open log file descriptor
	s.closeLogFDLocked(runID)

	runDir := s.RunPath(runID)
	if err := os.RemoveAll(runDir); err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	return nil
}

// --- Log operations ---

// AppendLog appends a log entry to a run's log file.
// This is optimized for high-frequency appends.
func (s *RunStore) AppendLog(runID uuid.UUID, entry *RunLogEntry) error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	// Get or create log file descriptor
	fd, err := s.getOrCreateLogFD(runID)
	if err != nil {
		return err
	}

	// Encode entry as JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Append with newline
	data = append(data, '\n')
	if _, err := fd.Write(data); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// AppendLogs appends multiple log entries atomically.
func (s *RunStore) AppendLogs(runID uuid.UUID, entries []*RunLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()

	fd, err := s.getOrCreateLogFD(runID)
	if err != nil {
		return err
	}

	// Build batch write
	var buf []byte
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		buf = append(buf, data...)
		buf = append(buf, '\n')
	}

	if _, err := fd.Write(buf); err != nil {
		return fmt.Errorf("failed to write log entries: %w", err)
	}

	return nil
}

// ReadLogs reads all log entries for a run.
func (s *RunStore) ReadLogs(runID uuid.UUID) ([]RunLogEntry, error) {
	logsPath := s.LogsPath(runID)

	file, err := os.Open(logsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No logs yet
		}
		return nil, fmt.Errorf("failed to open logs file: %w", err)
	}
	defer file.Close()

	var entries []RunLogEntry
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long lines
	const maxLineSize = 10 * 1024 * 1024 // 10MB max line
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		var entry RunLogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip malformed lines
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	return entries, nil
}

// FlushLogs ensures all buffered log data is written to disk.
func (s *RunStore) FlushLogs(runID uuid.UUID) error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if fd, ok := s.logFDs[runID]; ok {
		return fd.Sync()
	}
	return nil
}

// CloseLogs closes the log file for a run.
func (s *RunStore) CloseLogs(runID uuid.UUID) error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	return s.closeLogFDLocked(runID)
}

// --- Query operations ---

// ListRuns returns all local runs, sorted by start time (newest first).
func (s *RunStore) ListRuns() ([]*LocalRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runsDir := s.RunsPath()
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}

	var runs []*LocalRun
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runID, err := uuid.Parse(entry.Name())
		if err != nil {
			// Skip non-UUID directories
			continue
		}

		metaPath := s.MetadataPath(runID)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue // Skip runs without metadata
		}

		var run LocalRun
		if err := json.Unmarshal(data, &run); err != nil {
			continue // Skip malformed metadata
		}

		runs = append(runs, &run)
	}

	// Sort by start time, newest first
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})

	return runs, nil
}

// ListPendingRuns returns runs that need to be synced to the server.
func (s *RunStore) ListPendingRuns() ([]*LocalRun, error) {
	runs, err := s.ListRuns()
	if err != nil {
		return nil, err
	}

	var pending []*LocalRun
	for _, run := range runs {
		if run.PendingSync {
			pending = append(pending, run)
		}
	}

	return pending, nil
}

// MarkSynced marks a run as successfully synced.
func (s *RunStore) MarkSynced(runID uuid.UUID) error {
	run, err := s.GetRun(runID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run %s not found", runID)
	}

	run.PendingSync = false
	return s.UpdateRun(run)
}

// --- Cleanup operations ---

// CleanOldRuns removes runs older than maxAge.
// Returns the number of runs deleted.
func (s *RunStore) CleanOldRuns(maxAge time.Duration) (int, error) {
	runs, err := s.ListRuns()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, run := range runs {
		if run.StartedAt.Before(cutoff) {
			if err := s.DeleteRun(run.RunID); err != nil {
				continue // Skip failed deletes
			}
			deleted++
		}
	}

	return deleted, nil
}

// CleanExcessRuns keeps only the most recent N runs.
// Returns the number of runs deleted.
func (s *RunStore) CleanExcessRuns(keepCount int) (int, error) {
	runs, err := s.ListRuns()
	if err != nil {
		return 0, err
	}

	if len(runs) <= keepCount {
		return 0, nil
	}

	// Runs are already sorted newest first
	deleted := 0
	for i := keepCount; i < len(runs); i++ {
		if err := s.DeleteRun(runs[i].RunID); err != nil {
			continue // Skip failed deletes
		}
		deleted++
	}

	return deleted, nil
}

// --- Internal helpers ---

// writeMetadataLocked writes run metadata to disk.
// Must be called with s.mu held.
func (s *RunStore) writeMetadataLocked(run *LocalRun) error {
	metaPath := s.MetadataPath(run.RunID)
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write run metadata: %w", err)
	}

	return nil
}

// getOrCreateLogFD gets or creates a file descriptor for appending logs.
// Must be called with s.logMu held.
func (s *RunStore) getOrCreateLogFD(runID uuid.UUID) (*os.File, error) {
	if fd, ok := s.logFDs[runID]; ok {
		return fd, nil
	}

	logsPath := s.LogsPath(runID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logsPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Open file for appending
	fd, err := os.OpenFile(logsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open logs file: %w", err)
	}

	s.logFDs[runID] = fd
	return fd, nil
}

// closeLogFDLocked closes the log file descriptor for a run.
// Must be called with s.logMu held.
func (s *RunStore) closeLogFDLocked(runID uuid.UUID) error {
	if fd, ok := s.logFDs[runID]; ok {
		delete(s.logFDs, runID)
		return fd.Close()
	}
	return nil
}

// Close closes all open log file descriptors.
func (s *RunStore) Close() error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	var lastErr error
	for runID := range s.logFDs {
		if err := s.closeLogFDLocked(runID); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
