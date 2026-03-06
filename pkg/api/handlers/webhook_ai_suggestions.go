package handlers

import (
	"context"
	"log/slog"
	"strings"

	"github.com/mujhtech/dagryn/pkg/database/models"
)

// detectSuggestionAcceptance checks push commits for patterns that indicate
// a developer accepted a GitHub suggestion (e.g., "Apply suggestion from code review").
// This is best-effort and non-blocking — errors are logged and ignored.
func (h *Handler) detectSuggestionAcceptance(ctx context.Context, project *models.Project, branch string, commits []GitHubPushCommit) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("suggestion acceptance detection panic", "recover", r)
		}
	}()

	// Fetch posted suggestions for this project+branch.
	suggestions, err := h.store.AI.ListPostedSuggestionsByProjectAndBranch(ctx, project.ID, branch)
	if err != nil || len(suggestions) == 0 {
		return
	}

	// Build a map of file paths from posted suggestions for quick lookup.
	suggestionsByFile := make(map[string][]models.AISuggestion)
	for _, s := range suggestions {
		suggestionsByFile[s.FilePath] = append(suggestionsByFile[s.FilePath], s)
	}

	for _, commit := range commits {
		if !isSuggestionAcceptanceCommit(commit.Message) {
			continue
		}

		// Cross-reference modified files against posted suggestions.
		modifiedFiles := append(commit.Modified, commit.Added...)
		for _, file := range modifiedFiles {
			matched, ok := suggestionsByFile[file]
			if !ok {
				continue
			}
			for _, s := range matched {
				if err := h.store.AI.UpdateSuggestionStatus(ctx, s.ID, models.AISuggestionStatusAccepted, nil, nil); err != nil {
					slog.Warn("failed to mark suggestion as accepted",
						"suggestion_id", s.ID.String(),
						"error", err)
				} else {
					slog.Info("suggestion accepted",
						"suggestion_id", s.ID.String(),
						"file", s.FilePath,
						"commit", commit.ID)
				}
			}
			// Remove matched suggestions so we don't double-mark.
			delete(suggestionsByFile, file)
		}
	}
}

// isSuggestionAcceptanceCommit checks if a commit message matches patterns
// that GitHub generates when a user applies a suggestion from a code review.
func isSuggestionAcceptanceCommit(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "apply suggestion from code review") ||
		strings.Contains(lower, "apply suggestions from code review") ||
		(strings.Contains(lower, "co-authored-by") && strings.Contains(lower, "noreply@github.com"))
}
