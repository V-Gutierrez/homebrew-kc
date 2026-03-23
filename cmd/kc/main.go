package main

import (
	"fmt"
	"os"

	"github.com/v-gutierrez/kc/internal/cli"
	"github.com/v-gutierrez/kc/internal/clipboard"
	"github.com/v-gutierrez/kc/internal/keychain"
	"github.com/v-gutierrez/kc/internal/vault"
)

func main() {
	handled, err := clipboard.RunClearIfRequested()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if handled {
		return
	}

	kc := keychain.New()
	vm := vault.New(kc)
	cb := clipboard.New()

	app := &cli.App{
		Store:     &storeAdapter{vm: vm},
		Vaults:    &vaultAdapter{vm: vm},
		Clipboard: cb,
	}

	root := cli.NewRootCmd(app)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// storeAdapter bridges vault.Manager to the cli.KeychainStore interface.
type storeAdapter struct {
	vm *vault.Manager
}

func (s *storeAdapter) Get(vaultName, key string) (string, error) {
	return s.vm.Get(key, vaultName)
}

func (s *storeAdapter) Set(vaultName, key, value string) error {
	return s.vm.Set(key, value, vaultName)
}

func (s *storeAdapter) Delete(vaultName, key string) error {
	return s.vm.Delete(key, vaultName)
}

func (s *storeAdapter) List(vaultName string) ([]string, error) {
	return s.vm.ListKeys(vaultName)
}

// vaultAdapter bridges vault.Manager to the cli.VaultManager interface.
type vaultAdapter struct {
	vm *vault.Manager
}

func (v *vaultAdapter) List() ([]string, error) {
	return v.vm.ListVaults()
}

func (v *vaultAdapter) Create(name string) error {
	return v.vm.Create(name)
}

func (v *vaultAdapter) Active() (string, error) {
	return v.vm.ActiveVault(), nil
}

func (v *vaultAdapter) Switch(name string) error {
	return v.vm.Switch(name)
}
