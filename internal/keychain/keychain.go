// Package keychain wraps the macOS `security` CLI to manage Keychain items.
// It uses exec.Command (no CGo) so the binary stays pure Go.
package keychain

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Common errors returned by Keychain operations.
var (
	ErrNotFound = errors.New("keychain: item not found")
	ErrDup      = errors.New("keychain: item already exists")
)

// CommandRunner abstracts command execution so tests can inject fakes.
type CommandRunner interface {
	// Run executes a command and returns combined stdout, stderr, and error.
	Run(name string, args ...string) ([]byte, error)
}

// ExecRunner is the real implementation that calls os/exec.
type ExecRunner struct{}

// Run executes the given command via exec.Command.
func (ExecRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Keychain provides CRUD operations against the macOS Keychain.
type Keychain struct {
	Runner CommandRunner
}

// New returns a Keychain using the real exec runner.
func New() *Keychain {
	return &Keychain{Runner: ExecRunner{}}
}

// Get retrieves the password for (service, account).
func (k *Keychain) Get(service, account string) (string, error) {
	out, err := k.Runner.Run("security", "find-generic-password",
		"-s", service,
		"-a", account,
		"-w",
	)
	if err != nil {
		if strings.Contains(string(out), "could not be found") ||
			strings.Contains(string(out), "SecKeychainSearchCopyNext") {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain get: %w: %s", err, string(out))
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Set stores or updates the password for (service, account).
// It deletes any existing entry first, then adds the new one.
func (k *Keychain) Set(service, account, password string) error {
	// Delete existing (ignore "not found" errors).
	_ = k.Delete(service, account)

	out, err := k.Runner.Run("security", "add-generic-password",
		"-s", service,
		"-a", account,
		"-w", password,
		"-U", // update if duplicate (belt-and-suspenders)
	)
	if err != nil {
		return fmt.Errorf("keychain set: %w: %s", err, string(out))
	}
	return nil
}

// Delete removes the item for (service, account).
func (k *Keychain) Delete(service, account string) error {
	out, err := k.Runner.Run("security", "delete-generic-password",
		"-s", service,
		"-a", account,
	)
	if err != nil {
		if strings.Contains(string(out), "could not be found") ||
			strings.Contains(string(out), "SecKeychainSearchCopyNext") {
			return ErrNotFound
		}
		return fmt.Errorf("keychain delete: %w: %s", err, string(out))
	}
	return nil
}

// List returns account names for all generic-password items matching service.
// It parses the output of `security dump-keychain` filtered by service.
func (k *Keychain) List(service string) ([]string, error) {
	out, err := k.Runner.Run("security", "find-generic-password",
		"-s", service,
		"-g", "-a", "",
	)
	// If there are zero matches, security exits non-zero.
	// We fall through to the dump approach.
	_ = out
	_ = err

	// The reliable approach: dump the whole keychain, then filter.
	dumpOut, dumpErr := k.Runner.Run("security", "dump-keychain")
	if dumpErr != nil {
		return nil, fmt.Errorf("keychain list: %w: %s", dumpErr, string(dumpOut))
	}

	return parseAccounts(string(dumpOut), service), nil
}

// parseAccounts extracts "acct" values from dump-keychain output
// for entries whose "svce" (service) matches the target.
func parseAccounts(dump, service string) []string {
	var accounts []string
	for _, block := range strings.Split(dump, "class:") {
		itemService, itemAccount := parseBlock(block)
		if itemService != service || itemAccount == "" {
			continue
		}
		accounts = append(accounts, itemAccount)
	}
	return accounts
}

func parseBlock(block string) (string, string) {
	var service string
	var account string

	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, `"svce"`):
			service = extractQuotedValue(trimmed)
		case strings.Contains(trimmed, `"acct"`):
			account = extractQuotedValue(trimmed)
		}
	}

	return service, account
}

func Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

// extractQuotedValue pulls the last ="..." value from a dump-keychain attribute line.
func extractQuotedValue(line string) string {
	idx := strings.LastIndex(line, `="`)
	if idx < 0 {
		return ""
	}
	rest := line[idx+2:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}
