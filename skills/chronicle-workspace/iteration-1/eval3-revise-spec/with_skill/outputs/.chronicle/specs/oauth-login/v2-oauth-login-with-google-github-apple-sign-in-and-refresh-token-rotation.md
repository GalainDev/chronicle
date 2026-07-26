---
type: spec
capability: oauth-login
status: proposed
supersedes: oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation
created: 2026-07-26T18:30:36Z
---

# OAuth login with Google, GitHub, Apple Sign-In, and refresh token rotation

Supersedes [[oauth-login/v1-oauth-login-with-google-github-and-refresh-token-rotation]].

## What changed and why

Refresh-token-rotation shipped to prod and is fully working — carried
forward unchanged below. Partway through implementation, product decided
Apple Sign-In is now in scope (it was not part of v1); this version adds
it as a new provider alongside Google and GitHub.

## Requirements

- Users can log in via OAuth using Google, GitHub, or Apple Sign-In as the
  identity provider.
- Apple Sign-In follows Apple's "Sign in with Apple" REST API flow
  (authorization code + identity token verification via Apple's public
  keys), consistent with how Google/GitHub are integrated.
- On successful OAuth login (any provider), the backend issues a short-lived
  access token and a refresh token to the client.
- Refresh tokens rotate on every use: each refresh exchanges the presented
  refresh token for a new access token and a new refresh token, and
  invalidates the previously issued refresh token.
- Reuse of an already-rotated (invalidated) refresh token is detected and
  treated as a compromise signal — the associated session/token family is
  revoked.
- Account linking: if a user authenticates with a new provider using an
  email address that matches an existing account, the login is linked to
  that existing account rather than creating a duplicate.
- Provider-specific profile data (name, email, avatar) is normalized into a
  common user profile shape regardless of which provider was used.
