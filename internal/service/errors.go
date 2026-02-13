package service

import (
	"errors"
	"fmt"
)

// QuotaExceededError is returned when a plan limit is exceeded.
type QuotaExceededError struct {
	Resource   string // "cache_storage", "bandwidth", "projects", "team_members", "concurrent_runs"
	Current    int64
	Limit      int64
	PlanSlug   string
	UpgradeURL string
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s quota exceeded: %d / %d (plan: %s). Upgrade at %s",
		e.Resource, e.Current, e.Limit, e.PlanSlug, e.UpgradeURL)
}

// IsQuotaExceeded returns true if the error is a QuotaExceededError.
func IsQuotaExceeded(err error) bool {
	var qe *QuotaExceededError
	return errors.As(err, &qe)
}
