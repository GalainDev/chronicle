# OAuth Login Spec — Google & GitHub with Refresh Token Rotation

Status: Draft for review
Owner: TBD
Last updated: 2026-07-27

## 1. Goal

Add "Sign in with Google" and "Sign in with GitHub" to the app, backed by our
own session system (not a delegated auth provider like Auth0/Clerk). Access
tokens are short-lived JWTs; refresh tokens are long-lived, stored server-side,
and rotated on every use.

## 2. Non-Goals

- Username/password login (out of scope; may coexist but isn't part of this spec).
- Additional OAuth providers beyond Google/GitHub (design should make adding one
  easy, but we're not building a generic "N providers" abstraction up front).
- Mobile app deep-linking / native OAuth flows (web only for v1).
- SSO/SAML for enterprise customers.

## 3. User-Facing Flow

1. User clicks "Continue with Google" or "Continue with GitHub" on the login page.
2. Redirect to the provider's consent screen (Authorization Code flow).
3. Provider redirects back to our callback URL with a `code` and `state`.
4. Server exchanges the code for the provider's tokens, fetches the user's
   profile (email, name, avatar, provider account id).
5. Server finds-or-creates a local user record, links the provider identity,
   and creates a session: sets an httpOnly access-token cookie and issues a
   refresh token.
6. User lands back in the app, authenticated.

Edge cases to handle explicitly:
- Email from provider already exists on an account created via a different
  provider (or a future password login) → link accounts by verified email,
  don't silently create a duplicate user. Require the email to be
  provider-verified before auto-linking; otherwise show an "account exists,
  confirm to link" step.
- User denies consent → redirect back to login with a friendly error, no crash.
- `state` mismatch / missing → reject with 400, log as a potential CSRF attempt.
- Provider returns no email (can happen with GitHub if the user's email is
  private) → fall back to GitHub's `/user/emails` endpoint to get the primary
  verified email; if still none, block sign-in with a clear message rather than
  creating an unusable account.

## 4. Architecture

### 4.1 Protocol

- **Authorization Code flow** for both providers (never implicit flow).
- PKCE is used even though we're a confidential client (server has a client
  secret), as defense-in-depth and to future-proof for a public-client/mobile
  flow later.
- `state` parameter: random 32-byte value, stored server-side (short-lived,
  keyed to the pending login attempt) or as a signed, encrypted cookie;
  validated on callback to prevent CSRF.
- Scopes requested:
  - Google: `openid email profile`
  - GitHub: `read:user user:email`

### 4.2 Data model

```
users
  id                uuid pk
  email             text unique not null
  email_verified    boolean not null default false
  name              text
  avatar_url        text
  created_at        timestamptz
  updated_at        timestamptz

oauth_identities
  id                  uuid pk
  user_id             uuid fk -> users.id
  provider            text not null        -- 'google' | 'github'
  provider_account_id text not null        -- stable id from provider, not email
  access_token_enc    bytea                -- provider's token, encrypted at rest; optional, only if we need to call their API later
  refresh_token_enc   bytea                -- provider's refresh token, encrypted at rest, if provided
  scopes              text[]
  created_at          timestamptz
  updated_at          timestamptz
  unique (provider, provider_account_id)

refresh_tokens
  id                uuid pk
  user_id           uuid fk -> users.id
  token_hash        text unique not null   -- sha256 of the token; raw token never stored
  family_id         uuid not null          -- groups a chain of rotated tokens
  parent_id         uuid null fk -> refresh_tokens.id
  issued_at         timestamptz
  expires_at        timestamptz
  revoked_at        timestamptz null
  revoked_reason    text null              -- 'rotated' | 'reuse_detected' | 'logout' | 'admin'
  replaced_by_id    uuid null fk -> refresh_tokens.id
  user_agent        text
  ip                inet
```

Notes:
- `oauth_identities` is separate from `users` so one user can have both Google
  and GitHub linked, and so we're not overloading the users table with
  provider-specific columns.
- We store our own `refresh_tokens`, distinct from the provider's OAuth
  refresh token. These serve different purposes: the provider's token (if any)
  lets us call their API later; ours is what keeps the user logged into *our*
  app.

### 4.3 Session tokens (our own, post-login)

- **Access token**: short-lived JWT (10–15 min), httpOnly + Secure + SameSite=Lax
  cookie. Contains `sub` (user id), `iat`, `exp`. Not stored server-side —
  validated by signature only.
- **Refresh token**: opaque random value (256-bit), httpOnly + Secure +
  SameSite=Strict cookie, path-scoped to the refresh endpoint only. Server
  stores only its hash (`token_hash`). Lifetime: 30 days sliding, absolute cap
  90 days.

### 4.4 Refresh token rotation

On every use of a refresh token:
1. Look up by `token_hash`. If not found → reject (401).
2. If `revoked_at` is set → this token was already used or revoked.
   **Reuse detected**: revoke the entire `family_id` chain immediately, and
   force the user to re-authenticate. Log this as a security event (this
   pattern indicates token theft — an attacker used a token the legitimate
   user already rotated past, or vice versa).
3. If valid and unused: mark it `revoked_at = now()`, `revoked_reason =
   'rotated'`, issue a brand-new refresh token with the same `family_id`,
   `parent_id` = old token's id, set old token's `replaced_by_id` = new
   token's id. Issue a new access token too. Return both to the client via
   cookies.
4. If `expires_at` has passed → reject (401), require full re-login.

This gives us rotation + reuse detection (the standard mitigation against
stolen refresh tokens) without needing an external session store — it's just
rows in `refresh_tokens`.

### 4.5 Logout / revocation

- Logout revokes the current refresh token (`revoked_reason = 'logout'`) and
  clears cookies. Does not revoke the whole family (other devices stay logged
  in).
- "Log out everywhere" revokes all non-revoked tokens for the user's `family_id`
  values (i.e., all sessions).
- Admin/account deletion path revokes all tokens and deletes `oauth_identities`.

## 5. API Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/auth/google/start` | Redirect to Google consent screen, set `state` |
| GET | `/auth/google/callback` | Exchange code, create/link user, start session |
| GET | `/auth/github/start` | Redirect to GitHub consent screen, set `state` |
| GET | `/auth/github/callback` | Exchange code, create/link user, start session |
| POST | `/auth/refresh` | Rotate refresh token, issue new access token |
| POST | `/auth/logout` | Revoke current refresh token, clear cookies |
| POST | `/auth/logout-all` | Revoke all refresh tokens for the user |

## 6. Security Requirements

- All provider tokens and our refresh tokens are compared/looked-up by hash;
  raw values never touch the database or logs.
- `state` must be validated with constant-time comparison; expire after 10 min.
- Callback endpoints must validate the `redirect_uri` matches exactly what was
  registered with the provider (no open redirect).
- Cookies: `httpOnly`, `Secure`, appropriate `SameSite`, scoped `path` for the
  refresh cookie so it's only sent to `/auth/refresh` and `/auth/logout*`.
- Rate-limit `/auth/*` endpoints (per IP and per account) to blunt
  brute-force/enumeration.
- Encrypt provider access/refresh tokens at rest if we store them (only store
  them at all if a real feature needs to call the provider API later — don't
  store "just in case").
- CSRF: state param covers the OAuth redirect; refresh/logout endpoints should
  also check `SameSite` cookie behavior is sufficient, or add a CSRF token if
  we ever call them from a cross-site context.
- Log security-relevant events (reuse detection, failed state validation,
  repeated failed callbacks) to a place that alerts, not just app logs.

## 7. Configuration

Per `.env.schema` conventions (no values committed):

```
GOOGLE_OAUTH_CLIENT_ID=
GOOGLE_OAUTH_CLIENT_SECRET=
GOOGLE_OAUTH_REDIRECT_URI=

GITHUB_OAUTH_CLIENT_ID=
GITHUB_OAUTH_CLIENT_SECRET=
GITHUB_OAUTH_REDIRECT_URI=

SESSION_JWT_SIGNING_KEY=
REFRESH_TOKEN_ENCRYPTION_KEY=
```

## 8. Open Questions (need decisions before/while building)

1. Do we need to call Google/GitHub APIs on behalf of the user after login
   (e.g., GitHub repo access)? This determines whether we store provider
   tokens at all, and what scopes to request.
2. What's the account-linking UX when the email matches an existing account —
   auto-link silently, or require an explicit "confirm to link" step? (Spec
   above assumes the latter for unverified-email safety, but product should
   confirm.)
3. Refresh token lifetime (30-day sliding / 90-day cap) — does this match our
   desired "remember me" duration, or do we want a shorter session for a
   security-sensitive part of the app?
4. Where do refresh tokens live — cookie only, or do we also need a
   header-based flow for a future mobile/SPA-on-different-origin client?
5. Do we need per-device session listing/management UI ("log out this
   device") in v1, or just "log out everywhere"?

## 9. Testing Plan

- Unit: token rotation logic (valid use, reuse detection, expiry), state
  validation, account-linking decision logic.
- Integration: full callback flow against mocked provider responses (success,
  denied consent, missing email, provider error).
- Security: attempt refresh-token replay after rotation (expect family
  revocation), attempt CSRF via forged callback without valid `state`, attempt
  cookie theft scenario (confirm httpOnly prevents JS access).
- Manual/E2E: real Google and GitHub sandbox apps, full login → refresh →
  logout cycle in a browser.

## 10. Rollout

1. Build behind a feature flag; internal team dogfoods with real Google/GitHub
   OAuth apps registered for staging.
2. Verify refresh rotation and reuse detection work correctly under normal use
   (no false-positive lockouts from race conditions — e.g., two tabs
   refreshing near-simultaneously should be handled, see note below).
3. Enable for all users; monitor `/auth/*` error rates and reuse-detection
   triggers for a week before considering it stable.

**Race condition note**: two concurrent requests using the same refresh token
(e.g., two tabs refreshing at once) will race — one wins the rotation, the
other hits "already revoked" and would trigger reuse detection incorrectly.
Mitigation: allow a short grace window (a few seconds) where a *just-rotated*
token still succeeds and returns the same new token/session instead of
triggering a security incident, distinguishing "used a token that's already a
few seconds stale" from "used a token that's long dead." This needs care — see
Open Question territory if the team wants a different approach (e.g.,
mutex/lock per family on refresh).
