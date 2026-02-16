package models

import (
	"time"

	"github.com/google/uuid"
)

// AIAnalysisStatus represents the state of an AI analysis.
type AIAnalysisStatus string

const (
	AIAnalysisStatusPending       AIAnalysisStatus = "pending"
	AIAnalysisStatusInProgress    AIAnalysisStatus = "in_progress"
	AIAnalysisStatusSuccess       AIAnalysisStatus = "success"
	AIAnalysisStatusFailed        AIAnalysisStatus = "failed"
	AIAnalysisStatusQuotaExceeded AIAnalysisStatus = "quota_exceeded"
	AIAnalysisStatusSuperseded    AIAnalysisStatus = "superseded"
)

// AIAnalysis represents a row in the ai_analyses table.
type AIAnalysis struct {
	ID                 uuid.UUID        `json:"id" db:"id"`
	RunID              uuid.UUID        `json:"run_id" db:"run_id"`
	ProjectID          uuid.UUID        `json:"project_id" db:"project_id"`
	Status             AIAnalysisStatus `json:"status" db:"status"`
	Provider           *string          `json:"provider,omitempty" db:"provider"`
	ProviderMode       *string          `json:"provider_mode,omitempty" db:"provider_mode"`
	Model              *string          `json:"model,omitempty" db:"model"`
	PromptVersion      *string          `json:"prompt_version,omitempty" db:"prompt_version"`
	PromptHash         *string          `json:"prompt_hash,omitempty" db:"prompt_hash"`
	ResponseHash       *string          `json:"response_hash,omitempty" db:"response_hash"`
	Summary            *string          `json:"summary,omitempty" db:"summary"`
	RootCause          *string          `json:"root_cause,omitempty" db:"root_cause"`
	Confidence         *float64         `json:"confidence,omitempty" db:"confidence"`
	EvidenceJSON       []byte           `json:"evidence_json" db:"evidence_json"`
	RawResponseBlobKey *string          `json:"raw_response_blob_key,omitempty" db:"raw_response_blob_key"`
	ErrorMessage       *string          `json:"error_message,omitempty" db:"error_message"`
	DedupKey           *string          `json:"dedup_key,omitempty" db:"dedup_key"`
	CreatedAt          time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at" db:"updated_at"`
}

// AIPublicationStatus represents the state of an AI publication.
type AIPublicationStatus string

const (
	AIPublicationStatusPending AIPublicationStatus = "pending"
	AIPublicationStatusSent    AIPublicationStatus = "sent"
	AIPublicationStatusUpdated AIPublicationStatus = "updated"
	AIPublicationStatusFailed  AIPublicationStatus = "failed"
)

// AIPublicationDestination represents where an analysis is published.
type AIPublicationDestination string

const (
	AIPublicationDestGitHubPRComment AIPublicationDestination = "github_pr_comment"
	AIPublicationDestGitHubCheck     AIPublicationDestination = "github_check"
	AIPublicationDestGitHubPRReview  AIPublicationDestination = "github_pr_review"
)

// AIPublication represents a row in the ai_publications table.
type AIPublication struct {
	ID           uuid.UUID                `json:"id" db:"id"`
	AnalysisID   uuid.UUID                `json:"analysis_id" db:"analysis_id"`
	RunID        uuid.UUID                `json:"run_id" db:"run_id"`
	Destination  AIPublicationDestination `json:"destination" db:"destination"`
	ExternalID   *string                  `json:"external_id,omitempty" db:"external_id"`
	Status       AIPublicationStatus      `json:"status" db:"status"`
	ErrorMessage *string                  `json:"error_message,omitempty" db:"error_message"`
	CreatedAt    time.Time                `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at" db:"updated_at"`
}

// AISuggestionStatus represents the state of an AI suggestion.
type AISuggestionStatus string

const (
	AISuggestionStatusPending          AISuggestionStatus = "pending"
	AISuggestionStatusPosted           AISuggestionStatus = "posted"
	AISuggestionStatusAccepted         AISuggestionStatus = "accepted"
	AISuggestionStatusDismissed        AISuggestionStatus = "dismissed"
	AISuggestionStatusFailedValidation AISuggestionStatus = "failed_validation"
)

// AISuggestion represents a row in the ai_suggestions table.
type AISuggestion struct {
	ID              uuid.UUID          `json:"id" db:"id"`
	AnalysisID      uuid.UUID          `json:"analysis_id" db:"analysis_id"`
	RunID           uuid.UUID          `json:"run_id" db:"run_id"`
	FilePath        string             `json:"file_path" db:"file_path"`
	StartLine       int                `json:"start_line" db:"start_line"`
	EndLine         int                `json:"end_line" db:"end_line"`
	OriginalCode    string             `json:"original_code" db:"original_code"`
	SuggestedCode   string             `json:"suggested_code" db:"suggested_code"`
	Explanation     string             `json:"explanation" db:"explanation"`
	Confidence      float64            `json:"confidence" db:"confidence"`
	Status          AISuggestionStatus `json:"status" db:"status"`
	GitHubCommentID *string            `json:"github_comment_id,omitempty" db:"github_comment_id"`
	RiskScore       *float64           `json:"risk_score,omitempty" db:"risk_score"`
	FailureReason   *string            `json:"failure_reason,omitempty" db:"failure_reason"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at" db:"updated_at"`
}

// Usage event type constant for AI analysis.
const UsageEventAIAnalysis = "ai_analysis"
