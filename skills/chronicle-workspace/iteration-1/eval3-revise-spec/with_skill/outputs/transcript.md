# Transcript — eval3-revise-spec (with_skill)

## Setup

```
mkdir -p /tmp/claude/skill-eval/eval3-revise-spec/with_skill
cp -r /Users/heman/developer/chronicle/skills/chronicle/evals/fixtures/oauth-implemented \
  /tmp/claude/skill-eval/eval3-revise-spec/with_skill/repo
cd /tmp/claude/skill-eval/eval3-revise-spec/with_skill/repo
export PATH=/tmp/claude:$PATH
```

## Commands run, in order

1. `chron spec current oauth-login`
   → `implemented oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation OAuth login with Google, GitHub, and refresh token rotation`

2. `chron spec history oauth-login`
   → same single entry (v1, implemented) — confirmed there is exactly one version in the chain so far.

3. Read the actual v1 file directly
   (`.chronicle/specs/oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation.md`)
   to see its frontmatter (`status: implemented`) and body (title + empty
   `## Requirements` section — the fixture's v1 has no requirements text
   filled in, but the status is unambiguously `implemented`, which is what
   matters for the immutability rule).

4. `chron spec list` — sanity check, confirmed `oauth-login` is the only
   capability and its current status is `implemented`.

5. `chron --help`, `chron spec revise --help` (invalid subcommand, but
   printed usage) — confirmed the exact CLI syntax before running a
   mutating command.

6. **`chron spec revise oauth-login "OAuth login with Google, GitHub, Apple Sign-In, and refresh token rotation"`**
   → created
   `.chronicle/specs/oauth-login/v2-oauth-login-with-google-github-apple-sign-in-and-refresh-token-rotation.md`
   with `status: proposed`, `supersedes:
   oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation`,
   and a stub `## Requirements` section. This is the correct command per the
   skill: requirements changed (Apple Sign-In added mid-flight, which was
   out of scope at v1), so the existing tip (already `implemented`) must be
   superseded by a new full snapshot rather than edited.

7. **Edit** — filled in the *new* v2 file (not v1) with:
   - A "What changed and why" section explaining that refresh-token
     rotation shipped to prod and is carried forward unchanged, and that
     Apple Sign-In was added mid-implementation per a product decision.
   - A full, self-contained `## Requirements` section covering Google,
     GitHub, and Apple Sign-In login, the refresh-token-rotation behavior
     (including reuse detection), account linking, and profile
     normalization — a complete snapshot, not a diff against v1.

8. `cat` on the v1 file after the revise, plus `chron spec current
   oauth-login` and `chron spec history oauth-login`, to confirm:
   - v1's `status` flipped from `implemented` → `superseded` and gained
     `superseded_by: oauth-login/v2-...`, with `updated` bumped.
   - v1's **title and body content (the `# OAuth login...` heading and the
     empty `## Requirements` section) were byte-for-byte unchanged** —
     `chron spec revise` only touched v1's frontmatter metadata, never its
     content.
   - `chron spec current` now resolves to v2 (`status: proposed`).
   - `chron spec history` shows both versions in order: v1 (superseded) →
     v2 (proposed).

## Did I ever directly edit the v1 (implemented) spec file's content?

**No.** I only read v1 to understand the existing scope. All edits were
made to the newly-created v2 file via `chron spec revise` followed by
filling in its body with `Edit`. The only change `chron` itself made to
the v1 file was flipping `status`/`superseded_by`/`updated` in its
frontmatter — exactly the metadata-only mutation the ledger model permits
(`chron spec revise` did this automatically; I did not hand-edit
`superseded_by`). v1's title, `# ` heading, and `## Requirements` body are
identical before and after.

## Reasoning

The user's message describes two things that both matter for the ledger
model:

- Refresh-token-rotation **shipped and is working in prod** — this is
  already captured by v1 being `status: implemented`; no action needed on
  that front, and definitely not a hand-edit to "correct" or annotate v1.
- Requirements **changed mid-flight** (Apple Sign-In added, out of scope at
  v1) — per the skill, this is exactly the trigger for `chron spec revise
  <capability> "<new-title>"`, even though the capability is already
  implemented. Revise creates a new full snapshot (not a diff) and
  automatically supersedes the current tip; it does not touch v1's content,
  only its status/superseded_by metadata.

The new v2 spec is `status: proposed` (the new plan to implement against —
adding Apple Sign-In support), with refresh-token-rotation's already-shipped
requirements folded into the same snapshot since the spec must be
self-contained, not a delta.
