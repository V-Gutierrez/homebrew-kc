package cli_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/v-gutierrez/kc/internal/cli"
)

// --- Mock implementations ---

type mockStore struct {
	data map[string]map[string]string // vault -> key -> value
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]map[string]string)}
}

func (m *mockStore) Get(vault, key string) (string, error) {
	v, ok := m.data[vault]
	if !ok {
		return "", fmt.Errorf("vault %q not found", vault)
	}
	val, ok := v[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in vault %q", key, vault)
	}
	return val, nil
}

func (m *mockStore) Set(vault, key, value string) error {
	if m.data[vault] == nil {
		m.data[vault] = make(map[string]string)
	}
	m.data[vault][key] = value
	return nil
}

func (m *mockStore) Delete(vault, key string) error {
	v, ok := m.data[vault]
	if !ok {
		return fmt.Errorf("vault %q not found", vault)
	}
	if _, ok := v[key]; !ok {
		return fmt.Errorf("key %q not found in vault %q", key, vault)
	}
	delete(v, key)
	return nil
}

func (m *mockStore) List(vault string) ([]string, error) {
	v, ok := m.data[vault]
	if !ok {
		return nil, nil
	}
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	return keys, nil
}

type mockVaultManager struct {
	vaults []string
	active string
}

func (m *mockVaultManager) List() ([]string, error) {
	return m.vaults, nil
}

func (m *mockVaultManager) Create(name string) error {
	for _, v := range m.vaults {
		if v == name {
			return fmt.Errorf("vault %q already exists", name)
		}
	}
	m.vaults = append(m.vaults, name)
	return nil
}

func (m *mockVaultManager) Active() (string, error) {
	return m.active, nil
}

func (m *mockVaultManager) Switch(name string) error {
	for _, v := range m.vaults {
		if v == name {
			m.active = name
			return nil
		}
	}
	return fmt.Errorf("vault %q does not exist", name)
}

type mockClipboard struct {
	last string
}

func (m *mockClipboard) Copy(value string) error {
	m.last = value
	return nil
}

// --- Helpers ---

func newTestApp() (*cli.App, *mockStore, *mockVaultManager, *mockClipboard) {
	store := newMockStore()
	vaults := &mockVaultManager{vaults: []string{"default"}, active: "default"}
	clip := &mockClipboard{}
	app := &cli.App{Store: store, Vaults: vaults, Clipboard: clip}
	return app, store, vaults, clip
}

func executeCmd(app *cli.App, args ...string) (stdout, stderr string, err error) {
	root := cli.NewRootCmd(app)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// --- Tests ---

func TestGetSuccess(t *testing.T) {
	app, store, _, clip := newTestApp()
	store.Set("default", "API_KEY", "secret123")

	stdout, stderr, err := executeCmd(app, "get", "API_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "********" {
		t.Errorf("stdout = %q, want %q", got, "********")
	}
	if !strings.Contains(stderr, "Copied to clipboard") {
		t.Errorf("stderr = %q, want clipboard confirmation", stderr)
	}
	if clip.last != "secret123" {
		t.Errorf("clipboard = %q, want %q", clip.last, "secret123")
	}
}

func TestGetNotFound(t *testing.T) {
	app, _, _, _ := newTestApp()

	_, _, err := executeCmd(app, "get", "NOPE")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "NOPE") {
		t.Errorf("error = %q, want it to mention the key", err.Error())
	}
}

func TestGetWithVaultFlag(t *testing.T) {
	app, store, vaults, _ := newTestApp()
	vaults.Create("staging")
	store.Set("staging", "DB_PASS", "pg123")

	stdout, _, err := executeCmd(app, "get", "DB_PASS", "--vault", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "********" {
		t.Errorf("stdout = %q, want %q", got, "********")
	}
}

func TestGetMissingArg(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "get")
	if err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestSetSuccess(t *testing.T) {
	app, store, _, _ := newTestApp()

	stdout, _, err := executeCmd(app, "set", "TOKEN", "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Stored") {
		t.Errorf("stdout = %q, want confirmation", stdout)
	}

	val, _ := store.Get("default", "TOKEN")
	if val != "abc" {
		t.Errorf("stored value = %q, want %q", val, "abc")
	}
}

func TestSetWithVaultFlag(t *testing.T) {
	app, store, vaults, _ := newTestApp()
	vaults.Create("prod")

	_, _, err := executeCmd(app, "set", "KEY", "val", "--vault", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, _ := store.Get("prod", "KEY")
	if val != "val" {
		t.Errorf("stored value = %q, want %q", val, "val")
	}
}

func TestSetWithUnknownVaultFlag(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "set", "KEY", "val", "--vault", "missing")
	if err == nil {
		t.Fatal("expected error for unknown vault")
	}
}

func TestGetWithInvalidVaultFlag(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "get", "KEY", "--vault", "bad name")
	if err == nil {
		t.Fatal("expected error for invalid vault name")
	}
}

func TestSetMissingArgs(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "set", "KEY")
	if err == nil {
		t.Fatal("expected error for missing value argument")
	}
}

func TestDelSuccess(t *testing.T) {
	app, store, _, _ := newTestApp()
	store.Set("default", "OLD_KEY", "oldval")

	stdout, _, err := executeCmd(app, "del", "OLD_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Deleted") {
		t.Errorf("stdout = %q, want confirmation", stdout)
	}

	_, getErr := store.Get("default", "OLD_KEY")
	if getErr == nil {
		t.Error("expected key to be deleted")
	}
}

func TestDelNotFound(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "del", "GHOST")
	if err == nil {
		t.Fatal("expected error deleting non-existent key")
	}
}

func TestDelAlias(t *testing.T) {
	app, store, _, _ := newTestApp()
	store.Set("default", "X", "y")

	_, _, err := executeCmd(app, "rm", "X")
	if err != nil {
		t.Fatalf("unexpected error using 'rm' alias: %v", err)
	}
}

func TestListSuccess(t *testing.T) {
	app, store, _, _ := newTestApp()
	store.Set("default", "A", "1")
	store.Set("default", "B", "2")

	stdout, _, err := executeCmd(app, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "A") || !strings.Contains(stdout, "B") {
		t.Errorf("stdout = %q, want keys A and B", stdout)
	}
}

func TestListEmpty(t *testing.T) {
	app, _, _, _ := newTestApp()

	stdout, _, err := executeCmd(app, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "No keys") {
		t.Errorf("stdout = %q, want empty message", stdout)
	}
}

func TestListWithVault(t *testing.T) {
	app, store, vaults, _ := newTestApp()
	vaults.Create("staging")
	store.Set("staging", "DB_HOST", "localhost")

	stdout, _, err := executeCmd(app, "list", "--vault", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "DB_HOST") {
		t.Errorf("stdout = %q, want DB_HOST", stdout)
	}
}

func TestListAlias(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "ls")
	if err != nil {
		t.Fatalf("unexpected error using 'ls' alias: %v", err)
	}
}

func TestVaultList(t *testing.T) {
	app, _, _, _ := newTestApp()

	stdout, _, err := executeCmd(app, "vault", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "default") {
		t.Errorf("stdout = %q, want default vault", stdout)
	}
	// Active vault should be marked with *.
	if !strings.Contains(stdout, "* default") {
		t.Errorf("stdout = %q, want active marker on default", stdout)
	}
}

func TestVaultCreate(t *testing.T) {
	app, _, vaults, _ := newTestApp()

	stdout, _, err := executeCmd(app, "vault", "create", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Created") {
		t.Errorf("stdout = %q, want confirmation", stdout)
	}
	if len(vaults.vaults) != 2 {
		t.Errorf("vaults count = %d, want 2", len(vaults.vaults))
	}
}

func TestVaultCreateDuplicate(t *testing.T) {
	app, _, _, _ := newTestApp()

	_, _, err := executeCmd(app, "vault", "create", "default")
	if err == nil {
		t.Fatal("expected error creating duplicate vault")
	}
}

func TestVaultSwitch(t *testing.T) {
	app, _, vaults, _ := newTestApp()
	vaults.Create("prod")

	stdout, _, err := executeCmd(app, "vault", "switch", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Switched") {
		t.Errorf("stdout = %q, want confirmation", stdout)
	}
	if vaults.active != "prod" {
		t.Errorf("active = %q, want %q", vaults.active, "prod")
	}
}

func TestVaultSwitchNonexistent(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "vault", "switch", "nope")
	if err == nil {
		t.Fatal("expected error switching to non-existent vault")
	}
}

func TestActiveVaultFallback(t *testing.T) {
	app, store, vaults, _ := newTestApp()
	vaults.Create("staging")
	vaults.Switch("staging")
	store.Set("staging", "MYKEY", "myval")

	// No --vault flag → uses active vault (staging).
	stdout, _, err := executeCmd(app, "get", "MYKEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "********" {
		t.Errorf("stdout = %q, want %q", got, "********")
	}
}

func TestDefaultVaultFallback(t *testing.T) {
	store := newMockStore()
	// VaultManager with empty active.
	vaults := &mockVaultManager{vaults: []string{"default"}, active: ""}
	app := &cli.App{Store: store, Vaults: vaults, Clipboard: nil}

	store.Set("default", "X", "y")

	stdout, _, err := executeCmd(app, "get", "X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "********" {
		t.Errorf("stdout = %q, want %q", got, "********")
	}
}

func TestGetWithNilClipboard(t *testing.T) {
	store := newMockStore()
	vaults := &mockVaultManager{vaults: []string{"default"}, active: "default"}
	app := &cli.App{Store: store, Vaults: vaults, Clipboard: nil}

	store.Set("default", "K", "V")

	stdout, stderr, err := executeCmd(app, "get", "K")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "********" {
		t.Errorf("stdout = %q, want %q", got, "********")
	}
	if strings.Contains(stderr, "clipboard") {
		t.Error("should not mention clipboard when nil")
	}
}

func TestGetEmptyValueMask(t *testing.T) {
	app, store, _, _ := newTestApp()
	store.Set("default", "EMPTY", "")

	stdout, _, err := executeCmd(app, "get", "EMPTY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "[empty]" {
		t.Errorf("stdout = %q, want %q", got, "[empty]")
	}
}

func TestRootHelp(t *testing.T) {
	app, _, _, _ := newTestApp()

	stdout, _, err := executeCmd(app, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "kc") {
		t.Errorf("help output should mention 'kc'")
	}
	// Verify all subcommands are listed.
	for _, sub := range []string{"get", "set", "del", "list", "vault"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestVersion(t *testing.T) {
	app, _, _, _ := newTestApp()

	stdout, _, err := executeCmd(app, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("version output = %q, want 'dev'", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	app, _, _, _ := newTestApp()
	_, _, err := executeCmd(app, "bogus")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}
