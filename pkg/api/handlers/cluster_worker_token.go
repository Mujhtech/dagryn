package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

type createWorkerTokenRequest struct {
	Name      string  `json:"name"`
	TeamID    *string `json:"team_id,omitempty"`
	ClusterID *string `json:"cluster_id,omitempty"`
	ExpiresIn string  `json:"expires_in,omitempty"`
}

type clusterWorkerTokenResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	KeyPrefix   string  `json:"key_prefix"`
	ScopeType   string  `json:"scope_type"`
	TeamID      *string `json:"team_id,omitempty"`
	OwnerUserID *string `json:"owner_user_id,omitempty"`
	ClusterID   *string `json:"cluster_id,omitempty"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	RevokedAt   *string `json:"revoked_at,omitempty"`
}

type clusterWorkerTokenCreatedResponse struct {
	Token clusterWorkerTokenResponse `json:"token"`
	Key   string                     `json:"key"`
}

func (h *Handler) ListClusterWorkerTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	tokens, err := h.store.ClusterWorkerTokens.ListByUser(ctx, user.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list worker tokens"))
		return
	}

	out := make([]clusterWorkerTokenResponse, 0, len(tokens))
	for i := range tokens {
		out = append(out, toClusterWorkerTokenResponse(&tokens[i]))
	}
	_ = response.Ok(w, r, "worker tokens", out)
}

// ListClusterWorkerTokens godoc
//
//	@Summary		List worker tokens
//	@Description	Lists worker registration tokens accessible by the current user
//	@Tags			workers
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{array}		ClusterWorkerTokenResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/v1/workers/tokens [get]

// CreateClusterWorkerToken godoc
//
//	@Summary		Create worker token
//	@Description	Creates a scoped worker registration token for personal or team cluster workers
//	@Tags			workers
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateClusterWorkerTokenRequest	true	"Create worker token request"
//	@Success		201		{object}	ClusterWorkerTokenCreatedResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/workers/tokens [post]
func (h *Handler) CreateClusterWorkerToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req createWorkerTokenRequest
	if err := ParseJSON(r, &req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.Name == "" {
		_ = response.BadRequest(w, r, errors.New("name is required"))
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := parseDuration(req.ExpiresIn)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid expires_in duration"))
			return
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	tok := &models.ClusterWorkerToken{
		Name:            req.Name,
		CreatedByUserID: user.ID,
		ExpiresAt:       expiresAt,
	}

	if req.TeamID != nil && *req.TeamID != "" {
		teamID, err := uuid.Parse(*req.TeamID)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid team_id"))
			return
		}
		if _, err := h.store.Teams.GetMember(ctx, teamID, user.ID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.Forbidden(w, r, errors.New("you are not a member of this team"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to validate team membership"))
			return
		}
		tok.ScopeType = models.ClusterWorkerTokenScopeTeam
		tok.TeamID = &teamID
	} else {
		tok.ScopeType = models.ClusterWorkerTokenScopePersonal
		tok.OwnerUserID = &user.ID
	}

	if req.ClusterID != nil && *req.ClusterID != "" {
		clusterID, err := uuid.Parse(*req.ClusterID)
		if err != nil {
			_ = response.BadRequest(w, r, errors.New("invalid cluster_id"))
			return
		}
		if err := h.authorizeClusterAccess(ctx, user.ID, clusterID); err != nil {
			_ = response.Forbidden(w, r, err)
			return
		}
		cluster, err := h.store.Clusters.GetCluster(ctx, clusterID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				_ = response.NotFound(w, r, errors.New("cluster not found"))
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to resolve cluster"))
			return
		}

		if tok.ScopeType == models.ClusterWorkerTokenScopeTeam {
			if cluster.TeamID == nil || *cluster.TeamID != *tok.TeamID {
				_ = response.BadRequest(w, r, errors.New("cluster_id must belong to the selected team scope"))
				return
			}
		} else {
			if cluster.OwnerUserID == nil || *cluster.OwnerUserID != user.ID {
				_ = response.BadRequest(w, r, errors.New("cluster_id must belong to your personal scope"))
				return
			}
		}
		tok.ClusterID = &clusterID
	}

	raw, err := h.store.ClusterWorkerTokens.Create(ctx, tok)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to create worker token"))
		return
	}

	_ = response.Created(w, r, "worker token created", clusterWorkerTokenCreatedResponse{
		Token: toClusterWorkerTokenResponse(tok),
		Key:   raw,
	})
}

func (h *Handler) RevokeClusterWorkerToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	idStr, err := pathParamOrError(r, WorkerTokenIDParam)
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("worker token ID is required"))
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid worker token ID"))
		return
	}

	tok, err := h.store.ClusterWorkerTokens.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("worker token not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get worker token"))
		return
	}

	allowed := tok.OwnerUserID != nil && *tok.OwnerUserID == user.ID
	if tok.TeamID != nil {
		if _, err := h.store.Teams.GetMember(ctx, *tok.TeamID, user.ID); err == nil {
			allowed = true
		}
	}
	if !allowed {
		_ = response.Forbidden(w, r, errors.New("not authorized to revoke this worker token"))
		return
	}

	if err := h.store.ClusterWorkerTokens.Revoke(ctx, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("worker token not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to revoke worker token"))
		return
	}

	_ = response.NoContent(w, r)
}

// RevokeClusterWorkerToken godoc
//
//	@Summary		Revoke worker token
//	@Description	Revokes a worker registration token
//	@Tags			workers
//	@Security		BearerAuth
//	@Param			workerTokenId	path	string	true	"Worker token ID" format(uuid)
//	@Success		204
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Router			/api/v1/workers/tokens/{workerTokenId} [delete]

func toClusterWorkerTokenResponse(t *models.ClusterWorkerToken) clusterWorkerTokenResponse {
	var teamID, ownerID, clusterID, lastUsed, expiresAt, revokedAt *string
	if t.TeamID != nil {
		s := t.TeamID.String()
		teamID = &s
	}
	if t.OwnerUserID != nil {
		s := t.OwnerUserID.String()
		ownerID = &s
	}
	if t.ClusterID != nil {
		s := t.ClusterID.String()
		clusterID = &s
	}
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.UTC().Format(time.RFC3339)
		lastUsed = &s
	}
	if t.ExpiresAt != nil {
		s := t.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}
	if t.RevokedAt != nil {
		s := t.RevokedAt.UTC().Format(time.RFC3339)
		revokedAt = &s
	}
	return clusterWorkerTokenResponse{
		ID:          t.ID.String(),
		Name:        t.Name,
		KeyPrefix:   t.KeyPrefix,
		ScopeType:   string(t.ScopeType),
		TeamID:      teamID,
		OwnerUserID: ownerID,
		ClusterID:   clusterID,
		LastUsedAt:  lastUsed,
		ExpiresAt:   expiresAt,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
		RevokedAt:   revokedAt,
	}
}
