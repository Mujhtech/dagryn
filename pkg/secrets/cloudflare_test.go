package secrets

import "testing"

func TestParseCloudflareRef(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		wantStore  string
		wantSecret string
		wantErr    bool
	}{
		{name: "colon format", ref: "store123:MY_SECRET", wantStore: "store123", wantSecret: "MY_SECRET"},
		{name: "slash format", ref: "store123/MY_SECRET", wantStore: "store123", wantSecret: "MY_SECRET"},
		{name: "trim whitespace", ref: "  store123:MY_SECRET  ", wantStore: "store123", wantSecret: "MY_SECRET"},
		{name: "missing name", ref: "store123:", wantErr: true},
		{name: "missing store", ref: ":MY_SECRET", wantErr: true},
		{name: "empty", ref: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, secret, err := parseCloudflareRef(tc.ref)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if store != tc.wantStore {
				t.Fatalf("store mismatch: got %q want %q", store, tc.wantStore)
			}
			if secret != tc.wantSecret {
				t.Fatalf("secret mismatch: got %q want %q", secret, tc.wantSecret)
			}
		})
	}
}
