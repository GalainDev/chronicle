# Transcript — eval2-oauth-spec (with_skill)

## Task

"We're about to build OAuth login for this app — should support both
Google and GitHub, with refresh token rotation. Before any of us start
writing code, can you set up a spec for it?"

## Commands run, in order

1. `chron spec list`
   Ran with no `.chronicle/` present in the scratch repo. Returned nothing
   — actually resolved to the fallback *global* vault (confirmed via
   `chron list`, which showed unrelated global notes: decisions/runbooks
   about Chronicle/Pebbles itself). This confirmed no local vault existed
   yet for this repo.

2. `chron init`
   Per the skill's rule of thumb ("repo-specific decisions/tasks/specs
   belong in that repo's local `.chronicle/`... `chron init` there first if
   it doesn't exist yet"), scaffolded a local vault at
   `<repo>/.chronicle/notes`. This is required before `chron spec new` can
   target this repo instead of the global vault, since `specs/` only lives
   in per-repo vaults.

3. `chron spec list` (again, post-init)
   Confirmed the freshly-initialized local vault has no existing
   capabilities/specs — nothing to revise, this is a genuinely new spec.

4. `chron spec new oauth-login "OAuth login with Google and GitHub, refresh token rotation"`
   Created `specs/oauth-login/v1-oauth-login-with-google-and-github-refresh-token-rotation.md`
   with `status: proposed` frontmatter and an empty `## Requirements`
   heading. Chose the capability slug `oauth-login` — short, matches how
   the feature will be referred to in conversation and future
   `chron spec current/history oauth-login` calls, and covers both
   providers + rotation as one cohesive capability rather than splitting
   into `oauth-google`/`oauth-github` (the request treats it as one login
   system, not two independent features).

5. Edited the spec file directly (not a `chron` subcommand — there's no
   `chron spec edit`; per the format, the CLI scaffolds frontmatter/
   filename, and the requirements body is authored directly while
   `status: proposed`) to fill in the `## Requirements` section: scope,
   provider integration details (Google OIDC vs. GitHub plain OAuth2+PKCE),
   account model, token handling (rotation, reuse detection, hashed
   storage), session/logout, security requirements (state, PKCE, redirect
   allow-list), error handling, and explicit open questions deferred to
   implementation.

6. `chron spec current oauth-login`
   Verified the spec resolves as the current tip: `proposed
   oauth-login/v1-oauth-login-with-google-and-github-refresh-token-rotation`.

7. `chron lint`
   Ran after the write per the skill's guidance ("run `chron lint` after a
   batch of writes"). No findings — clean.

## Reasoning notes

- Did not call `chron spec implement` — the task is explicitly
  pre-implementation ("before any of us start writing code"), so the spec
  correctly stays `status: proposed`. Per the lifecycle, `chron spec
  implement` only runs once the work described is actually done.
- Did not create a `type: task` note for this — the skill is explicit that
  a `proposed` spec **is** the implementation plan; there's no separate
  task-tracking artifact for capability work that has a spec.
- `dangerouslyDisableSandbox` was required for `chron init` and `chron spec
  new`/edits because `chron` writes to `~/.config/chron/vaults.json` on
  init, which sits outside the default sandbox write allowlist.
