package spec

import "testing"

func TestLifecycle(t *testing.T) {
	dir := t.TempDir()

	n, err := New(dir, "oauth-login", "OAuth login v1", "work")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if n.Frontmatter.Status != "proposed" {
		t.Fatalf("New status = %q, want proposed", n.Frontmatter.Status)
	}

	cur, err := Current(dir, "oauth-login")
	if err != nil {
		t.Fatalf("Current after New: %v", err)
	}
	if cur.ID != n.ID {
		t.Fatalf("Current after New = %q, want %q", cur.ID, n.ID)
	}

	if _, err := Implement(dir, "oauth-login"); err != nil {
		t.Fatalf("Implement: %v", err)
	}
	cur, err = Current(dir, "oauth-login")
	if err != nil {
		t.Fatalf("Current after Implement: %v", err)
	}
	if cur.Frontmatter.Status != "implemented" {
		t.Fatalf("status after Implement = %q, want implemented", cur.Frontmatter.Status)
	}

	if _, err := Implement(dir, "oauth-login"); err == nil {
		t.Fatal("Implement twice: expected error, got nil")
	}

	v2, err := Revise(dir, "oauth-login", "OAuth login with refresh tokens", "work")
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if v2.Frontmatter.Supersedes != n.ID {
		t.Fatalf("v2.Supersedes = %q, want %q", v2.Frontmatter.Supersedes, n.ID)
	}

	cur, err = Current(dir, "oauth-login")
	if err != nil {
		t.Fatalf("Current after Revise: %v", err)
	}
	if cur.ID != v2.ID {
		t.Fatalf("Current after Revise = %q, want %q (tip did not advance)", cur.ID, v2.ID)
	}

	hist, err := History(dir, "oauth-login")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("History returned %d versions, want 2", len(hist))
	}
	if hist[0].ID != n.ID || hist[1].ID != v2.ID {
		t.Fatalf("History order = [%s, %s], want [%s, %s]", hist[0].ID, hist[1].ID, n.ID, v2.ID)
	}
	if hist[0].Frontmatter.Status != "superseded" {
		t.Fatalf("old tip status after Revise = %q, want superseded", hist[0].Frontmatter.Status)
	}
	if hist[0].Frontmatter.SupersededBy != v2.ID {
		t.Fatalf("old tip superseded_by = %q, want %q", hist[0].Frontmatter.SupersededBy, v2.ID)
	}

	caps, err := Capabilities(dir)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if len(caps) != 1 || caps[0] != "oauth-login" {
		t.Fatalf("Capabilities = %v, want [oauth-login]", caps)
	}
}

func TestCurrentUnknownCapability(t *testing.T) {
	dir := t.TempDir()
	if _, err := Current(dir, "does-not-exist"); err == nil {
		t.Fatal("Current on unknown capability: expected error, got nil")
	}
}

func TestNewRejectsDuplicateCapability(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir, "billing", "Billing v1", "work"); err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := New(dir, "billing", "Billing again", "work"); err == nil {
		t.Fatal("New on existing capability: expected error, got nil")
	}
}

func TestReviseWithoutExistingSpec(t *testing.T) {
	dir := t.TempDir()
	if _, err := Revise(dir, "ghost", "First version via revise", "work"); err == nil {
		t.Fatal("Revise with no prior spec: expected error, got nil")
	}
}
