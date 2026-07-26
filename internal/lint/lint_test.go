package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GalainDev/chronicle/internal/spec"
	"github.com/GalainDev/chronicle/internal/vault"
)

func testVault(t *testing.T) *vault.Vault {
	t.Helper()
	root := t.TempDir()
	v, err := vault.Init(root, "test")
	if err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	return v
}

func kinds(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, iss := range issues {
		out[i] = iss.Kind
	}
	return out
}

func contains(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func TestCleanSpecChainHasNoIssues(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	if _, err := spec.Implement(v.SpecsDir, "oauth-login"); err != nil {
		t.Fatalf("spec.Implement: %v", err)
	}
	if _, err := spec.Revise(v.SpecsDir, "oauth-login", "OAuth login with refresh", "work"); err != nil {
		t.Fatalf("spec.Revise: %v", err)
	}

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues on a clean chain, got %v", issues)
	}
}

func TestSpecCapabilityMismatch(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	path := filepath.Join(v.SpecsDir, "oauth-login", "v1-oauth-login-v1.md")
	rewriteField(t, path, "capability: oauth-login", "capability: wrong-name")

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !contains(kinds(issues), "spec_capability_mismatch") {
		t.Fatalf("expected spec_capability_mismatch, got %v", issues)
	}
}

func TestSpecBadFilename(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	oldPath := filepath.Join(v.SpecsDir, "oauth-login", "v1-oauth-login-v1.md")
	newPath := filepath.Join(v.SpecsDir, "oauth-login", "oauth-login-v1.md")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !contains(kinds(issues), "spec_bad_filename") {
		t.Fatalf("expected spec_bad_filename, got %v", issues)
	}
}

func TestSpecDanglingLink(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	path := filepath.Join(v.SpecsDir, "oauth-login", "v1-oauth-login-v1.md")
	appendField(t, path, "superseded_by: oauth-login/v2-does-not-exist")

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !contains(kinds(issues), "spec_dangling_link") {
		t.Fatalf("expected spec_dangling_link, got %v", issues)
	}
	// a dangling superseded_by also means there's no resolvable tip left
	if !contains(kinds(issues), "spec_broken_chain") {
		t.Fatalf("expected spec_broken_chain (tip points nowhere), got %v", issues)
	}
}

func TestSpecForkedChain(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	// hand-craft a second v2 that does NOT mark v1 as superseded --
	// simulates two independent tips (a forked chain).
	v2 := filepath.Join(v.SpecsDir, "oauth-login", "v2-forked.md")
	content := "---\ntype: spec\ncapability: oauth-login\nstatus: proposed\n---\n\n# Forked\n"
	if err := os.WriteFile(v2, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !contains(kinds(issues), "spec_broken_chain") {
		t.Fatalf("expected spec_broken_chain on a forked capability, got %v", issues)
	}
}

func TestSpecInvalidStatus(t *testing.T) {
	v := testVault(t)
	if _, err := spec.New(v.SpecsDir, "oauth-login", "OAuth login v1", "work"); err != nil {
		t.Fatalf("spec.New: %v", err)
	}
	path := filepath.Join(v.SpecsDir, "oauth-login", "v1-oauth-login-v1.md")
	rewriteField(t, path, "status: proposed", "status: bogus")

	issues, err := Run(v, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !contains(kinds(issues), "invalid_status") {
		t.Fatalf("expected invalid_status, got %v", issues)
	}
}

func rewriteField(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, old) {
		t.Fatalf("field %q not found in %s", old, path)
	}
	updated := strings.Replace(content, old, new, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// appendField inserts line as a new frontmatter field, just before the
// closing "---" delimiter (the closing delimiter is always preceded by a
// "\n---\n" once the frontmatter has at least one field, which every note
// written by this package does).
func appendField(t *testing.T, path, line string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "\n---\n") {
		t.Fatalf("closing frontmatter delimiter not found in %s", path)
	}
	updated := strings.Replace(content, "\n---\n", "\n"+line+"\n---\n", 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
