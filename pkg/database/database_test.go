package database

import (
	"testing"
	"testing/fstest"
)

func TestRegisterMigrations_MergesAndSorts(t *testing.T) {
	ResetExtraMigrations()
	defer ResetExtraMigrations()

	// Load baseline (core-only) migrations
	baseline, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() baseline error: %v", err)
	}
	baselineCount := len(baseline)

	// Register extra test migrations (versions 900, 901)
	extraFS := fstest.MapFS{
		"migrations/900_test_extra_a.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test_extra_a (id INT);"),
		},
		"migrations/901_test_extra_b.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test_extra_b (id INT);"),
		},
	}
	RegisterMigrations(extraFS)

	merged, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() after register error: %v", err)
	}

	// Should have 2 more migrations than baseline
	if len(merged) != baselineCount+2 {
		t.Errorf("expected %d migrations, got %d", baselineCount+2, len(merged))
	}

	// Verify they are sorted by version
	for i := 1; i < len(merged); i++ {
		if merged[i].Version <= merged[i-1].Version {
			t.Errorf("migrations not sorted: version %d at index %d follows version %d at index %d",
				merged[i].Version, i, merged[i-1].Version, i-1)
		}
	}

	// Verify the extra migrations are present at the end (high version numbers)
	last := merged[len(merged)-1]
	secondLast := merged[len(merged)-2]
	if secondLast.Version != 900 || last.Version != 901 {
		t.Errorf("expected last two versions to be 900, 901; got %d, %d",
			secondLast.Version, last.Version)
	}
	if secondLast.Name != "test_extra_a" {
		t.Errorf("expected name 'test_extra_a', got %q", secondLast.Name)
	}
	if last.Name != "test_extra_b" {
		t.Errorf("expected name 'test_extra_b', got %q", last.Name)
	}

	// Verify SQL content was loaded
	if secondLast.SQL == "" || last.SQL == "" {
		t.Error("expected non-empty SQL content for extra migrations")
	}
}

func TestRegisterMigrations_DetectsConflict(t *testing.T) {
	ResetExtraMigrations()
	defer ResetExtraMigrations()

	// Register FS that contains version 001 which conflicts with core migration 001_users
	conflictFS := fstest.MapFS{
		"migrations/001_conflict_users.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE conflict_test (id INT);"),
		},
	}
	RegisterMigrations(conflictFS)

	_, err := loadMigrations()
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !containsSubstr(err.Error(), "conflict") {
		t.Errorf("expected error to mention 'conflict', got: %v", err)
	}
}

func TestLoadMigrations_SkipsEmptyExtra(t *testing.T) {
	ResetExtraMigrations()
	defer ResetExtraMigrations()

	// Load baseline
	baseline, err := loadMigrations()
	if err != nil {
		t.Fatalf("baseline error: %v", err)
	}

	// Register an empty FS (no migrations/ dir)
	emptyFS := fstest.MapFS{}
	RegisterMigrations(emptyFS)

	afterEmpty, err := loadMigrations()
	if err != nil {
		t.Fatalf("expected no error with empty extra FS, got: %v", err)
	}

	if len(afterEmpty) != len(baseline) {
		t.Errorf("expected %d migrations (unchanged), got %d", len(baseline), len(afterEmpty))
	}
}

func TestLoadDownMigration_SearchesExtraSources(t *testing.T) {
	ResetExtraMigrations()
	defer ResetExtraMigrations()

	// Without extra sources, version 900 should not be found
	_, err := loadDownMigration(900)
	if err == nil {
		t.Fatal("expected error for version 900 without extra sources")
	}

	// Register extra migrations that include a .down.sql for version 900
	extraFS := fstest.MapFS{
		"migrations/900_test_extra_a.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test_extra_a (id INT);"),
		},
		"migrations/900_test_extra_a.down.sql": &fstest.MapFile{
			Data: []byte("DROP TABLE IF EXISTS test_extra_a;"),
		},
	}
	RegisterMigrations(extraFS)

	sql, err := loadDownMigration(900)
	if err != nil {
		t.Fatalf("expected to find down migration for version 900, got: %v", err)
	}
	if sql == "" {
		t.Error("expected non-empty down migration SQL")
	}
	if sql != "DROP TABLE IF EXISTS test_extra_a;" {
		t.Errorf("unexpected down migration content: %q", sql)
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
