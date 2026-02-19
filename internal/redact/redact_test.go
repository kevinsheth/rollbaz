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
