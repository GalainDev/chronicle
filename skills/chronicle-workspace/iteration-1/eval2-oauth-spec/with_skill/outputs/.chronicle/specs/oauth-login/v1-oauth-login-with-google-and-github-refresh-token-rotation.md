---
type: spec
capability: oauth-login
status: proposed
created: 2026-07-26T18:30:14Z
---

# OAuth login with Google and GitHub, refresh token rotation

## Requirements

### Scope

- Support user login via OAuth 2.0 / OIDC with two providers: Google and
  GitHub, selectable from the login screen.
- Support refresh token rotation for both providers so sessions can be
  renewed without forcing re-authentication, and so a stolen refresh token
  is only usable once.
- Out of scope for this version: additional providers (e.g. Microsoft,
  Apple), passwordless/magic-link login, SSO/SAML, account linking across
  providers for a single user (a user who signs in with both Google and
  GitHub gets two distinct accounts unless we later revise this spec).

### Provider integration

- Google: OAuth 2.0 with OpenID Connect. Request `openid email profile`
  scopes at minimum. Verify the ID token signature and `aud`/`iss` claims
  server-side before trusting any identity data.
- GitHub: OAuth 2.0 (GitHub does not speak OIDC). Request `read:user
  user:email` scopes. Fetch the primary verified email via the GitHub API
  after token exchange, since GitHub's email may be private/unset on the
  profile.
- Both providers use the Authorization Code flow with PKCE, not the
  implicit flow.
- Client secrets and API tokens are stored server-side only, never exposed
  to the browser.

### Account model

- A local user account is created on first successful login for a given
  provider + provider-user-id pair. Email is stored for display/contact but
  is not used as the account-matching key (providers don't guarantee a
  verified, stable email).
- Store `(provider, provider_user_id)` uniquely per account row.

### Token handling

- Access tokens (ours, issued to the client) are short-lived (target: 15
  minutes).
- Refresh tokens are long-lived (target: 30 days) and are rotated on every
  use: each refresh call issues a new refresh token and immediately
  invalidates the one presented.
- Reuse of an already-invalidated refresh token is treated as a signal of
  possible token theft: the entire token family (all descendants of that
  refresh token) is revoked, and the user is forced to re-authenticate.
- Refresh tokens are stored hashed at rest (never in plaintext), following
  the same standard as password storage would.
- Provider-issued OAuth tokens (Google/GitHub access/refresh tokens) are
  stored encrypted at rest and are only used server-side to call the
  provider's API (e.g. re-fetch profile data) — they are never returned to
  the client.

### Session and logout

- Successful login issues our own access + refresh token pair; the
  provider's tokens are not used as the app's session credential.
- Logout revokes the current refresh token (and, where feasible, the
  provider-side token/session) rather than only clearing the client-side
  cookie/storage.
- A user can view and revoke active sessions (i.e. active refresh token
  families) from account settings.

### Security requirements

- `state` parameter is used on every OAuth authorization request and
  validated on callback to prevent CSRF.
- PKCE `code_verifier`/`code_challenge` used for both providers.
- Redirect URIs are allow-listed exactly (no wildcard/path-prefix matches)
  in both our app config and each provider's app console.
- All OAuth callback and token endpoints are HTTPS-only.
- Rate-limit the token refresh endpoint per account to blunt refresh-token
  brute-force/replay attempts.

### Error handling

- Provider denial (user cancels consent) redirects back to login with a
  clear, non-technical message; no partial account is created.
- Provider outage or token-exchange failure surfaces a retryable error
  distinct from "invalid credentials."
- Expired/invalid refresh token returns a 401 that the client maps to
  "please log in again," clearing any local session state.

### Open questions (to resolve during implementation, not blocking spec approval)

- Exact access/refresh token lifetimes (15 min / 30 days above are starting
  targets, not fixed).
- Whether to support account linking (same human, multiple providers) in a
  later revision.
- Whether refresh tokens are stored as httpOnly cookies vs. returned to a
  native/mobile client for local secure storage — likely both, since this
  spec doesn't scope client platforms.

