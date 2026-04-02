package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/scim"
)

// SCIMHandler handles SCIM 2.0 provisioning endpoints.
type SCIMHandler struct {
	scimService *scim.Service
}

// NewSCIMHandler creates a new SCIM handler.
func NewSCIMHandler(scimService *scim.Service) *SCIMHandler {
	return &SCIMHandler{scimService: scimService}
}

func (h *SCIMHandler) writeSCIMJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (h *SCIMHandler) writeSCIMError(w http.ResponseWriter, status int, detail string) {
	h.writeSCIMJSON(w, status, scim.SCIMError{
		Schemas: []string{scim.SchemaError},
		Detail:  detail,
		Status:  status,
	})
}

// --- Users ---

// ListSCIMUsers lists users for SCIM provisioning.
func (h *SCIMHandler) ListSCIMUsers(w http.ResponseWriter, r *http.Request) {
	team := apiCtx.GetTeam(r.Context())
	if team == nil {
		h.writeSCIMError(w, http.StatusBadRequest, "missing team context")
		return
	}

	startIndex, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))

	filter, err := scim.ParseFilter(r.URL.Query().Get("filter"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.scimService.ListUsers(r.Context(), team.ID, filter, startIndex, count)
	if err != nil {
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// CreateSCIMUser creates a user via SCIM.
func (h *SCIMHandler) CreateSCIMUser(w http.ResponseWriter, r *http.Request) {
	team := apiCtx.GetTeam(r.Context())
	if team == nil {
		h.writeSCIMError(w, http.StatusBadRequest, "missing team context")
		return
	}

	var scimUser scim.SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&scimUser); err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.scimService.CreateUser(r.Context(), team.ID, scimUser)
	if err != nil {
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusCreated, result)
}

// GetSCIMUser returns a user by ID via SCIM.
func (h *SCIMHandler) GetSCIMUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	result, err := h.scimService.GetUser(r.Context(), userID)
	if err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, "user not found")
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// UpdateSCIMUser replaces a user via SCIM (PUT).
func (h *SCIMHandler) UpdateSCIMUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var scimUser scim.SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&scimUser); err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.scimService.UpdateUser(r.Context(), userID, scimUser)
	if err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, "user not found")
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// PatchSCIMUser applies a SCIM PATCH operation to a user.
func (h *SCIMHandler) PatchSCIMUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var patch scim.PatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.scimService.PatchUser(r.Context(), userID, patch)
	if err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, "user not found")
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// DeleteSCIMUser soft-deletes (deactivates) a user via SCIM.
func (h *SCIMHandler) DeleteSCIMUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := h.scimService.DeleteUser(r.Context(), userID); err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, "user not found")
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Groups ---

// ListSCIMGroups lists groups (teams) for SCIM.
func (h *SCIMHandler) ListSCIMGroups(w http.ResponseWriter, r *http.Request) {
	team := apiCtx.GetTeam(r.Context())
	if team == nil {
		h.writeSCIMError(w, http.StatusBadRequest, "missing team context")
		return
	}

	result, err := h.scimService.ListGroups(r.Context(), team.ID)
	if err != nil {
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// GetSCIMGroup returns a group (team) by ID.
func (h *SCIMHandler) GetSCIMGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	result, err := h.scimService.GetGroup(r.Context(), groupID)
	if err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, fmt.Sprintf("group %s not found", groupID))
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// PatchSCIMGroup applies SCIM PATCH operations to a group (team).
func (h *SCIMHandler) PatchSCIMGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid group ID")
		return
	}

	var patch scim.PatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		h.writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.scimService.PatchGroup(r.Context(), groupID, patch)
	if err != nil {
		if err == repo.ErrNotFound {
			h.writeSCIMError(w, http.StatusNotFound, fmt.Sprintf("group %s not found", groupID))
			return
		}
		h.writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeSCIMJSON(w, http.StatusOK, result)
}

// DeleteSCIMGroup is a no-op (teams are not deleted via SCIM).
func (h *SCIMHandler) DeleteSCIMGroup(w http.ResponseWriter, r *http.Request) {
	h.writeSCIMError(w, http.StatusNotImplemented, "group deletion is not supported via SCIM")
}
