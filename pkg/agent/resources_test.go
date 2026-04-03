package agent

import "testing"

func TestEstimateMillicoresAvailable(t *testing.T) {
	tests := []struct {
		name     string
		cores    int
		usage    float64
		expected int64
	}{
		{name: "full available", cores: 4, usage: 0, expected: 4000},
		{name: "half used", cores: 4, usage: 50, expected: 2000},
		{name: "fully used", cores: 4, usage: 100, expected: 0},
		{name: "invalid cores", cores: 0, usage: 10, expected: 0},
		{name: "negative usage clamps", cores: 2, usage: -10, expected: 2000},
		{name: "overflow usage clamps", cores: 2, usage: 200, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateMillicoresAvailable(tt.cores, tt.usage)
			if got != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}
