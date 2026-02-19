package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kevinsheth/rollbaz/internal/rollbar"
)

func TestRunSuccess(t *testing.T) {
	server := newSuccessServer(t)
	defer server.Close()

	restore := overrideClientFactory(func(token string) (*rollbar.Client, error) {
		return rollbar.NewWithBaseURL(token, server.URL+"/api/1")
	})
	defer restore()

	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	defer func() {
		stdoutWriter = os.Stdout
	}()

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "token")

	err := run(context.Background(), rootFlags{item: 269})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	want := expectedSuccessOutput()

	if got != want {
		t.Fatalf("run() output = %q, want %q", got, want)
	}
}

func newSuccessServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/item_by_counter/269":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"itemId":1755568172}}`)
		case "/api/1/item/1755568172/":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"id":1755568172,"project_id":766510,"counter":269,"title":"RST_STREAM","status":"active","environment":"production","total_occurrences":7}}`)
		case "/api/1/item/1755568172/instances":
			if r.URL.RawQuery != "per_page=1" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprint(w, `{"err":0,"result":[{"id":1,"data":{"trace":{"exception":{"description":"ABORTED"}}}}]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func expectedSuccessOutput() string {
	return strings.Join([]string{
		"main error: ABORTED",
		"title: RST_STREAM",
		"status: active | environment: production | occurrences: 7",
		"counter: 269 | item_id: 1755568172",
	}, "\n")
}

func TestRunMissingToken(t *testing.T) {
	t.Setenv("ROLLBAR_ACCESS_TOKEN", "")

	err := run(context.Background(), rootFlags{item: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunClientInitErrorIsRedacted(t *testing.T) {
	restore := overrideClientFactory(func(token string) (*rollbar.Client, error) {
		return nil, errors.New("failed for token " + token)
	})
	defer restore()

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "super-secret")

	err := run(context.Background(), rootFlags{item: 1})
	if err == nil {
		t.Fatalf("expected error")
	}

	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error should redact token")
	}
}

func TestLoadItemAndInstanceItemError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/item/1/":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprint(w, `oops token`)
		case "/api/1/item/1/instances":
			_, _ = fmt.Fprint(w, `{"err":0,"result":[]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := rollbar.NewWithBaseURL("token", server.URL+"/api/1")
	if err != nil {
		t.Fatalf("NewWithBaseURL() error = %v", err)
	}

	_, _, err = loadItemAndInstance(context.Background(), client, 1, "token")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestReadAccessToken(t *testing.T) {
	t.Setenv("ROLLBAR_ACCESS_TOKEN", "abc")

	token, err := readAccessToken()
	if err != nil {
		t.Fatalf("readAccessToken() error = %v", err)
	}

	if token != "abc" {
		t.Fatalf("readAccessToken() = %q", token)
	}
}

func TestRunResolveError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":1,"message":"denied"}`)
	}))
	defer server.Close()

	restore := overrideClientFactory(func(token string) (*rollbar.Client, error) {
		return rollbar.NewWithBaseURL(token, server.URL+"/api/1")
	})
	defer restore()

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "token")

	err := run(context.Background(), rootFlags{item: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadItemAndInstanceInstanceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/item/1/":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"id":1,"project_id":1,"counter":1,"title":"x","status":"active","environment":"prod"}}`)
		case "/api/1/item/1/instances":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `boom`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := rollbar.NewWithBaseURL("token", server.URL+"/api/1")
	if err != nil {
		t.Fatalf("NewWithBaseURL() error = %v", err)
	}

	_, _, err = loadItemAndInstance(context.Background(), client, 1, "token")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteSummaryWithNilInstance(t *testing.T) {
	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	defer func() {
		stdoutWriter = os.Stdout
	}()

	writeSummary(1, 2, rollbar.Item{Title: "x", Status: "active"}, nil)

	if !strings.Contains(stdout.String(), "main error: unknown") {
		t.Fatalf("expected unknown main error")
	}
}

func TestSanitizeError(t *testing.T) {
	err := sanitizeError(errors.New("bad token super-secret"), "super-secret")
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("token should be redacted")
	}
}

func overrideClientFactory(factory func(string) (*rollbar.Client, error)) func() {
	original := newRollbarClient
	newRollbarClient = factory

	return func() {
		newRollbarClient = original
	}
}
