package repo

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/internal/db/models"
	"github.com/stretchr/testify/assert"
)

func TestArtifact_Model(t *testing.T) {
	now := time.Now()
	taskName := "build"
	digest := "deadbeef"
	artifact := models.Artifact{
		ID:           uuid.New(),
		ProjectID:    uuid.New(),
		RunID:        uuid.New(),
		TaskName:     &taskName,
		Name:         "dist/app",
		FileName:     "app",
		ContentType:  "application/octet-stream",
		SizeBytes:    1024,
		StorageKey:   "artifacts/project/run/task/id/app",
		DigestSHA256: &digest,
		ExpiresAt:    &now,
		Metadata:     json.RawMessage(`{"path":"dist/app"}`),
		CreatedAt:    now,
	}

	assert.NotEqual(t, uuid.Nil, artifact.ID)
	assert.Equal(t, "build", *artifact.TaskName)
	assert.Equal(t, int64(1024), artifact.SizeBytes)
	assert.NotNil(t, artifact.DigestSHA256)
	assert.NotNil(t, artifact.Metadata)
}
