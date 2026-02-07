package remote

import "fmt"

// ActionKey returns the storage key for an action cache entry: ac/{taskName}/{cacheKey}.
func ActionKey(taskName, cacheKey string) string {
	return fmt.Sprintf("ac/%s/%s", taskName, cacheKey)
}
