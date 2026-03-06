package handlers

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/encrypt"
)

// WebhookEventDataPayload is the decrypted payload for a webhook job.
type WebhookEventDataPayload struct {
	ProviderID       string `json:"provider_id"`
	Header           string `json:"header"`
	Body             string `json:"body"`
	WebhookSignature string `json:"webhook_signature"`
}

// GitHubPushPayload represents a GitHub push event (subset of fields we need).
type GitHubPushPayload struct {
	Ref        string `json:"ref"`   // e.g. "refs/heads/main"
	After      string `json:"after"` // commit SHA
	Repository struct {
		FullName string `json:"full_name"` // e.g. "owner/repo"
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

// WebhookHandler holds dependencies for processing webhook jobs.
type WebhookHandler struct {
	encrypter encrypt.Encrypt
}

// NewWebhookHandler creates a webhook handler that decrypts and parses payloads.
func NewWebhookHandler(encrypter encrypt.Encrypt) *WebhookHandler {
	return &WebhookHandler{encrypter: encrypter}
}

// Handle processes the webhook task: decrypt payload (if encrypter set), parse, and dispatch by provider.
func (h *WebhookHandler) Handle(ctx context.Context, t *asynq.Task) error {
	rawPayload := string(t.Payload())
	var plaintext string
	if h.encrypter != nil {
		var err error
		plaintext, err = h.encrypter.Decrypt(rawPayload)
		if err != nil {
			slog.Error("webhook: decrypt failed", "error", err)
			return err
		}
	} else {
		plaintext = rawPayload
	}

	var payload WebhookEventDataPayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		slog.Error("webhook: parse payload failed", "error", err)
		return err
	}

	switch payload.ProviderID {
	case "github":
		return h.dispatchGitHub(ctx, &payload)
	case "gitlab":
		return h.dispatchGitLab(ctx, &payload)
	case "bitbucket":
		return h.dispatchBitbucket(ctx, &payload)
	default:
		slog.Info("webhook: unknown provider", "provider_id", payload.ProviderID)
		return nil
	}
}

func (h *WebhookHandler) dispatchGitHub(ctx context.Context, payload *WebhookEventDataPayload) error {
	var event GitHubPushPayload
	if err := json.Unmarshal([]byte(payload.Body), &event); err != nil {
		slog.Error("webhook: github parse body failed", "error", err)
		return err
	}
	// Log for now; trigger run will be wired when ExecuteRun job and project lookup exist
	slog.Info("webhook: github event",
		"repo", event.Repository.FullName,
		"ref", event.Ref,
		"commit", event.After,
	)
	return nil
}

func (h *WebhookHandler) dispatchGitLab(ctx context.Context, payload *WebhookEventDataPayload) error {
	// Placeholder: parse GitLab event body when needed
	slog.Info("webhook: gitlab event received", "body_len", len(payload.Body))
	return nil
}

func (h *WebhookHandler) dispatchBitbucket(ctx context.Context, payload *WebhookEventDataPayload) error {
	// Placeholder: parse Bitbucket event body when needed
	slog.Info("webhook: bitbucket event received", "body_len", len(payload.Body))
	return nil
}

// HandleWebhook returns a handler function that uses the given encrypter.
// For job registration with encrypter injection, use NewWebhookHandler(enc).Handle instead.
func HandleWebhook(encrypter encrypt.Encrypt) func(context.Context, *asynq.Task) error {
	handler := NewWebhookHandler(encrypter)
	return handler.Handle
}
