package secrets

import "testing"

func TestNormalizeGCPSecretRef(t *testing.T) {
	got := normalizeGCPSecretRef("projects/p1/secrets/s1/versions/3")
	want := "projects/p1/secrets/s1"
	if got != want {
		t.Fatalf("normalizeGCPSecretRef mismatch: got %q want %q", got, want)
	}
}

func TestNormalizeGCPSecretVersionRef(t *testing.T) {
	t.Run("already versioned", func(t *testing.T) {
		got := normalizeGCPSecretVersionRef("projects/p1/secrets/s1/versions/9")
		want := "projects/p1/secrets/s1/versions/9"
		if got != want {
			t.Fatalf("normalizeGCPSecretVersionRef mismatch: got %q want %q", got, want)
		}
	})

	t.Run("append latest", func(t *testing.T) {
		got := normalizeGCPSecretVersionRef("projects/p1/secrets/s1")
		want := "projects/p1/secrets/s1/versions/latest"
		if got != want {
			t.Fatalf("normalizeGCPSecretVersionRef mismatch: got %q want %q", got, want)
		}
	})
}
