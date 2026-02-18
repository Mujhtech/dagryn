package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/entitlement"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListPendingInvitations godoc
//
//	@Summary		List pending invitations
//	@Description	Returns all pending invitations for the current user
//	@Tags			invitations
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	[]InvitationResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/v1/invitations [get]
func (h *Handler) ListPendingInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	invitations, err := h.store.Invitations.ListPendingByEmail(ctx, user.Email)
	if err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to list invitations"))
		return
	}

	resp := make([]InvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		r := invitationWithDetailsToResponse(&inv)
		r.AcceptToken = inv.Token // So the client can call accept/decline with this token
		resp = append(resp, r)
	}

	_ = response.Ok(w, r, "Success", resp)
}

// AcceptInvitation godoc
//
//	@Summary		Accept invitation
//	@Description	Accepts a pending invitation to join a team or project
//	@Tags			invitations
//	@Security		BearerAuth
//	@Param			token	path		string	true	"Invitation token"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		410		{object}	ErrorResponse	"Invitation expired"
//	@Router			/api/v1/invitations/{token}/accept [post]
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		_ = response.BadRequest(w, r, errors.New("invitation token is required"))
		return
	}

	// Get the invitation
	inv, err := h.store.Invitations.GetPendingByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("invitation not found or already accepted/expired"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get invitation"))
		return
	}

	// Verify the invitation is for this user's email
	if inv.Email != user.Email {
		_ = response.Forbidden(w, r, errors.New("this invitation was sent to a different email address"))
		return
	}

	// Check if expired
	if inv.IsExpired() {
		_ = response.Gone(w, r, errors.New("this invitation has expired"))
		return
	}

	// Check team members quota before accepting
	if h.entitlements != nil {
		var quotaProjectID uuid.UUID
		if inv.ProjectID != nil {
			quotaProjectID = *inv.ProjectID
		}
		if err := h.entitlements.CheckQuota(ctx, "team_members", quotaProjectID, 0); err != nil {
			if entitlement.IsQuotaError(err) {
				_ = response.PaymentRequired(w, r, err)
				return
			}
			_ = response.InternalServerError(w, r, errors.New("failed to check quota"))
			return
		}
	}

	// Accept the invitation based on type
	if inv.TeamID != nil {
		// Team invitation - add user to team
		err = h.store.Teams.AddMember(ctx, *inv.TeamID, user.ID, inv.Role, &inv.InvitedBy)
		if err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to add you to the team"))
			return
		}
	} else if inv.ProjectID != nil {
		// Project invitation - add user to project
		err = h.store.Projects.AddMember(ctx, *inv.ProjectID, user.ID, inv.Role, &inv.InvitedBy)
		if err != nil {
			_ = response.InternalServerError(w, r, errors.New("failed to add you to the project"))
			return
		}
	}

	// Mark invitation as accepted
	if err := h.store.Invitations.Accept(ctx, token); err != nil {
		// User was already added, so this is a minor error
		// Log it but don't fail the request
		slog.Error("failed to accept invitation", "error", err)
	}

	_ = response.Ok(w, r, "Success", SuccessResponse{
		Message: "Invitation accepted successfully",
	})
}

// DeclineInvitation godoc
//
//	@Summary		Decline invitation
//	@Description	Declines a pending invitation
//	@Tags			invitations
//	@Security		BearerAuth
//	@Param			token	path		string	true	"Invitation token"
//	@Success		200		{object}	SuccessResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/invitations/{token}/decline [post]
func (h *Handler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := apiCtx.GetUser(ctx)
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		_ = response.BadRequest(w, r, errors.New("invitation token is required"))
		return
	}

	// Get the invitation
	inv, err := h.store.Invitations.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("invitation not found"))
			return
		}
		_ = response.InternalServerError(w, r, errors.New("failed to get invitation"))
		return
	}

	// Verify the invitation is for this user's email
	if inv.Email != user.Email {
		_ = response.Forbidden(w, r, errors.New("this invitation was sent to a different email address"))
		return
	}

	// Delete the invitation (declining)
	if err := h.store.Invitations.DeleteByToken(ctx, token); err != nil {
		_ = response.InternalServerError(w, r, errors.New("failed to decline invitation"))
		return
	}

	_ = response.Ok(w, r, "Success", SuccessResponse{
		Message: "Invitation declined",
	})
}
