package clipboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/v-gutierrez/kc/internal/keychain"
)

const (
	// DefaultClearDelay is how long before clipboard auto-clears.
	DefaultClearDelay = 30 * time.Second
	envClearAfter     = "KC_CLIPBOARD_CLEAR_AFTER"
	envClearDigest    = "KC_CLIPBOARD_CLEAR_DIGEST"
)

type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
	RunWithInput(input string, name string, args ...string) ([]byte, error)
	Start(name string, args []string, env []string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (ExecRunner) RunWithInput(input string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.CombinedOutput()
}

func (ExecRunner) Start(name string, args []string, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.Start()
}

type Clipboard struct {
	Runner     CommandRunner
	ClearDelay time.Duration
	Executable string
}

func New() *Clipboard {
	executable, _ := os.Executable()
	return &Clipboard{
		Runner:     ExecRunner{},
		ClearDelay: DefaultClearDelay,
		Executable: executable,
	}
}

func (c *Clipboard) Copy(text string) error {
	if err := c.write(text); err != nil {
		return err
	}

	if c.ClearDelay > 0 {
		if err := c.startDetachedClear(text); err != nil {
			return err
		}
	}

	return nil
}

func (c *Clipboard) Clear() error {
	return c.write("")
}

func (c *Clipboard) Read() (string, error) {
	out, err := c.Runner.Run("pbpaste")
	if err != nil {
		return "", fmt.Errorf("clipboard read: %w: %s", err, string(out))
	}
	return string(out), nil
}

func (c *Clipboard) write(text string) error {
	out, err := c.Runner.RunWithInput(text, "pbcopy")
	if err != nil {
		return fmt.Errorf("clipboard write: %w: %s", err, string(out))
	}
	return nil
}

func (c *Clipboard) startDetachedClear(original string) error {
	if c.Executable == "" {
		return errors.New("clipboard auto-clear: executable path unavailable")
	}

	env := []string{
		fmt.Sprintf("%s=%d", envClearAfter, c.ClearDelay/time.Second),
		fmt.Sprintf("%s=%s", envClearDigest, keychain.Digest(original)),
	}

	if err := c.Runner.Start(c.Executable, nil, env); err != nil {
		return fmt.Errorf("clipboard auto-clear: %w", err)
	}

	return nil
}

func RunClearIfRequested() (bool, error) {
	afterRaw := os.Getenv(envClearAfter)
	if afterRaw == "" {
		return false, nil
	}

	expectedDigest := os.Getenv(envClearDigest)
	if expectedDigest == "" {
		return true, errors.New("clipboard auto-clear: missing expected clipboard digest")
	}

	seconds, err := time.ParseDuration(afterRaw + "s")
	if err != nil {
		return true, fmt.Errorf("clipboard auto-clear: parse delay: %w", err)
	}

	time.Sleep(seconds)

	clipboard := New()
	current, err := clipboard.Read()
	if err != nil {
		return true, nil
	}

	if keychain.Digest(strings.TrimRight(current, "\n")) == expectedDigest {
		_ = clipboard.Clear()
	}

	return true, nil
}
