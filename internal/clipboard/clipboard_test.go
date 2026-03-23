package clipboard

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/v-gutierrez/kc/internal/keychain"
)

type fakeRunner struct {
	calls   []fakeCall
	results []fakeResult
}

type fakeCall struct {
	Name string
	Args []string
}

type fakeResult struct {
	Output []byte
	Err    error
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, fakeCall{Name: name, Args: args})
	if len(f.results) == 0 {
		return nil, fmt.Errorf("fakeRunner: no results queued")
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.Output, r.Err
}

func (f *fakeRunner) RunWithInput(input string, name string, args ...string) ([]byte, error) {
	callArgs := append([]string{"stdin=" + input}, args...)
	f.calls = append(f.calls, fakeCall{Name: name, Args: callArgs})
	if len(f.results) == 0 {
		return nil, fmt.Errorf("fakeRunner: no results queued")
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.Output, r.Err
}

func (f *fakeRunner) Start(name string, args []string, env []string) error {
	callArgs := append(append([]string{}, args...), env...)
	f.calls = append(f.calls, fakeCall{Name: name, Args: callArgs})
	if len(f.results) == 0 {
		return nil
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.Err
}

func TestCopy_Success(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: nil, Err: nil}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: 0}

	if err := cb.Copy("my-secret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.calls))
	}
	if runner.calls[0].Name != "pbcopy" {
		t.Fatalf("expected pbcopy, got %q", runner.calls[0].Name)
	}
	if runner.calls[0].Args[0] != "stdin=my-secret" {
		t.Fatalf("expected stdin payload, got %v", runner.calls[0].Args)
	}
}

func TestCopy_Error(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: []byte("pbcopy: command not found\n"), Err: fmt.Errorf("exit status 127")}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: 0}

	err := cb.Copy("test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "clipboard write") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCopy_StartsDetachedClear(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: nil, Err: nil}},
	}
	cb := &Clipboard{
		Runner:     runner,
		ClearDelay: 30 * time.Second,
		Executable: "/tmp/kc",
	}

	if err := cb.Copy("my-secret"); err != nil {
		t.Fatal(err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(runner.calls), runner.calls)
	}
	if runner.calls[1].Name != "/tmp/kc" {
		t.Fatalf("expected detached helper to start /tmp/kc, got %q", runner.calls[1].Name)
	}
	joined := strings.Join(runner.calls[1].Args, " ")
	if !strings.Contains(joined, envClearAfter+"=30") {
		t.Fatalf("expected clear-after env, got %v", runner.calls[1].Args)
	}
	digest := keychain.Digest("my-secret")
	if !strings.Contains(joined, envClearDigest+"="+digest) {
		t.Fatalf("expected clipboard digest, got %v", runner.calls[1].Args)
	}
}

func TestCopy_DetachedClearRequiresExecutable(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: nil, Err: nil}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: time.Second}

	err := cb.Copy("my-secret")
	if err == nil {
		t.Fatal("expected error when executable path is missing")
	}
}

func TestClear(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: nil, Err: nil}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: 0}

	if err := cb.Clear(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.calls))
	}
}

func TestRead_Success(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: []byte("clipboard-contents"), Err: nil}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: 0}

	val, err := cb.Read()
	if err != nil {
		t.Fatal(err)
	}
	if val != "clipboard-contents" {
		t.Fatalf("got %q, want %q", val, "clipboard-contents")
	}
	if runner.calls[0].Name != "pbpaste" {
		t.Fatalf("expected pbpaste, got %q", runner.calls[0].Name)
	}
}

func TestRead_Error(t *testing.T) {
	runner := &fakeRunner{
		results: []fakeResult{{Output: nil, Err: fmt.Errorf("exit status 1")}},
	}
	cb := &Clipboard{Runner: runner, ClearDelay: 0}

	if _, err := cb.Read(); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunClearIfRequested_NoopWithoutEnv(t *testing.T) {
	handled, err := RunClearIfRequested()
	if err != nil {
		t.Fatal(err)
	}
	if handled {
		t.Fatal("expected helper mode to be disabled")
	}
}

func TestRunClearIfRequested_MissingDigest(t *testing.T) {
	t.Setenv(envClearAfter, "1")
	handled, err := RunClearIfRequested()
	if err == nil {
		t.Fatal("expected missing digest error")
	}
	if !handled {
		t.Fatal("expected helper mode to be handled")
	}
}

func TestRunClearIfRequested_InvalidDelay(t *testing.T) {
	t.Setenv(envClearAfter, "abc")
	t.Setenv(envClearDigest, keychain.Digest("x"))
	handled, err := RunClearIfRequested()
	if err == nil {
		t.Fatal("expected invalid delay error")
	}
	if !handled {
		t.Fatal("expected helper mode to be handled")
	}
}

func TestDefaultClearDelay(t *testing.T) {
	if DefaultClearDelay != 30*time.Second {
		t.Fatalf("DefaultClearDelay = %v, want 30s", DefaultClearDelay)
	}
}

func TestNew(t *testing.T) {
	cb := New()
	if cb.Runner == nil {
		t.Fatal("Runner should not be nil")
	}
	if cb.ClearDelay != DefaultClearDelay {
		t.Fatalf("ClearDelay = %v, want %v", cb.ClearDelay, DefaultClearDelay)
	}
	if cb.Executable == "" {
		t.Fatal("Executable should not be empty")
	}
}

func TestRunClearIfRequested_ClearsMatchingClipboard(t *testing.T) {
	t.Setenv(envClearAfter, "0")
	t.Setenv(envClearDigest, keychain.Digest("same-value"))

	binDir := t.TempDir()
	logFile := binDir + "/pbcopy.log"
	pbpastePath := binDir + "/pbpaste"
	pbcopyPath := binDir + "/pbcopy"

	if err := os.WriteFile(pbpastePath, []byte("#!/bin/sh\nprintf 'same-value'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbcopyPath, []byte("#!/bin/sh\ncat > /dev/null\nprintf 'cleared' >> \"$PB_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("PB_LOG", logFile)

	handled, err := RunClearIfRequested()
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected helper mode to be handled")
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cleared" {
		t.Fatalf("expected clear action, got %q", string(data))
	}
}

func TestRunClearIfRequested_SkipsWhenClipboardChanged(t *testing.T) {
	t.Setenv(envClearAfter, "0")
	t.Setenv(envClearDigest, keychain.Digest("same-value"))

	binDir := t.TempDir()
	logFile := binDir + "/pbcopy.log"
	pbpastePath := binDir + "/pbpaste"
	pbcopyPath := binDir + "/pbcopy"

	if err := os.WriteFile(pbpastePath, []byte("#!/bin/sh\nprintf 'different-value'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbcopyPath, []byte("#!/bin/sh\ncat > /dev/null\nprintf 'cleared' >> \"$PB_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("PB_LOG", logFile)

	handled, err := RunClearIfRequested()
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected helper mode to be handled")
	}

	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Fatalf("expected no clear action, stat err=%v", err)
	}
}
