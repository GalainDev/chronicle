---
type: spec
capability: oauth-login
area: work
status: proposed
supersedes: oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation
created: 2026-07-26T18:30:33Z
---

# OAuth login with Google, GitHub, Apple Sign-In, and refresh token rotation

Supersedes [[oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation]].

## Requirements

- Google and GitHub OAuth login, plus refresh token rotation — shipped in
  [[oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation]],
  live in prod. No further work needed here.
- Add Apple Sign-In as a third login provider, to the same standard as Google
  and GitHub (login, account linking, refresh token rotation applies to it too).
  New requirement from product, out of scope for v1.
