package handlers

import (
	"context"
	"testing"

	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/stretchr/testify/require"
)

func TestBuildEnvAuditMetadata(t *testing.T) {
	meta := buildEnvAuditMetadata(context.Background(), map[string]any{"action": "set", "count": 2})
	require.NotEmpty(t, meta)
	require.Contains(t, string(meta), "action")
	require.Contains(t, string(meta), "count")
}

func TestEnvPermissionsPresentForAdmin(t *testing.T) {
	perms := models.RolePermissions[models.RoleAdmin]
	require.Contains(t, perms, models.PermissionEnvView)
	require.Contains(t, perms, models.PermissionEnvManage)
	require.Contains(t, perms, models.PermissionEnvRotate)
	require.Contains(t, perms, models.PermissionEnvReveal)
}
