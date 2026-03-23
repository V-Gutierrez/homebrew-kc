package cli

// KeychainStore abstracts CRUD operations against the macOS Keychain.
// The vault parameter corresponds to the Keychain "service" field (prefixed "kc:{vault}").
// The key parameter corresponds to the Keychain "account" field.
type KeychainStore interface {
	Get(vault, key string) (string, error)
	Set(vault, key, value string) error
	Delete(vault, key string) error
	List(vault string) ([]string, error)
}

// VaultManager handles vault lifecycle: listing, creating, switching the active vault.
// Active vault is persisted across invocations (e.g. in ~/.kc/config).
type VaultManager interface {
	List() ([]string, error)
	Create(name string) error
	Active() (string, error)
	Switch(name string) error
}

// Clipboard abstracts clipboard write + optional auto-clear.
type Clipboard interface {
	Copy(value string) error
}

// DefaultVault is the fallback when no --vault flag and no active vault override.
const DefaultVault = "default"
