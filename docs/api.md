# API contract

The API is versioned under `/api/v1`. Every request must include `X-API-Key`. In production the
Caddy edge proxy adds that header so the browser only handles a user bearer token.

## Authentication

Registration and login return an opaque token. Store it only in the browser session storage or a
secure application-managed store; do not put it in a URL. Send it as:

```http
Authorization: Bearer <access-token>
```

Passwords are validated at the edge and hashed with BCrypt before persistence. The API never
returns `password_hash`.

Account emails must be valid 42 addresses ending exactly with `@learner.42.tech`. This rule is
enforced for registration, login and profile updates.

Profiles accept a public bio; avatars are owned by Uno Ranked's internal Go CDN. If a profile has
no uploaded image, the CDN creates and persists a generated avatar on first use. The account
remains tied to the validated 42 email.

`PUT /api/v1/users/me/avatar` accepts a multipart field named `avatar`. PNG, JPEG and WebP are
accepted up to 5 MB. The Go API streams the file to the private Go CDN and stores the returned
versioned URL in `users.avatar_url`; the internal CDN key is never sent to the browser.

`GET /api/v1/users/search?q=<username>&limit=12` searches public usernames so the web
client can open a precise player profile without loading every account into the browser.

## Error shape

Errors are returned as JSON:

```json
{
  "code": "invalid_request",
  "message": "A human-readable explanation",
  "timestamp": "2026-10-24T20:00:00Z"
}
```

Useful status codes are `400` for invalid input, `401` for an invalid API key or session, `404`
for a missing resource, `409` for a unique-field conflict, and `429` for a rate-limit response.

## Game result boundary

The API owns accounts, lobbies, profiles and reporting data. The Uno service owns deck state and
turn validation. After a match, it sends a result to
`POST /api/v1/games/{id}/result` with both the public API key and the internal `X-Game-Service-Key`:

```json
{
  "participants": [
    {"userId": "winner-id", "position": 1, "score": 0, "unosCalled": 1, "cardsDrawn": 4},
    {"userId": "loser-id", "position": 2, "score": 86, "unoPenalties": 1, "cardsDrawn": 12}
  ]
}
```

The payload must contain every participant exactly once, with contiguous positions starting at 1.
The API updates `games`, `game_participants` and `user_stats` in one transaction. Ranked games
also update the players' ELO and append immutable `user_elo_history` entries; unranked and custom
games always store an ELO delta of zero. A retry after completion is idempotent. The current
`PUT /games/{id}/status` endpoint can move a lobby to `in_progress`, but cannot bypass settlement
by marking a game directly as `finished`.

## Private lobbies

`POST /api/v1/games` creates a custom lobby. In addition to `turnTimeLimit`, `privateGame` and
`mode: "custom"`, it accepts `maxPlayers` (2–10), `startingHandSize` (3–14), `drawPenalty` (1–4)
and house-rule flags: `stackDrawCards`, `jumpIn`, `sevenZero`, `drawUntilPlayable`, `forcePlay`,
`challengeDrawFour` and `allowMultipleDecks`. The response returns the persisted `settings` object
so the future Uno service can enforce the exact table configuration. The API rejects joins after
`maxPlayers` is reached. A private lobby contains a six-character `joinCode`; the host can add a
whitelist, kick participants, rotate the code or delete the lobby.

New accounts start with 1000 ELO.

## Matchmaking modes

`POST /api/v1/matchmaking/queue` accepts `{"mode":"ranked"}` or `{"mode":"unranked"}`. The
Go API authenticates the caller, reads their current ELO and owns the in-memory queue directly.
Ranked searches use a gradually widening ELO window; unranked searches do not compare rating.
The response reports queue position, `searchEloMin`, `searchEloMax` and `searchEloGap` for ranked
searches. The window starts at ±200 ELO, widens by 75 every five seconds and caps at ±800 ELO.
Unranked responses expose no ELO window. When available, the response also includes a match ticket
for the future Uno game service. `DELETE /api/v1/matchmaking/queue` cancels the search. Custom
lobbies do not enter this queue and never affect ELO.

## Administration

Admin routes require a session whose user has the admin role. Configure bootstrap admins with
`ADMIN_USERNAMES` and/or `ADMIN_EMAILS` in `.env`, then run `make run-d`. The admin panel exposes
platform counts, activity graphs, ELO and mode distributions, user search, ELO editing, admin
promotion or demotion, account deletion, and a searchable audit stream. The audit stream records
API requests, authentication/security failures and privileged admin actions without storing request
bodies, passwords or API keys. An admin cannot demote or delete their own account.

Administrative routes are guarded centrally by the API as well as by the controller methods. They
require a valid bearer session with the `admin` role and have a separate per-client rate limit
(`ADMIN_REQUESTS_PER_MINUTE`, default `60`). The PHP admin proxy uses a separate HttpOnly admin
cookie, checks same-origin mutating requests and never exposes the API key or session token to its
JavaScript client.
