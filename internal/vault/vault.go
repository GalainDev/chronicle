// Package vault resolves which Chronicle vault applies to the current
// working directory and maintains the registry that lets vaults link to
// each other.
package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirName        = ".chronicle"
	notesSubdir    = "notes"
	specsSubdir    = "specs"
	envVault       = "CHRONICLE_VAULT"
	defaultGlobal  = "chronicle-vault"
	registryEnv    = "CHRONICLE_REGISTRY"
	registryRelDir = "chron"
)

// Vault is a resolved Chronicle vault: a directory containing a notes/ tree
// and, for per-repo vaults, a specs/ tree (spec-driven dev is repo-scoped —
// the global vault has no meaningful use for it, but nothing stops it from
// existing there too).
type Vault struct {
	Name     string // registry name, used in cross-vault links as "name:path/to/note"
	Root     string // directory containing notes/ (the .chronicle dir, or the vault repo root)
	NotesDir string
	SpecsDir string
}

// Resolve finds the vault that applies to cwd: the nearest ancestor
// directory containing .chronicle/, walking up like git finds .git/. If
// none is found, falls back to the global vault (CHRONICLE_VAULT env, or
// ~/developer/chronicle-vault by default).
func Resolve(cwd string) (*Vault, error) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(dir, dirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return &Vault{
				Name:     filepath.Base(dir),
				Root:     candidate,
				NotesDir: filepath.Join(candidate, notesSubdir),
				SpecsDir: filepath.Join(candidate, specsSubdir),
			}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return globalVault()
}

func globalVault() (*Vault, error) {
	root := os.Getenv(envVault)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(home, "developer", defaultGlobal)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("no .chronicle/ found above cwd, and global vault %s does not exist (set %s or run chron init there)", root, envVault)
	}
	return &Vault{
		Name:     defaultGlobal,
		Root:     root,
		NotesDir: filepath.Join(root, notesSubdir),
		SpecsDir: filepath.Join(root, specsSubdir),
	}, nil
}

// Init scaffolds a new vault at root: root/notes/index.md plus a minimal
// index. Safe to call on an existing vault (no-op if notes/index.md exists).
// name is the vault's registry name (the CLI decides root and name: a local
// per-repo vault uses root=<repo>/.chronicle, name=<repo>; a dedicated vault
// repo like chronicle-vault uses root=<repo> directly, name=<repo>, with no
// .chronicle/ wrapper since the whole repo already serves that role).
func Init(root, name string) (*Vault, error) {
	notesDir := filepath.Join(root, notesSubdir)
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		return nil, err
	}
	index := filepath.Join(notesDir, "index.md")
	if _, err := os.Stat(index); os.IsNotExist(err) {
		content := "# Index\n\nChronicle vault index. `chron new` keeps this updated;\nfollow links from here or start with `chron list`.\n"
		if err := os.WriteFile(index, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	return &Vault{
		Name:     name,
		Root:     root,
		NotesDir: notesDir,
		SpecsDir: filepath.Join(root, specsSubdir),
	}, nil
}

// Registry is the set of known vaults by name, used to resolve cross-vault
// links written as "vaultname:relative/path.md".
type Registry struct {
	Vaults map[string]string `json:"vaults"` // name -> absolute root path
	path   string
}

func registryPath() (string, error) {
	if p := os.Getenv(registryEnv); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", registryRelDir, "vaults.json"), nil
}

// LoadRegistry reads the registry, returning an empty one if it doesn't exist yet.
func LoadRegistry() (*Registry, error) {
	p, err := registryPath()
	if err != nil {
		return nil, err
	}
	r := &Registry{Vaults: map[string]string{}, path: p}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	r.path = p
	return r, nil
}

// Save persists the registry to disk.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

// Register adds or updates a vault's location in the registry.
func (r *Registry) Register(name, root string) {
	if r.Vaults == nil {
		r.Vaults = map[string]string{}
	}
	r.Vaults[name] = root
}

// ResolveCrossLink resolves a "vaultname:relative/path" reference to an
// absolute note path using the registry. Returns an error if the vault
// name is unknown.
func (r *Registry) ResolveCrossLink(vaultName, relPath string) (string, error) {
	root, ok := r.Vaults[vaultName]
	if !ok {
		return "", fmt.Errorf("unknown vault %q in registry (known: %v)", vaultName, r.knownNames())
	}
	return filepath.Join(root, notesSubdir, relPath), nil
}

func (r *Registry) knownNames() []string {
	names := make([]string, 0, len(r.Vaults))
	for n := range r.Vaults {
		names = append(names, n)
	}
	return names
}
