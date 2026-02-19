package summary

import (
	"encoding/json"
	"testing"
)

var mainErrorTests = []struct {
	name string
	body string
	data string
	want string
}{
	{
		name: "prefers data trace exception description",
		data: `{"trace":{"exception":{"description":"bad db timeout"}}}`,
		want: "bad db timeout",
	},
	{
		name: "falls back to body message",
		body: `{"message":{"body":"body-level error"}}`,
		want: "body-level error",
	},
	{
		name: "handles trace_chain path",
		data: `{"trace_chain":[{"exception":{"message":"chain message"}}]}`,
		want: "chain message",
	},
	{
		name: "returns unknown for invalid json",
		body: `{"broken":`,
		want: "unknown",
	},
	{
		name: "returns unknown when absent",
		data: `{"foo":"bar"}`,
		want: "unknown",
	},
	{
		name: "supports nested body trace_chain",
		data: `{"body":{"trace_chain":[{"exception":{"message":"nested chain message"}}]}}`,
		want: "nested chain message",
	},
}

func TestMainError(t *testing.T) {
	t.Parallel()

	for _, tc := range mainErrorTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MainError(json.RawMessage(tc.body), json.RawMessage(tc.data))
			if got != tc.want {
				t.Fatalf("MainError() = %q, want %q", got, tc.want)
			}
		})
	}
}
