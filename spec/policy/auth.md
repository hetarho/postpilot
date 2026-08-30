# Policy — Accounts, passwords, sessions

Canonical rules that are **currently true** in the code. Source: [plan/01](../plan/01.accounts-and-login.md),
built by jobs 01 and 02. A change to any rule here is a change to shipped behavior — go through `/create-change`.

## Accounts

- **There is no registration surface.** No `Signup` RPC, no signup screen, no invite flow. The API exposes exactly
  three auth procedures: `Login`, `Logout`, `GetMe` (PRD F-1).
- Accounts are created **only** by the operator, from a shell on the box:
  `api adduser <login_id>` (the deployed image dispatches the subcommand through its unchanged
  `ENTRYPOINT ["/api"]`) or `go run ./cmd/adduser <login_id>` on a host. Both read the password twice from stdin,
  require the two to match, and refuse an existing id with a non-zero exit.
- A duplicate login id is rejected at the database (primary key) and reported as `auth.ErrDuplicateUser`.
- There is **no password change or reset flow.** The operator recreates the account. This is a known PRD gap, not an
  oversight — see plan 01 Non-goals.

## Passwords

- Stored as **argon2id in PHC string form** only: `$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>`. Plaintext and
  unsalted hashes are forbidden.
- Parameters: `time=3`, `memory=64 MiB`, `parallelism=2`, 16-byte random salt, 32-byte key. They are encoded in
  every hash, so raising them later affects only newly written hashes — no schema change, no forced reset.
- Verification is constant-time. A stored hash that will not parse (including out-of-range cost parameters, which
  would otherwise panic inside `argon2.IDKey`) is an error that is logged and then answered as an ordinary wrong
  password.

## Login and what a failure may reveal

- **Every login failure is byte-identical**: `connect.CodeUnauthenticated` with the fixed message
  `invalid credentials`. Unknown id, wrong password, and an unusable stored hash are indistinguishable.
- **Unknown ids run a real argon2id verification** against a dummy hash whose password nobody knows, so the
  "no such account" path costs the same as "wrong password" and id existence does not leak by timing. The dummy hash
  is derived at boot (not on first use) and always carries the *current* cost parameters — a hardcoded one would
  drift cheaper the day the parameters are raised and reopen the leak.
- There is no rate limiting and no account lockout. At two users, timing equalization is the defense (plan 01
  Non-goals).
- An infrastructure failure during login (store unavailable) is **not** reported as `invalid credentials`; it is
  `CodeInternal`, so an outage is not mistaken for a typo.

## Sessions

- Token: 32 bytes from `crypto/rand`, base64url, sent only in the cookie. The database stores
  `hex(sha256(raw_token))` — a leaked database yields no usable cookies. A plain sha256 (not a KDF) is correct here:
  the input is 256 bits of uniform randomness, so there is nothing to brute force.
- **Fixed 30-day lifetime**, not sliding: `expires_at = login_time + 720h`. The cookie's `Max-Age` is derived from
  the same configured duration, so the cookie and the row can never disagree.
- The session cookie is exactly:

  ```
  Set-Cookie: pp_session=<token>; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax
  ```

  **No `Domain` attribute** — host-only, so sibling projects on the same registered domain never receive it.
- `Logout` **deletes the session row** (server-side revocation), then clears the cookie with the same attributes and
  `Max-Age=0`. Clearing the cookie alone would leave a stolen copy valid for the rest of its 30 days.
- Expired rows are deleted on the lookup that finds them, and swept once at boot. A sweep failure is logged, not
  fatal — a stale row still fails the expiry check.
- Timestamps are stored as **fixed-width RFC3339 in UTC**. The width matters: `expires_at < ?` is a plain string
  comparison in SQL, and a trimmed fraction (`…08.5Z`) sorts after a longer one (`…08.513110616Z`).

## Authorization on every RPC

- A single Connect interceptor (`internal/auth/rpc`) is the **only** place a request becomes an acting user. It
  resolves the cookie, checks expiry, and puts the user id in the `context`.
- **Authenticated handlers take the acting user from the context, never from a request payload.** A user id in a
  message is a claim by the caller, not a fact.
- The interceptor **fails closed**: missing, forged, expired, and "the store is down" all return
  `CodeUnauthenticated` (HTTP 401) with the message `unauthenticated`. A new service is protected the moment it is
  mounted, and it covers streaming handlers as well as unary ones.
- Exactly three procedures are public, and the list lives in one place
  (`publicProcedures` in `internal/auth/rpc/interceptor.go`):

  | Procedure | Why |
  |---|---|
  | `AuthService/Login` | it is how you get a session |
  | `AuthService/Logout` | logout must always clear the cookie; the cookie is HttpOnly, so a 401 would strand it in the browser for its full 30 days |
  | `HealthService/Ping` | the wire test the frontend runs before any account exists |

  The plain `/health` endpoint is mounted outside the Connect stack and is likewise unauthenticated — it is what the
  deploy's rollback gate probes.

## Paired publishing-agent credentials

- Publishing-agent bearer tokens are device capabilities, not human sessions. Agent procedures are explicitly
  bypassed by the cookie interceptor and then fail closed in their own interceptor; every human publishing procedure
  still requires the HttpOnly session. A cookie cannot claim/advance a job, and a bearer token cannot list/start/
  cancel a user's publication.
- The raw device token is returned once at pairing, stored only in macOS Keychain, and represented on the server only
  by SHA-256 of 32 random bytes. Each authenticated request rechecks revocation and updates safe `last_seen_at`
  metadata. Naver credentials, cookies, profile paths, and CDP endpoints are not postpilot credentials and never
  cross this boundary. Full lifecycle rules are in [publishing.md](publishing.md).

## The browser side

- **The token never reaches JavaScript.** `document.cookie` cannot see it; the browser attaches it as a `Cookie`
  header. No `Authorization` header is used anywhere.
- The SPA and the API are different origins in production, so every RPC opts into sending cookies. connect-web has
  no `credentials` option — the opt-in lives in a `fetch` wrapper the transport is built with.
- **A 401 on any procedure except `Login` means the session is gone**, and the app returns to `/login`. Login is
  exempt because its 401 means the password was wrong, which the form reports itself.
- Every protected screen is a child of one pathless guard route, so a new screen is protected by being added, not by
  remembering to protect it. The guard re-checks a cached session after `SESSION_STALE_MS` (30 s), so a session
  revoked elsewhere stops granting access without a full reload.
- **An outage is not a logout.** A failure that is not a 401 reaches the error boundary rather than sending the user
  to a login form that cannot work. The one exception is `/login` itself, which renders regardless — it is what a
  user reaches for when the app is misbehaving.
- **A post-login `redirect` is followed only if it stays in the app.** It is validated by resolving it against an
  origin and refusing anything that escapes — `//host` and `/\host` both leave, and the router does not check `to`
  against the known routes.
- **Logout that fails does not pretend to succeed.** The cookie is still valid, so the user stays signed in and is
  told; a "logged out" screen would offer safety the live HttpOnly cookie does not.
- The login form shows one message for every failure — `아이디 또는 비밀번호가 맞지 않아요` — mirroring the server's
  refusal to distinguish an unknown id from a wrong password.

## Transport

- CORS allows **exactly one origin** (`CORS_ORIGIN`) with `AllowCredentials: true`, which is what lets the browser
  send the HttpOnly cookie cross-origin.
- `CORS_ORIGIN` is validated at boot and the process refuses to start on a bad value. Any `*` is rejected — not just
  a bare wildcard: `rs/cors` reads an embedded `*` as a *pattern* and would reflect any matching subdomain, handing
  a sibling project the ability to make authenticated calls with the user's session.
- The session token appears in **no** proto message, response body, log line, or URL. It travels only in
  `Set-Cookie` / `Cookie`.
