package redact

import "testing"

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		token string
		want  string
	}{
		{
			name:  "redacts access_token query",
			input: "https://api.rollbar.com/api/1/item?access_token=abc123",
			want:  "https://api.rollbar.com/api/1/item?access_token=[REDACTED]",
		},
		{
			name:  "redacts literal token",
			input: "request failed for token super-secret",
			token: "super-secret",
			want:  "request failed for token [REDACTED]",
		},
		{
			name:  "handles empty value",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := String(tc.input, tc.token)
			if got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValue(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"access_token": "abc",
		"nested": map[string]any{
			"authorization": "Bearer x",
			"url":           "https://x?access_token=abc",
		},
	}

	got := Value(input, "abc").(map[string]any)
	if got["access_token"] != "[REDACTED]" {
		t.Fatalf("expected access_token redacted, got %v", got["access_token"])
	}
	nested := got["nested"].(map[string]any)
	if nested["authorization"] != "[REDACTED]" {
		t.Fatalf("expected authorization redacted, got %v", nested["authorization"])
	}
	if nested["url"] != "https://x?access_token=[REDACTED]" {
		t.Fatalf("expected URL token redacted, got %v", nested["url"])
	}
}

func TestValueArrayAndDefaultTypes(t *testing.T) {
	t.Parallel()

	input := []any{
		"token abc",
		123,
		map[string]any{"api_key": "xyz"},
	}

	got := Value(input, "abc").([]any)
	if got[0] != "token [REDACTED]" {
		t.Fatalf("expected first value redacted, got %v", got[0])
	}
	if got[1] != 123 {
		t.Fatalf("expected numeric value unchanged, got %v", got[1])
	}
	nested := got[2].(map[string]any)
	if nested["api_key"] != "[REDACTED]" {
		t.Fatalf("expected api_key redacted, got %v", nested["api_key"])
	}
}

func TestValueStruct(t *testing.T) {
	t.Parallel()

	type payload struct {
		AccessToken string `json:"access_token"`
	}

	got := Value(payload{AccessToken: "abc"}, "abc").(map[string]any)
	if got["access_token"] != "[REDACTED]" {
		t.Fatalf("expected struct token redacted, got %v", got["access_token"])
	}
}
