package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAddUseCycleRemove(t *testing.T) {
	t.Parallel()

	store, _ := newTempStore(t)

	if err := store.AddProject("a", "token-a"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if err := store.AddProject("b", "token-b"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}

	token, name, err := store.ResolveToken("")
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token == "" || name == "" {
		t.Fatalf("ResolveToken() returned empty values")
	}

	if err := store.UseProject("b"); err != nil {
		t.Fatalf("UseProject() error = %v", err)
	}
	if next, err := store.CycleProject(); err != nil || next == "" {
		t.Fatalf("CycleProject() = %q, err=%v", next, err)
	}

	if err := store.RemoveProject("a"); err != nil {
		t.Fatalf("RemoveProject() error = %v", err)
	}
}

func TestStorePermissions(t *testing.T) {
	t.Parallel()

	store, path := newTempStore(t)
	if err := store.AddProject("a", "token-a"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreErrors(t *testing.T) {
	t.Parallel()

	store, _ := newTempStore(t)

	if err := store.AddProject("", "token"); err == nil {
		t.Fatalf("expected empty name error")
	}
	if err := store.AddProject("name", ""); err == nil {
		t.Fatalf("expected empty token error")
	}
	if err := store.UseProject("missing"); err == nil {
		t.Fatalf("expected missing project error")
	}
	if _, err := store.CycleProject(); err == nil {
		t.Fatalf("expected no configured projects error")
	}
	if _, _, err := store.ResolveToken("missing"); err == nil {
		t.Fatalf("expected missing project error")
	}
}

func TestStoreAddProjectUpdatesExisting(t *testing.T) {
	t.Parallel()

	store, _ := newTempStore(t)
	if err := store.AddProject("same", "token-1"); err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if err := store.AddProject("same", "token-2"); err != nil {
		t.Fatalf("AddProject() update error = %v", err)
	}

	token, _, err := store.ResolveToken("same")
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "token-2" {
		t.Fatalf("ResolveToken() token = %q", token)
	}
}

func TestStoreLoadDecodeError(t *testing.T) {
	t.Parallel()

	_, path := newTempStore(t)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store := NewStoreAtPath(path)
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestNewStorePath(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	store, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if !filepath.IsAbs(store.Path()) {
		t.Fatalf("expected absolute config path, got %q", store.Path())
	}
}

func newTempStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	return NewStoreAtPath(path), path
}
