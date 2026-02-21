package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	stdout, err := runIssueListCommand(
		t,
		"/api/1/reports/top_active_items",
		`{"err":0,"result":[{"item":{"id":1755568172,"counter":269,"title":"RST_STREAM","status":"active","environment":"production","occurrences":7,"last_occurrence_timestamp":1700000000}}]}`,
		func() error { return runActive(context.Background(), rootFlags{Format: "json", Limit: 10}) },
	)
	if err != nil {
		t.Fatalf("runActive() error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"issues"`) {
		t.Fatalf("expected issues json output, got: %q", stdout.String())
	}
}

func TestRunRecentHuman(t *testing.T) {
	stdout, err := runIssueListCommand(
		t,
		"/api/1/items",
		`{"err":0,"result":{"items":[{"id":1,"counter":2,"title":"Recent","status":"active","environment":"production","last_occurrence_timestamp":1700000000,"occurrences":5}]}}`,
		func() error { return runRecent(context.Background(), rootFlags{Format: "human", Limit: 10}) },
	)
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

func TestProjectRemoveAll(t *testing.T) {
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

	runRootCommand(t, "project", "remove", "--all")

	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if file.ActiveProject != "" {
		t.Fatalf("ActiveProject = %q, want empty", file.ActiveProject)
	}
	if len(file.Projects) != 0 {
		t.Fatalf("Projects length = %d, want 0", len(file.Projects))
	}
}

func TestProjectRemoveValidation(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	store := config.NewStoreAtPath(configPath)

	restoreStore := overrideConfigStore(func() (*config.Store, error) {
		return store, nil
	})
	defer restoreStore()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"project", "remove"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "specify a project name or use --all") {
		t.Fatalf("expected missing-args remove error, got %v", err)
	}

	cmd = NewRootCmd()
	cmd.SetArgs([]string{"project", "remove", "alpha", "--all"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot use --all with a project name") {
		t.Fatalf("expected mixed remove args/flag error, got %v", err)
	}
}

func TestRootCommandDefaultRunsRecent(t *testing.T) {
	stdout := setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/items":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":1,"counter":3,"title":"y","status":"active","environment":"production"}]}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root execute error = %v", err)
	}
	if !strings.Contains(stdout.String(), "COUNTER") || !strings.Contains(stdout.String(), "y") {
		t.Fatalf("expected recent list output, got %q", stdout.String())
	}
}

func TestVersionFlag(t *testing.T) {
	originalVersion := version
	version = "v9.9.9"
	t.Cleanup(func() {
		version = originalVersion
	})

	stdout := setupStdout(t)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--version"})
	cmd.SetOut(stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version flag execute error = %v", err)
	}
	if !strings.Contains(stdout.String(), "v9.9.9") {
		t.Fatalf("expected version output, got %q", stdout.String())
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

func TestResolveCommandRequiresConfirmation(t *testing.T) {
	setupServerAndStdout(t, newActionSuccessHandler(t, nil))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resolve", "269"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestResolveCommandWithYesShortFlag(t *testing.T) {
	var patchPayload rollbar.ItemPatch
	setupServerAndStdout(t, newActionSuccessHandler(t, &patchPayload))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resolve", "269", "-y", "--resolved-in-version", "v1.2.3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command error = %v", err)
	}

	if patchPayload.Status != "resolved" || patchPayload.ResolvedInVersion != "v1.2.3" {
		t.Fatalf("unexpected patch payload: %+v", patchPayload)
	}
}

func TestResolveCommandWithYes(t *testing.T) {
	var patchPayload rollbar.ItemPatch
	setupServerAndStdout(t, newActionSuccessHandler(t, &patchPayload))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resolve", "269", "--yes", "--resolved-in-version", "v1.2.3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command error = %v", err)
	}

	if patchPayload.Status != "resolved" || patchPayload.ResolvedInVersion != "v1.2.3" {
		t.Fatalf("unexpected patch payload: %+v", patchPayload)
	}
}

func TestMuteCommandInvalidDuration(t *testing.T) {
	if err := runMute(context.Background(), rootFlags{}, 269, "500ms"); err == nil {
		t.Fatalf("expected invalid duration error")
	}
}

func TestMuteCommandWithYesAndDuration(t *testing.T) {
	var patchPayload rollbar.ItemPatch
	setupServerAndStdout(t, newActionSuccessHandler(t, &patchPayload))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mute", "269", "--for", "2h", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mute command error = %v", err)
	}
	if patchPayload.Status != "muted" {
		t.Fatalf("expected muted status patch, got %+v", patchPayload)
	}
	if patchPayload.SnoozeExpirationInSeconds == nil || *patchPayload.SnoozeExpirationInSeconds != int64(7200) {
		t.Fatalf("unexpected mute expiration in patch: %+v", patchPayload)
	}
}

func TestMuteCommandWithYesShortFlagAndDuration(t *testing.T) {
	var patchPayload rollbar.ItemPatch
	setupServerAndStdout(t, newActionSuccessHandler(t, &patchPayload))

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mute", "269", "--for", "2h", "-y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mute command error = %v", err)
	}
	if patchPayload.Status != "muted" {
		t.Fatalf("expected muted status patch, got %+v", patchPayload)
	}
	if patchPayload.SnoozeExpirationInSeconds == nil || *patchPayload.SnoozeExpirationInSeconds != int64(7200) {
		t.Fatalf("unexpected mute expiration in patch: %+v", patchPayload)
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

func runIssueListCommand(t *testing.T, expectedPath string, responseBody string, run func() error) (*bytes.Buffer, error) {
	t.Helper()

	stdout := setupServerAndStdout(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, responseBody)
	}))

	err := run()

	return stdout, err
}

func TestShowCommandInvalidCounter(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"show", "not-a-number"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestShowCommandRejectsZeroCounter(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"show", "0"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "item counter must be greater than 0") {
		t.Fatalf("expected zero counter error, got %v", err)
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
	t.Setenv("CI", "")

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

func TestParseIssueFilters(t *testing.T) {
	tests := []struct {
		name    string
		flags   rootFlags
		wantErr bool
	}{
		{name: "valid rfc3339 and counts", flags: rootFlags{Environment: "production", Status: "active", Since: "2026-02-19T10:00:00Z", Until: "2026-02-19T12:00:00Z", MinOccurrences: "5", MaxOccurrences: "9"}},
		{name: "valid unix seconds", flags: rootFlags{Since: "1771495200"}},
		{name: "negative unix seconds", flags: rootFlags{Since: "-1"}, wantErr: true},
		{name: "invalid since", flags: rootFlags{Since: "not-a-time"}, wantErr: true},
		{name: "invalid min occurrences", flags: rootFlags{MinOccurrences: "x"}, wantErr: true},
		{name: "since after until", flags: rootFlags{Since: "2026-02-19T13:00:00Z", Until: "2026-02-19T12:00:00Z"}, wantErr: true},
		{name: "min greater than max", flags: rootFlags{MinOccurrences: "10", MaxOccurrences: "9"}, wantErr: true},
	}

	for _, tc := range tests {
		filters, err := parseIssueFilters(tc.flags)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if tc.flags.Environment != "" && filters.Environment != tc.flags.Environment {
			t.Fatalf("%s: environment mismatch", tc.name)
		}
	}
}

func TestParseFilterTime(t *testing.T) {
	now := time.Unix(1771495200, 0).UTC()
	parsed, err := parseFilterTime("1771495200")
	if err != nil || parsed == nil || !parsed.Equal(now) {
		t.Fatalf("parseFilterTime(unix) = %v, %v", parsed, err)
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

func newActionSuccessHandler(t *testing.T, capturedPatch *rollbar.ItemPatch) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/1/item_by_counter/269":
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"itemId":1755568172}}`)
		case "/api/1/item/1755568172":
			if r.Method != http.MethodPatch {
				t.Fatalf("unexpected method for patch endpoint: %s", r.Method)
			}
			if capturedPatch != nil {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if err := json.Unmarshal(body, capturedPatch); err != nil {
					t.Fatalf("Unmarshal() error = %v", err)
				}
			}
			_, _ = fmt.Fprint(w, `{"err":0,"result":{}}`)
		case "/api/1/item/1755568172/":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method for get item endpoint: %s", r.Method)
			}
			_, _ = fmt.Fprint(w, `{"err":0,"result":{"id":1755568172,"project_id":766510,"counter":269,"title":"RST_STREAM","status":"active","environment":"production","total_occurrences":7}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
}
