package rollbar

import (
	"encoding/json"
	"testing"
)

func TestFlexibleUnmarshalJSON(t *testing.T) {
	t.Parallel()

	levelCases := map[string]struct {
		input string
		want  flexibleLevel
	}{
		"numeric level":         {input: `40`, want: "error"},
		"string level":          {input: `"warning"`, want: "warning"},
		"unknown numeric level": {input: `99`, want: "99"},
	}
	for name, tc := range levelCases {
		var level flexibleLevel
		if err := json.Unmarshal([]byte(tc.input), &level); err != nil {
			t.Fatalf("%s: unmarshal error = %v", name, err)
		}
		if level != tc.want {
			t.Fatalf("%s: level = %q, want %q", name, level, tc.want)
		}
	}

	uintCases := map[string]struct {
		input string
		want  flexibleUint64
	}{
		"string uint":  {input: `"123"`, want: 123},
		"numeric uint": {input: `456`, want: 456},
		"null uint":    {input: `null`, want: 0},
	}
	for name, tc := range uintCases {
		var value flexibleUint64
		if err := json.Unmarshal([]byte(tc.input), &value); err != nil {
			t.Fatalf("%s: unmarshal error = %v", name, err)
		}
		if value != tc.want {
			t.Fatalf("%s: value = %d, want %d", name, value, tc.want)
		}
	}
}

func TestFlexibleUint64UnmarshalJSONInvalid(t *testing.T) {
	t.Parallel()

	var value flexibleUint64
	if err := json.Unmarshal([]byte(`"not-number"`), &value); err == nil {
		t.Fatalf("expected parse error")
	}
}
