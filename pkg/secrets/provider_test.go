package secrets

import "testing"

func TestCloudflareAPIBaseURL(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_BASE_URL", "")
	if got := cloudflareAPIBaseURL(""); got != "https://api.cloudflare.com/client/v4" {
		t.Fatalf("default base url mismatch: %s", got)
	}

	t.Setenv("CLOUDFLARE_API_BASE_URL", "https://example.test/base/")
	if got := cloudflareAPIBaseURL(""); got != "https://example.test/base" {
		t.Fatalf("trimmed base url mismatch: %s", got)
	}

	if got := cloudflareAPIBaseURL("https://cfg.example/v4/"); got != "https://cfg.example/v4" {
		t.Fatalf("configured base url mismatch: %s", got)
	}
}
