package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/encrypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProjectEnvStore struct {
	repo.ProjectEnvStore
	resolved      []repo.ResolvedEnvVar
	resolveErr    error
	resolvedParam repo.EnvResolutionParams
	secretRecords map[uuid.UUID]*models.SecretRecord
}

func (m *mockProjectEnvStore) ResolveEnv(_ context.Context, params repo.EnvResolutionParams) ([]repo.ResolvedEnvVar, error) {
	m.resolvedParam = params
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	return m.resolved, nil
}

func (m *mockProjectEnvStore) GetSecretRecordByID(_ context.Context, id uuid.UUID) (*models.SecretRecord, error) {
	if rec, ok := m.secretRecords[id]; ok {
		return rec, nil
	}
	return nil, repo.ErrNotFound
}

func TestResolveProjectEnvForRun_EnvironmentSelection(t *testing.T) {
	projectID := uuid.New()
	branch := "feat/test"
	runEnv := "staging"
	run := &models.Run{Environment: &runEnv}

	t.Run("requested environment has highest precedence", func(t *testing.T) {
		t.Setenv("DAGRYN_RUN_ENV", "prod")
		store := &mockProjectEnvStore{}
		h := &ExecuteRunHandler{projectEnv: store}

		_, warnings := h.resolveProjectEnvForRun(context.Background(), run, projectID, "qa", branch)
		require.Empty(t, warnings)
		assert.Equal(t, "qa", store.resolvedParam.Environment)
		assert.Equal(t, branch, store.resolvedParam.Branch)
	})

	t.Run("run environment beats DAGRYN_RUN_ENV", func(t *testing.T) {
		t.Setenv("DAGRYN_RUN_ENV", "prod")
		store := &mockProjectEnvStore{}
		h := &ExecuteRunHandler{projectEnv: store}

		_, warnings := h.resolveProjectEnvForRun(context.Background(), run, projectID, "", branch)
		require.Empty(t, warnings)
		assert.Equal(t, "staging", store.resolvedParam.Environment)
		assert.Equal(t, branch, store.resolvedParam.Branch)
	})

	t.Run("defaults to dev when no scope is provided", func(t *testing.T) {
		t.Setenv("DAGRYN_RUN_ENV", "")
		store := &mockProjectEnvStore{}
		h := &ExecuteRunHandler{projectEnv: store}

		_, warnings := h.resolveProjectEnvForRun(context.Background(), &models.Run{}, projectID, "", branch)
		require.Empty(t, warnings)
		assert.Equal(t, "dev", store.resolvedParam.Environment)
		assert.Equal(t, branch, store.resolvedParam.Branch)
	})
}

func TestResolveProjectEnvForRun_ResolvesValuesAndWarnings(t *testing.T) {
	projectID := uuid.New()
	secretID := uuid.New()
	secretCipher, err := encrypt.NewNoOpEncrypt().Encrypt([]byte("super-secret"))
	require.NoError(t, err)

	store := &mockProjectEnvStore{
		resolved: []repo.ResolvedEnvVar{
			{Key: "PLAIN_OK", ValueType: models.EnvValueTypePlain, PlainValue: testStringPtr("hello")},
			{Key: "PLAIN_REQUIRED_MISSING", ValueType: models.EnvValueTypePlain, Required: true},
			{Key: "SECRET_DB_OK", ValueType: models.EnvValueTypeSecret, Provider: models.SecretProviderDB, SecretRecordID: &secretID, Required: true},
			{Key: "SECRET_DB_REQUIRED_MISSING", ValueType: models.EnvValueTypeSecret, Provider: models.SecretProviderDB, Required: true},
			{Key: "SECRET_PROVIDER_REF_MISSING", ValueType: models.EnvValueTypeSecret, Provider: models.SecretProviderAWS, Required: true},
		},
		secretRecords: map[uuid.UUID]*models.SecretRecord{
			secretID: {
				ID:         secretID,
				Provider:   models.SecretProviderDB,
				Ciphertext: []byte(secretCipher),
			},
		},
	}

	h := &ExecuteRunHandler{
		projectEnv:          store,
		encrypter:           encrypt.NewNoOpEncrypt(),
		envSecretsProvider:  string(models.SecretProviderDB),
		envSecretsAWSRegion: "us-east-1",
	}

	resolved, warnings := h.resolveProjectEnvForRun(context.Background(), nil, projectID, "dev", "main")

	assert.Equal(t, "hello", resolved["PLAIN_OK"])
	assert.Equal(t, "super-secret", resolved["SECRET_DB_OK"])
	assert.NotContains(t, resolved, "PLAIN_REQUIRED_MISSING")
	assert.NotContains(t, resolved, "SECRET_DB_REQUIRED_MISSING")
	assert.NotContains(t, resolved, "SECRET_PROVIDER_REF_MISSING")

	assert.Contains(t, warnings, "missing required plain env: PLAIN_REQUIRED_MISSING")
	assert.Contains(t, warnings, "missing required db secret record: SECRET_DB_REQUIRED_MISSING")
	assert.Contains(t, warnings, "missing provider_ref for secret: SECRET_PROVIDER_REF_MISSING")
}

func TestResolveProjectEnvForRun_WarnsWhenDBSecretCannotDecrypt(t *testing.T) {
	projectID := uuid.New()
	secretID := uuid.New()

	store := &mockProjectEnvStore{
		resolved: []repo.ResolvedEnvVar{
			{Key: "SECRET_DB", ValueType: models.EnvValueTypeSecret, Provider: models.SecretProviderDB, SecretRecordID: &secretID, Required: true},
		},
		secretRecords: map[uuid.UUID]*models.SecretRecord{
			secretID: {
				ID:         secretID,
				Provider:   models.SecretProviderDB,
				Ciphertext: []byte("not-base64"),
			},
		},
	}

	h := &ExecuteRunHandler{projectEnv: store, encrypter: encrypt.NewNoOpEncrypt()}

	resolved, warnings := h.resolveProjectEnvForRun(context.Background(), nil, projectID, "dev", "main")
	assert.Empty(t, resolved)
	assert.Contains(t, warnings, "failed decrypting db secret: SECRET_DB")
}

func testStringPtr(s string) *string {
	return &s
}
