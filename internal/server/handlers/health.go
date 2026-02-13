package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/mujhtech/dagryn/internal/server/dto"
	"github.com/mujhtech/dagryn/internal/server/response"
)

// Version is set at build time.
var Version = "dev"

// Health godoc
// @Summary Health check
// @Description Returns the health status of the server
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	mode := "self_hosted"
	edition := "community"
	licensed := false

	if h.IsCloudMode() {
		mode = "cloud"
		edition = "cloud"
		licensed = true
	} else if h.featureGate != nil {
		edition = string(h.featureGate.Edition())
		licensed = h.featureGate.Claims() != nil
	}

	_ = response.Ok(w, r, "Server is healthy", HealthResponse{
		Status:    "healthy",
		Version:   Version,
		Mode:      mode,
		Edition:   edition,
		Licensed:  licensed,
		Timestamp: time.Now().UTC(),
	})
}

// Ready godoc
// @Summary Readiness check
// @Description Returns the readiness status of the server (config-driven: database and optional Redis)
// @Tags health
// @Produce json
// @Success 200 {object} ReadyResponse
// @Failure 503 {object} ReadyResponse
// @Router /ready [get]
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := make(map[string]string)
	allOk := true

	if h.readyCheckDatabase {
		dbStatus := "connected"
		if err := h.db.Ping(ctx); err != nil {
			dbStatus = "disconnected"
			allOk = false
		}
		checks["database"] = dbStatus
	}

	if h.readyCheckRedis && h.redisForReady != nil {
		redisStatus := "connected"
		if err := h.redisForReady.Ready(ctx); err != nil {
			redisStatus = "disconnected"
			allOk = false
		}
		checks["redis"] = redisStatus
	}

	if !allOk {
		_ = response.ServiceUnavailable(w, r, errors.New("one or more readiness checks failed"))
		return
	}

	resp := ReadyResponse{Status: "ready", Checks: checks}
	if v, ok := checks["database"]; ok {
		resp.Database = v
	}
	_ = response.Ok(w, r, "Server is ready", resp)
}

// ListAuthProviders godoc
// @Summary List authentication providers
// @Description Returns the list of available OAuth authentication providers
// @Tags auth
// @Produce json
// @Success 200 {object} AuthProvidersResponse
// @Router /api/v1/auth/providers [get]
func (h *Handler) ListAuthProviders(w http.ResponseWriter, r *http.Request) {
	// TODO: Get these from config in Phase 1b
	providers := []dto.AuthProvider{
		{
			ID:      "github",
			Name:    "GitHub",
			Enabled: true, // Will be determined by config
		},
		{
			ID:      "google",
			Name:    "Google",
			Enabled: true, // Will be determined by config
		},
	}

	_ = response.Ok(w, r, "Authentication providers retrieved", providers)
}
