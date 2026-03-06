package client

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRunStore_CreateAndGetRun(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		ProjectID: uuid.New(),
		ServerURL: "http://localhost:9000",
		Targets:   []string{"build", "test"},
		GitBranch: "main",
		GitCommit: "abc1234",
		StartedAt: time.Now(),
		Status:    "running",
	}

	// Create
	err := store.CreateRun(run)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Verify directory was created
	runDir := store.RunPath(run.RunID)
	if _, err := os.Stat(runDir); os.IsNotExist(err) {
		t.Error("run directory was not created")
	}

	// Get
	retrieved, err := store.GetRun(run.RunID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetRun returned nil")
	}

	// Verify fields
	if retrieved.RunID != run.RunID {
		t.Errorf("RunID mismatch: got %s, want %s", retrieved.RunID, run.RunID)
	}
	if retrieved.ProjectID != run.ProjectID {
		t.Errorf("ProjectID mismatch")
	}
	if retrieved.ServerURL != run.ServerURL {
		t.Errorf("ServerURL mismatch")
	}
	if len(retrieved.Targets) != 2 || retrieved.Targets[0] != "build" {
		t.Errorf("Targets mismatch")
	}
	if retrieved.GitBranch != "main" {
		t.Errorf("GitBranch mismatch")
	}
	if retrieved.Status != "running" {
		t.Errorf("Status mismatch")
	}
}

func TestRunStore_UpdateRun(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		ProjectID: uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}

	// Create
	if err := store.CreateRun(run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update
	now := time.Now()
	run.FinishedAt = &now
	run.Status = "success"
	run.PendingSync = true

	if err := store.UpdateRun(run); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	// Verify
	retrieved, _ := store.GetRun(run.RunID)
	if retrieved.Status != "success" {
		t.Errorf("Status not updated")
	}
	if retrieved.FinishedAt == nil {
		t.Error("FinishedAt not set")
	}
	if !retrieved.PendingSync {
		t.Error("PendingSync not set")
	}
}

func TestRunStore_UpdateNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:  uuid.New(),
		Status: "running",
	}

	err := store.UpdateRun(run)
	if err == nil {
		t.Error("expected error when updating non-existent run")
	}
}

func TestRunStore_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run, err := store.GetRun(uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run != nil {
		t.Error("expected nil for non-existent run")
	}
}

func TestRunStore_DeleteRun(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}

	// Create
	if err := store.CreateRun(run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Add some logs
	entry := &RunLogEntry{
		Timestamp: time.Now(),
		TaskName:  "build",
		Stream:    "stdout",
		Line:      "hello",
	}
	if err := store.AppendLog(run.RunID, entry); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	// Delete
	if err := store.DeleteRun(run.RunID); err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Verify deleted
	runDir := store.RunPath(run.RunID)
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Error("run directory still exists after delete")
	}

	// GetRun should return nil
	retrieved, err := store.GetRun(run.RunID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if retrieved != nil {
		t.Error("GetRun should return nil after delete")
	}
}

func TestRunStore_AppendAndReadLogs(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}
	if err := store.CreateRun(run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Append logs
	entries := []*RunLogEntry{
		{Timestamp: time.Now(), TaskName: "build", Stream: "stdout", Line: "line 1"},
		{Timestamp: time.Now(), TaskName: "build", Stream: "stdout", Line: "line 2"},
		{Timestamp: time.Now(), TaskName: "build", Stream: "stderr", Line: "warning"},
	}

	for _, e := range entries {
		if err := store.AppendLog(run.RunID, e); err != nil {
			t.Fatalf("AppendLog failed: %v", err)
		}
	}

	// Flush and close
	_ = store.FlushLogs(run.RunID)
	_ = store.CloseLogs(run.RunID)

	// Read logs
	logs, err := store.ReadLogs(run.RunID)
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}

	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	if logs[0].Line != "line 1" {
		t.Errorf("log 0: expected 'line 1', got %q", logs[0].Line)
	}
	if logs[2].Stream != "stderr" {
		t.Errorf("log 2: expected 'stderr', got %q", logs[2].Stream)
	}
}

func TestRunStore_AppendLogsBatch(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}
	if err := store.CreateRun(run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Append batch
	entries := []*RunLogEntry{
		{Timestamp: time.Now(), TaskName: "build", Stream: "stdout", Line: "batch 1"},
		{Timestamp: time.Now(), TaskName: "build", Stream: "stdout", Line: "batch 2"},
	}

	if err := store.AppendLogs(run.RunID, entries); err != nil {
		t.Fatalf("AppendLogs failed: %v", err)
	}

	_ = store.CloseLogs(run.RunID)

	// Read
	logs, _ := store.ReadLogs(run.RunID)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestRunStore_ReadLogsNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	logs, err := store.ReadLogs(uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs != nil {
		t.Error("expected nil for non-existent logs")
	}
}

func TestRunStore_ListRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	// Create multiple runs with different times
	runs := []*LocalRun{
		{RunID: uuid.New(), StartedAt: time.Now().Add(-2 * time.Hour), Status: "success"},
		{RunID: uuid.New(), StartedAt: time.Now().Add(-1 * time.Hour), Status: "failed"},
		{RunID: uuid.New(), StartedAt: time.Now(), Status: "running"},
	}

	for _, r := range runs {
		if err := store.CreateRun(r); err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
	}

	// List
	listed, err := store.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(listed))
	}

	// Should be sorted newest first
	if listed[0].Status != "running" {
		t.Errorf("expected newest run first, got status %s", listed[0].Status)
	}
	if listed[2].Status != "success" {
		t.Errorf("expected oldest run last, got status %s", listed[2].Status)
	}
}

func TestRunStore_ListPendingRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	runs := []*LocalRun{
		{RunID: uuid.New(), StartedAt: time.Now(), Status: "success", PendingSync: false},
		{RunID: uuid.New(), StartedAt: time.Now(), Status: "failed", PendingSync: true},
		{RunID: uuid.New(), StartedAt: time.Now(), Status: "success", PendingSync: true},
	}

	for _, r := range runs {
		_ = store.CreateRun(r)
	}

	pending, err := store.ListPendingRuns()
	if err != nil {
		t.Fatalf("ListPendingRuns failed: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("expected 2 pending runs, got %d", len(pending))
	}
}

func TestRunStore_MarkSynced(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:       uuid.New(),
		StartedAt:   time.Now(),
		Status:      "success",
		PendingSync: true,
	}
	_ = store.CreateRun(run)

	// Mark synced
	if err := store.MarkSynced(run.RunID); err != nil {
		t.Fatalf("MarkSynced failed: %v", err)
	}

	// Verify
	retrieved, _ := store.GetRun(run.RunID)
	if retrieved.PendingSync {
		t.Error("run should not be pending sync")
	}
}

func TestRunStore_CleanOldRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	runs := []*LocalRun{
		{RunID: uuid.New(), StartedAt: time.Now().Add(-48 * time.Hour), Status: "success"},
		{RunID: uuid.New(), StartedAt: time.Now().Add(-25 * time.Hour), Status: "success"},
		{RunID: uuid.New(), StartedAt: time.Now().Add(-1 * time.Hour), Status: "running"},
	}

	for _, r := range runs {
		_ = store.CreateRun(r)
	}

	// Clean runs older than 24 hours
	deleted, err := store.CleanOldRuns(24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanOldRuns failed: %v", err)
	}

	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify
	remaining, _ := store.ListRuns()
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestRunStore_CleanExcessRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	for i := 0; i < 5; i++ {
		run := &LocalRun{
			RunID:     uuid.New(),
			StartedAt: time.Now().Add(time.Duration(-i) * time.Hour),
			Status:    "success",
		}
		_ = store.CreateRun(run)
	}

	// Keep only 2
	deleted, err := store.CleanExcessRuns(2)
	if err != nil {
		t.Fatalf("CleanExcessRuns failed: %v", err)
	}

	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	remaining, _ := store.ListRuns()
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(remaining))
	}
}

func TestRunStore_ConcurrentLogAppend(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}
	_ = store.CreateRun(run)

	// Concurrent appends
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			entry := &RunLogEntry{
				Timestamp: time.Now(),
				TaskName:  "build",
				Stream:    "stdout",
				Line:      "concurrent line",
			}
			_ = store.AppendLog(run.RunID, entry)
		}(i)
	}
	wg.Wait()

	if err := store.CloseLogs(run.RunID); err != nil {
		t.Fatalf("CloseLogs failed: %v", err)
	}

	// Verify all logs were written
	logs, _ := store.ReadLogs(run.RunID)
	if len(logs) != 100 {
		t.Errorf("expected 100 logs, got %d", len(logs))
	}
}

func TestRunStore_Paths(t *testing.T) {
	store := NewRunStore("/project")
	runID := uuid.MustParse("12345678-1234-1234-1234-123456789012")

	runsPath := store.RunsPath()
	expected := filepath.Join("/project", ".dagryn", "runs")
	if runsPath != expected {
		t.Errorf("RunsPath: expected %s, got %s", expected, runsPath)
	}

	runPath := store.RunPath(runID)
	expected = filepath.Join("/project", ".dagryn", "runs", runID.String())
	if runPath != expected {
		t.Errorf("RunPath: expected %s, got %s", expected, runPath)
	}

	metaPath := store.MetadataPath(runID)
	expected = filepath.Join("/project", ".dagryn", "runs", runID.String(), "run.json")
	if metaPath != expected {
		t.Errorf("MetadataPath: expected %s, got %s", expected, metaPath)
	}

	logsPath := store.LogsPath(runID)
	expected = filepath.Join("/project", ".dagryn", "runs", runID.String(), "logs.jsonl")
	if logsPath != expected {
		t.Errorf("LogsPath: expected %s, got %s", expected, logsPath)
	}
}

func TestRunStore_LongLogLine(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRunStore(tmpDir)
	defer func() { _ = store.Close() }()

	run := &LocalRun{
		RunID:     uuid.New(),
		StartedAt: time.Now(),
		Status:    "running",
	}
	_ = store.CreateRun(run)

	// Create a very long line (1MB)
	longLine := make([]byte, 1024*1024)
	for i := range longLine {
		longLine[i] = 'x'
	}

	entry := &RunLogEntry{
		Timestamp: time.Now(),
		TaskName:  "build",
		Stream:    "stdout",
		Line:      string(longLine),
	}

	if err := store.AppendLog(run.RunID, entry); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	_ = store.CloseLogs(run.RunID)

	logs, err := store.ReadLogs(run.RunID)
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if len(logs[0].Line) != 1024*1024 {
		t.Errorf("expected line length %d, got %d", 1024*1024, len(logs[0].Line))
	}
}
