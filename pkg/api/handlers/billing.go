package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	apiCtx "github.com/mujhtech/dagryn/pkg/api/context"
	"github.com/mujhtech/dagryn/pkg/database/repo"
	"github.com/mujhtech/dagryn/pkg/http/response"
)

// ListBillingPlans returns all active billing plans.
// GET /api/v1/billing/plans
func (h *Handler) ListBillingPlans(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	plans, err := h.billingService.ListPlans(r.Context())
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "billing plans", plans)
}

// GetBillingPlan returns a specific billing plan by slug.
// GET /api/v1/billing/plans/{slug}
func (h *Handler) GetBillingPlan(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		_ = response.BadRequest(w, r, errors.New("plan slug is required"))
		return
	}

	plan, err := h.billingService.GetPlan(r.Context(), slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("plan not found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "billing plan", plan)
}

// GetBillingOverview returns the billing overview for the current user.
// GET /api/v1/billing/overview
func (h *Handler) GetBillingOverview(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	account, err := h.billingService.GetOrCreateAccount(r.Context(), user.ID, user.Email, derefStr(user.Name))
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	overview, err := h.billingService.GetOverview(r.Context(), account.ID)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "billing overview", overview)
}

// CreateCheckoutSession creates a Stripe Checkout session for subscribing to a plan.
// POST /api/v1/billing/checkout
func (h *Handler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req struct {
		PlanSlug   string `json:"plan_slug"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.PlanSlug == "" || req.SuccessURL == "" || req.CancelURL == "" {
		_ = response.BadRequest(w, r, errors.New("plan_slug, success_url, and cancel_url are required"))
		return
	}

	account, err := h.billingService.GetOrCreateAccount(r.Context(), user.ID, user.Email, derefStr(user.Name))
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	url, err := h.billingService.CreateCheckoutSession(r.Context(), account.ID, req.PlanSlug, req.SuccessURL, req.CancelURL)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "checkout session created", map[string]string{"url": url})
}

// CreatePortalSession creates a Stripe Billing Portal session.
// POST /api/v1/billing/portal
func (h *Handler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req struct {
		ReturnURL string `json:"return_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}
	if req.ReturnURL == "" {
		_ = response.BadRequest(w, r, errors.New("return_url is required"))
		return
	}

	account, err := h.billingService.GetOrCreateAccount(r.Context(), user.ID, user.Email, derefStr(user.Name))
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	url, err := h.billingService.CreatePortalSession(r.Context(), account.ID, req.ReturnURL)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "portal session created", map[string]string{"url": url})
}

// CancelSubscription cancels the current user's subscription.
// POST /api/v1/billing/cancel
func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	var req struct {
		AtPeriodEnd bool `json:"at_period_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = response.BadRequest(w, r, errors.New("invalid request body"))
		return
	}

	account, err := h.billingService.GetOrCreateAccount(r.Context(), user.ID, user.Email, derefStr(user.Name))
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	if err := h.billingService.CancelSubscription(r.Context(), account.ID, req.AtPeriodEnd); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			_ = response.NotFound(w, r, errors.New("no active subscription found"))
			return
		}
		_ = response.InternalServerError(w, r, err)
		return
	}

	_ = response.Ok(w, r, "subscription canceled", nil)
}

// ListInvoices returns the current user's invoices.
// GET /api/v1/billing/invoices?limit=20&offset=0
func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	if h.billingService == nil {
		_ = response.ServiceUnavailable(w, r, errors.New("billing service not configured"))
		return
	}

	user := apiCtx.GetUser(r.Context())
	if user == nil {
		_ = response.Unauthorized(w, r, errors.New("authentication required"))
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	account, err := h.billingService.GetOrCreateAccount(r.Context(), user.ID, user.Email, derefStr(user.Name))
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}

	invoices, err := h.billingService.ListInvoices(r.Context(), account.ID, limit, offset)
	if err != nil {
		_ = response.InternalServerError(w, r, err)
		return
	}
	_ = response.Ok(w, r, "invoices", invoices)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
