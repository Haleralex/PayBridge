package telemetry

import (
	"testing"
)

func TestParseOTLPHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "single header",
			input:    "Authorization=Basic dXNlcjpwYXNz",
			expected: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
		},
		{
			name:  "multiple headers",
			input: "Authorization=Basic abc,X-Scope-OrgID=1",
			expected: map[string]string{
				"Authorization":  "Basic abc",
				"X-Scope-OrgID": "1",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "whitespace trimmed",
			input:    " Authorization=Basic abc , X-Scope-OrgID=1 ",
			expected: map[string]string{"Authorization": "Basic abc", "X-Scope-OrgID": "1"},
		},
		{
			name:  "base64 value with padding equals signs",
			input: "Authorization=Basic dXNlcjpwYXNz==",
			expected: map[string]string{"Authorization": "Basic dXNlcjpwYXNz=="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOTLPHeaders(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d headers, want %d: %v", len(got), len(tt.expected), got)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("header %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare host:port gets http prefix",
			input:    "jaeger:4318",
			expected: "http://jaeger:4318",
		},
		{
			name:     "http scheme unchanged",
			input:    "http://jaeger:4318",
			expected: "http://jaeger:4318",
		},
		{
			name:     "https scheme unchanged (Grafana Cloud)",
			input:    "https://tempo-prod-06-prod-us-east-0.grafana.net:443",
			expected: "https://tempo-prod-06-prod-us-east-0.grafana.net:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeEndpoint(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeEndpoint(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
