package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinsheth/rollbaz/internal/config"
	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
)

func TestRunShowHuman(t *testing.T) {
	stdout, err := runShowForFormat(t, "human")
	if err != nil {
		t.Fatalf("runShow() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Main Error: ABORTED") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRunActiveJSON(t *testing.T) {
	stdout := setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/reports/top_active_items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":[{"item":{"id":1755568172,"counter":269,"title":"RST_STREAM","status":"active","environment":"production","occurrences":7,"last_occurrence_timestamp":1700000000}}]}`)
	}))

	err := runActive(context.Background(), rootFlags{Format: "json", Limit: 10})
	if err != nil {
		t.Fatalf("runActive() error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"issues"`) {
		t.Fatalf("expected issues json output, got: %q", stdout.String())
	}
}

func TestRunRecentHuman(t *testing.T) {
	stdout := setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":1,"counter":2,"title":"Recent","status":"active","environment":"production","last_occurrence_timestamp":1700000000,"occurrences":5}]}}`)
	}))

	err := runRecent(context.Background(), rootFlags{Format: "human", Limit: 10})
	if err != nil {
		t.Fatalf("runRecent() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Recent") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestResolveAccessTokenFromConfig(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStoreAtPath(filepath.Join(dir, "config.json"))
	if err := store.AddProject("figure", "abc123"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}

	restoreStore := overrideConfigStore(func() (*config.Store, error) {
		return store, nil
	})
	defer restoreStore()

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "")

	token, err := resolveAccessToken(rootFlags{Project: "figure"})
	if err != nil {
		t.Fatalf("resolveAccessToken() error = %v", err)
	}
	if token != "abc123" {
		t.Fatalf("resolveAccessToken() = %q", token)
	}
}

func TestResolveAccessTokenFromEnvFallback(t *testing.T) {
	setNoConfigStore(t)

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "env-token")

	token, err := resolveAccessToken(rootFlags{})
	if err != nil {
		t.Fatalf("resolveAccessToken() error = %v", err)
	}
	if token != "env-token" {
		t.Fatalf("resolveAccessToken() = %q", token)
	}
}

func TestResolveAccessTokenMissing(t *testing.T) {
	setNoConfigStore(t)

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "")

	if _, err := resolveAccessToken(rootFlags{}); err == nil {
		t.Fatalf("expected token resolution error")
	}
}

func TestPrintOutputUnsupportedFormat(t *testing.T) {
	if err := printOutput("xml", "", nil); err == nil {
		t.Fatalf("expected unsupported format error")
	}
}

func TestProjectCommandsLifecycle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	store := config.NewStoreAtPath(configPath)

	restoreStore := overrideConfigStore(func() (*config.Store, error) {
		return store, nil
	})
	defer restoreStore()

	stdout := setupStdout(t)

	runRootCommand(t, "project", "add", "alpha", "--token", "t1")
	runRootCommand(t, "project", "add", "beta", "--token", "t2")

	stdout.Reset()
	runRootCommand(t, "project", "list")
	if !strings.Contains(stdout.String(), "alpha") {
		t.Fatalf("expected listed project names, got %q", stdout.String())
	}
}

func TestProjectCommandsUseNextRemove(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	store := config.NewStoreAtPath(configPath)
	if err := store.AddProject("alpha", "t1"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if err := store.AddProject("beta", "t2"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}

	restoreStore := overrideConfigStore(func() (*config.Store, error) {
		return store, nil
	})
	defer restoreStore()

	stdout := setupStdout(t)

	runRootCommand(t, "project", "use", "beta")

	stdout.Reset()
	runRootCommand(t, "project", "next")
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("expected active project output")
	}

	runRootCommand(t, "project", "remove", "beta")
}

func TestRootCommandDefaultRunsActive(t *testing.T) {
	stdout := setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/reports/top_active_items":
			_, _ = fmt.Fprint(w, `{"err":0,"result":[{"item":{"id":1,"counter":2,"title":"x","status":"active","environment":"production"}}]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root execute error = %v", err)
	}
	if !strings.Contains(stdout.String(), "COUNTER") || !strings.Contains(stdout.String(), "x") {
		t.Fatalf("expected active list output, got %q", stdout.String())
	}
}

func TestActiveAndRecentSubcommands(t *testing.T) {
	setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/reports/top_active_items":
			_, _ = fmt.Fprint(w, `{"err":0,"result":[{"item":{"id":1,"counter":2,"title":"x","status":"active","environment":"production"}}]}`)
		case "/api/1/items":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":1,"counter":3,"title":"y","status":"active","environment":"production"}]}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"active"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active command error = %v", err)
	}

	cmd = NewRootCmd()
	cmd.SetArgs([]string{"recent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("recent command error = %v", err)
	}
}

func setupServerAndStdout(t *testing.T, handler http.Handler) *bytes.Buffer {
	t.Helper()
	t.Setenv("ROLLBAR_ACCESS_TOKEN", "token")

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	restoreClient := overrideClientFactory(func(token string) (*rollbar.Client, error) {
		return rollbar.NewWithBaseURL(token, server.URL+"/api/1")
	})
	t.Cleanup(restoreClient)

	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	t.Cleanup(func() {
		stdoutWriter = os.Stdout
	})

	return stdout
}

func runRootCommand(t *testing.T, args ...string) {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v error = %v", args, err)
	}
}

func TestShowCommandInvalidCounter(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"show", "not-a-number"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestRunShowJSON(t *testing.T) {
	stdout, err := runShowForFormat(t, "json")
	if err != nil {
		t.Fatalf("runShow() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"main_error"`) {
		t.Fatalf("expected json detail output, got %q", stdout.String())
	}
}

func TestBuildServiceErrors(t *testing.T) {
	setNoConfigStore(t)

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "")
	if _, _, err := buildService(rootFlags{}); err == nil {
		t.Fatalf("expected buildService token error")
	}

	restoreClient := overrideClientFactory(func(token string) (*rollbar.Client, error) {
		return nil, fmt.Errorf("bad token %s", token)
	})
	defer restoreClient()

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "token")
	if _, _, err := buildService(rootFlags{}); err == nil {
		t.Fatalf("expected buildService client error")
	}
}

func TestExecuteFailure(t *testing.T) {
	setNoConfigStore(t)

	t.Setenv("ROLLBAR_ACCESS_TOKEN", "")
	originalArgs := os.Args
	os.Args = []string{"rollbaz"}
	defer func() { os.Args = originalArgs }()

	setupStderr(t)

	if code := Execute(); code != 1 {
		t.Fatalf("Execute() = %d, want 1", code)
	}
}

func TestSanitizeError(t *testing.T) {
	err := sanitizeError(fmt.Errorf("token leaked super-secret"), "super-secret")
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("token should be redacted")
	}
}

func TestShouldRenderProgress(t *testing.T) {
	originalStdout := stdoutWriter
	originalIsTerminal := isTerminal
	stdoutWriter = os.Stdout
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() {
		stdoutWriter = originalStdout
		isTerminal = originalIsTerminal
	})

	t.Setenv("CI", "")
	if !shouldRenderProgress("human") {
		t.Fatalf("expected progress enabled for human terminal output")
	}
	if shouldRenderProgress("json") {
		t.Fatalf("expected progress disabled for json")
	}
	t.Setenv("CI", "1")
	if shouldRenderProgress("human") {
		t.Fatalf("expected progress disabled in CI")
	}
}

func TestTerminalRenderWidth(t *testing.T) {
	originalStdout := stdoutWriter
	originalIsTerminal := isTerminal
	originalGetSize := getTerminalSize
	t.Cleanup(func() {
		stdoutWriter = originalStdout
		isTerminal = originalIsTerminal
		getTerminalSize = originalGetSize
	})

	stdoutWriter = io.Discard
	if got := terminalRenderWidth(); got != fallbackRenderWidth {
		t.Fatalf("terminalRenderWidth(non-file) = %d", got)
	}

	stdoutWriter = os.Stdout
	isTerminal = func(int) bool { return true }
	getTerminalSize = func(int) (int, int, error) { return 220, 50, nil }
	if got := terminalRenderWidth(); got != maxRenderWidth {
		t.Fatalf("terminalRenderWidth(max clamp) = %d", got)
	}

	getTerminalSize = func(int) (int, int, error) { return 100, 50, nil }
	if got := terminalRenderWidth(); got != 98 {
		t.Fatalf("terminalRenderWidth(normal) = %d", got)
	}
}

func TestRunWithProgress(t *testing.T) {
	originalStdout := stdoutWriter
	originalIsTerminal := isTerminal
	originalGetSize := getTerminalSize
	t.Cleanup(func() {
		stdoutWriter = originalStdout
		isTerminal = originalIsTerminal
		getTerminalSize = originalGetSize
	})

	stdoutWriter = io.Discard
	value, err := runWithProgress("json", "noop", func() (int, error) { return 42, nil })
	if err != nil || value != 42 {
		t.Fatalf("runWithProgress(json) = %d, err=%v", value, err)
	}

	tempFile, err := os.CreateTemp(t.TempDir(), "progress")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer func() {
		_ = tempFile.Close()
	}()
	stdoutWriter = tempFile
	isTerminal = func(int) bool { return true }
	getTerminalSize = func(int) (int, int, error) { return 120, 40, nil }

	value, err = runWithProgress("human", "test", func() (int, error) { return 7, nil })
	if err != nil || value != 7 {
		t.Fatalf("runWithProgress(human) = %d, err=%v", value, err)
	}
}

func overrideClientFactory(factory func(string) (*rollbar.Client, error)) func() {
	original := newRollbarClient
	newRollbarClient = factory

	return func() {
		newRollbarClient = original
	}
}

func overrideConfigStore(factory func() (*config.Store, error)) func() {
	original := newConfigStore
	newConfigStore = factory

	return func() {
		newConfigStore = original
	}
}

func runShowForFormat(t *testing.T, format string) (*bytes.Buffer, error) {
	t.Helper()
	stdout := setupServerAndStdout(t, newSuccessHandler(t))

	err := runShow(context.Background(), rootFlags{Format: format}, domain.ItemCounter(269))

	return stdout, err
}

func setupStdout(t *testing.T) *bytes.Buffer {
	t.Helper()
	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	t.Cleanup(func() {
		stdoutWriter = os.Stdout
	})

	return stdout
}

func setupStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	stderr := &bytes.Buffer{}
	stderrWriter = stderr
	t.Cleanup(func() {
		stderrWriter = os.Stderr
	})

	return stderr
}

func setNoConfigStore(t *testing.T) {
	t.Helper()
	restoreStore := overrideConfigStore(func() (*config.Store, error) {
		return nil, fmt.Errorf("no config")
	})
	t.Cleanup(restoreStore)
}

func newSuccessHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
}
