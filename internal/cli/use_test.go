package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContextProjectID(t *testing.T) {
	dir := t.TempDir()
	dagrynDir := filepath.Join(dir, ".dagryn")
	require.NoError(t, os.MkdirAll(dagrynDir, 0755))

	// No file → empty string
	assert.Equal(t, "", loadContextProjectID(dir))

	// Write a context file
	ctx := contextConfig{
		ProjectID:   "550e8400-e29b-41d4-a716-446655440000",
		ProjectName: "test-project",
		SetAt:       "2025-01-01T00:00:00Z",
	}
	data, err := json.MarshalIndent(ctx, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dagrynDir, "context.json"), data, 0644))

	// Now it should return the project ID
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", loadContextProjectID(dir))
}

func TestLoadContextProjectIDInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	dagrynDir := filepath.Join(dir, ".dagryn")
	require.NoError(t, os.MkdirAll(dagrynDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dagrynDir, "context.json"), []byte("not json"), 0644))
	assert.Equal(t, "", loadContextProjectID(dir))
}
