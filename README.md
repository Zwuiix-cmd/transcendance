*This activity has been created as part of the 42 curriculum by Zwuiix, clfontai, julhang and two additional contributors.*

# Uno Ranked

## Description

Uno Ranked is a secure multiplayer Uno platform. It provides account creation and login,
player profiles, ranked and unranked matchmaking, custom tables, a leaderboard, a protected
administration workspace, and an integrated 3D Uno client. The API owns the match state and
exposes server-authoritative start, state, draw and play endpoints for the multiplayer client.

The application is split into four runtime services:

- `apps/api/`: a Go REST API backed by SQLite.
- `apps/web/`: a React and TypeScript frontend served through Caddy.
- `apps/admin/`: an independent PHP administration workspace with a server-side API proxy.
- `apps/cdn/`: a Go file CDN/API for every user-accessible asset. Avatars are one
  namespace inside it; the service also supports future documents, media and game assets.

The API has an API-key middleware, per-client rate limiting, bearer sessions, BCrypt password
hashing, input validation, CORS restrictions, secure response headers, and a SQLite schema
with foreign keys and WAL mode. In the container deployment, Caddy terminates HTTPS and
adds the internal API key server-side, so the browser does not need to know the secret key.

## Current status

Implemented in this foundation:

- Secure registration, login, logout and session-based authentication.
- Profile read/update/delete endpoints, CDN-backed avatar upload and
  a responsive profile page. Players without an uploaded image receive a persisted generated avatar.
- Waiting/in-progress/finished game listing, ranked/unranked/custom mode separation, configurable
  custom tables (capacity, hand size, penalties, house rules and optional bots), invite codes,
  host kick controls, whitelist management and host launch into the integrated 3D client.
- Leaderboard endpoint and clickable public player profiles, plus a protected PHP admin workspace
  for platform stats, activity graphs, ELO/mode distributions, user management, ELO changes,
  role management and searchable site-wide audit logs.
- SQLite schema for users, statistics, games, custom game settings, participants, sessions and
  audit events.
- API-key validation, invalid-key throttling and general request validation.
- Go matchmaking queue with dynamic ranked ELO windows and unranked search. The queue is
  deliberately kept in the API process until a persistent game/match service is needed.
- Go game-result settlement with an internal service key, idempotent ranked ELO calculation,
  participant scores and atomic Uno statistics updates.
- Docker Compose deployment with an HTTPS Caddy entry point.

Known limitations:

- Complete production rules such as every penalty variant, scoring and final-match settlement
  still need to be completed and validated in the 3D game.
- Persistent matchmaking state is not implemented yet; the current queue is an in-memory Go
  service and will later hand tickets to the Uno game service.
- Friends and online presence UI.

Statistics and game-history modules will only be claimed after a complete game is available,
as required by the subject.

## Instructions

### Prerequisites

- Docker Engine 24+ and Docker Compose v2.
- Go 1.24+ for local API and CDN development.
- Node.js 20+ and npm 9+ for frontend development.
- PHP 8.1+ for the independent administration workspace.
- Python 3.11+ for repository scripts and the CDN avatar generator.

### Run the complete stack

```sh
make setup
# The Uno 3D assets are already stored in apps/cdn/assets/uno.
# Replace the API, CDN and game-service keys in .env with random secrets.
make run-d                              # setup + build if needed + background mode
# Or: make run                           # setup + build if needed + foreground mode
```

Open `https://localhost` (or `https://localhost:8443` when the default ports are already in use).
Caddy uses an internal development certificate, so the browser may
ask for a one-time trust exception. HTTP traffic on port 80 is redirected to HTTPS.
For a deployed hostname, set `SITE_ADDRESS` and `CORS_ALLOWED_ORIGINS` in `.env`, then use a
trusted certificate configuration instead of the local `tls internal` certificate.

Useful commands:

```sh
make build                              # build every local image and Go artifact
make test                               # run API, frontend, admin and CDN tests
make lint                               # run frontend and admin static checks
make secrets                            # print fresh API/CDN/game-service secrets without writing files
make audit                              # audit the frontend lockfile
make ps                                 # show container status
make logs SERVICE=api TAIL=100          # follow one service's logs
make admin-grant-email EMAIL=you@learner.42.tech # grant the persistent admin role
make down                               # stop and remove containers, keep database volumes
make debug                              # explicit local login bypass for Zwuiix
make fclean                             # remove containers, images and local dependencies
make reset-data CONFIRM=1               # remove containers and database volumes
```

`make fclean` does not delete the SQLite, file CDN or Caddy volumes. Use `make reset-data CONFIRM=1` only
when you intentionally want to remove persisted project data. `make debug` is opt-in: it enables
`Bearer debug` for the configured `DEBUG_AUTH_USERNAME` (default `Zwuiix`). Never enable this
mode outside local development.

### Run a service locally

```sh
make install
```

For local development, use `make run` or `make run-d` so the same Compose configuration is used
for the API, web, admin and CDN. The API is also bound to `127.0.0.1:8080` by default for backend
checks and still requires `X-API-Key`; keep that binding local on a development machine.

Never commit `.env`, database files or production secrets. Use a reverse proxy or a real TLS
certificate for any non-local deployment.

## API overview

All `/api/*` routes require `X-API-Key`. Authentication routes return an opaque bearer token;
protected requests also send `Authorization: Bearer <token>`.

| Method | Route | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Create an account and session |
| `POST` | `/api/v1/auth/login` | Authenticate an account |
| `POST` | `/api/v1/auth/logout` | Revoke the current session |
| `GET` | `/api/v1/auth/me` | Read the current profile and statistics |
| `GET` | `/api/v1/users/{id}` | Read a public profile |
| `PUT` | `/api/v1/users/me` | Update the current profile |
| `PUT` | `/api/v1/users/me/avatar` | Upload a PNG, JPEG or WebP avatar (5 MB max) |
| `DELETE` | `/api/v1/users/me` | Delete the current account |
| `GET` | `/api/v1/games` | List waiting and historical games |
| `GET` | `/api/v1/games/{id}` | Read lobby participants and host controls data |
| `POST` | `/api/v1/games` | Create a custom public or private waiting lobby (`mode: custom`) |
| `POST` | `/api/v1/games/join` | Join a private lobby with its code |
| `POST` | `/api/v1/games/{id}/join` | Join a lobby, optionally providing its code |
| `POST` | `/api/v1/games/{id}/leave` | Leave a lobby as a participant |
| `POST` | `/api/v1/games/{id}/uno/start` | Start the Uno game as the table owner |
| `GET` | `/api/v1/games/{id}/uno/state` | Read the server-authoritative Uno snapshot |
| `POST` | `/api/v1/games/{id}/uno/draw` | Draw a card during the current turn |
| `POST` | `/api/v1/games/{id}/uno/play` | Play a card during the current turn |
| `POST` | `/api/v1/games/{id}/result` | Settle a finished game from the internal game service |
| `DELETE` | `/api/v1/games/{id}/participants/{userId}` | Kick a participant as the host |
| `POST` | `/api/v1/games/{id}/whitelist` | Add a username to a private lobby whitelist |
| `DELETE` | `/api/v1/games/{id}/whitelist/{userId}` | Remove a whitelist entry as the host |
| `POST` | `/api/v1/games/{id}/code/rotate` | Rotate a private lobby code as the host |
| `PUT` | `/api/v1/games/{id}/status` | Update a lobby status |
| `DELETE` | `/api/v1/games/{id}` | Delete a lobby created by the current user |
| `POST` | `/api/v1/matchmaking/queue` | Join the ranked or unranked Go matchmaking queue |
| `GET` | `/api/v1/matchmaking/queue` | Read the current queue position or match ticket |
| `DELETE` | `/api/v1/matchmaking/queue` | Leave the current queue |
| `GET` | `/api/v1/leaderboard` | Return the ELO leaderboard |
| `GET` | `/api/v1/admin/overview` | Read platform statistics (admin only) |
| `GET` | `/api/v1/admin/dashboard` | Read dashboard KPIs, activity series, distributions and recent audit events (admin only) |
| `GET` | `/api/v1/admin/logs` | Search/filter site audit events (admin only) |
| `GET` | `/api/v1/admin/users` | List users for administration (admin only) |
| `GET` | `/api/v1/admin/users/{id}/analytics` | Read a user's private admin analytics (admin only) |
| `PUT` | `/api/v1/admin/users/{id}/elo` | Update a user's ELO (admin only) |
| `PUT` | `/api/v1/admin/users/{id}/role` | Set the persistent `player`, `staff`, `developer` or `admin` role (admin only) |
| `DELETE` | `/api/v1/admin/users/{id}` | Delete a user and linked data (admin only) |

Private lobbies always require their generated code. Once a whitelist contains at least one
account, only whitelisted accounts and the host may join. New accounts start at 1000 ELO.

Ranked and unranked are queue modes. Only a completed ranked game can affect ELO; unranked and
custom games never change the rating. Custom games do not enter the matchmaking queue. Their full
table configuration, including the optional `botsEnabled` flag and its `botDifficulty` (`easy`,
`intermediate` or `expert`, each with its own server-side playing style), is stored with the lobby and
consumed by the Uno engine. A custom-game owner can launch alone when bots are enabled; when bots
are disabled, the server requires at least two human participants before starting the game.

The engine plays the official Mattel rules by default and enforces every optional house rule stored
with the table: configurable `drawPenalty` for +2 cards, `stackDrawCards` (+2 on +2, +4 on anything,
the whole stack lands on whoever accepts), `jumpIn` (an identical card may be played out of turn),
`sevenZero` (a 7 lets the player pick a hand to swap with, a 0 rotates every hand in play direction),
`drawUntilPlayable`, `forcePlay` (the drawn card must be played) and `challengeDrawFour` (off means a
+4 resolves immediately). Bots follow the same options. A finished snapshot carries a ranked
`results` list with cards remaining, hand points and the winner's points earned.

The internal game service submits a result with `X-Game-Service-Key`, configured through
`GAME_SERVICE_KEY`. A result must contain every participant exactly once, with contiguous
positions starting at 1. The API settles the game once, updates participant scores and Uno
statistics in one SQLite transaction, and records ranked ELO history. Repeating the same request
is safe and returns the already-settled result.

The complete request/response notes are in [`docs/api.md`](docs/api.md).

## Team information

The project is developed by a five-person team. Responsibilities are documented below
according to the actual contributions to this project:

| Member | Role(s) | Responsibilities |
| --- | --- | --- |
| `Zwuiix` | Team Leader / Product Owner / Backend contributor | Project concept and product direction, team leadership, and contribution to the backend implementation |
| `clfontai` | REST API and CDN developer | REST API and CDN implementation, developed in collaboration with the team leader |
| `julhang` | 3D game developer | Complete 3D client, scene, table, camera and 3D gameplay presentation |
| Two additional team members | Developers | The remaining frontend, administration, integration and project modules |

The two additional contributors are intentionally grouped here because their individual
usernames were not recorded in this README. Their work covers the remaining parts of the
application.

## Project management

The work was divided by product area and service boundary. Zwuiix led the product direction and
worked on the backend, clfontai owned the REST API and CDN work with Zwuiix, julhang owned the
3D client, and the two remaining developers handled the other application areas. Git and GitHub
are used as the shared source-control and review history. The team coordinates implementation,
integration and peer review directly, and every member must be able to explain the code they
contributed during peer review.

## Technical stack

- Frontend: React, TypeScript, Vite, React Router and a small reusable CSS design system.
- Administration: PHP 8.3 server-rendered shell, vanilla JavaScript charts and a restricted
  server-side proxy to the authenticated API.
- API: Go 1.24, the standard-library `net/http` package, BCrypt, SQLite and an in-memory ELO queue.
- Realtime: a WebSocket endpoint (`/api/v1/ws`, `github.com/coder/websocket`) with an in-process
  hub that pushes lobby, table, matchmaking, leaderboard and profile changes; the web client does
  not poll and the socket doubles as the presence signal.
- Matchmaking: Go service with a synchronized ranked/unranked ELO queue behind the authenticated API.
- File delivery: Go 1.24 standard library with a Python avatar script, Go SHA-256/FNV helpers,
  format detection, image dimensions and SVG safety checks.
- Analytics: Go computes leaderboard ordering, activity summaries and chart geometry in the API;
  TypeScript only renders the returned data and handles pointer interaction.
- Project tooling: dependency-free Python scripts for environment, schema, secret and admin checks.
- Edge/deployment: Docker, Docker Compose and Caddy with HTTPS.
- Database: SQLite is appropriate for this project’s single durable data store and keeps local
  setup simple. Foreign keys, WAL mode, busy timeouts and explicit transactions protect
  concurrent requests. SQLite calls stay inside the API service while the HTTP layer remains
  concurrent and isolated from the frontend.

## Database schema

```text
users 1 ───── 1 user_stats
  │ ├──────< auth_sessions
  │ ├──────< games (created_by)
  │ ├──────< game_participants >──── games
  │ ├──────< game_spectators >────── games
  │ ├──────< game_whitelist >─────── games
  │ ├──────< user_elo_history >───── games
  │ └──────< audit_logs (actor)
  │
  └──── games 1 ───── 1 game_settings
             │
             └─────── 1 uno_game_states
```

`users` stores the account identity, BCrypt hash, CDN avatar URL, ELO, role and timestamps.
`user_stats` stores Uno statistics. `games` stores the lifecycle and mode, while
`game_settings` stores custom-table rules. `game_participants` and `game_spectators` are
composite-key membership tables. `uno_game_states` stores the versioned server-authoritative
game snapshot as JSON. `game_whitelist` stores private-table access, `auth_sessions` stores
revocable bearer sessions, `user_elo_history` stores rating events, and `audit_logs` stores
security and administration events. The canonical columns, SQLite types and foreign keys are
documented in [schema.sql](apps/api/db/schema.sql).

The canonical schema is [`apps/api/db/schema.sql`](apps/api/db/schema.sql).

## Features and ownership

| Feature | Contributors | What it provides |
| --- | --- | --- |
| Product direction and project leadership | Zwuiix | Project concept, priorities, integration decisions and team coordination |
| Authentication and user management | Zwuiix and the two additional developers | Registration, login, profiles, sessions, validation and account lifecycle |
| REST API and persistence | Zwuiix and clfontai | Go routes, SQLite schema, validation, matchmaking, game lifecycle and settlement |
| CDN and avatar delivery | Zwuiix and clfontai | Namespaced file storage, avatar processing, cache headers and static Uno asset delivery |
| Matchmaking and game tables | Zwuiix and the two additional developers | Ranked/unranked queues, custom tables, settings, invitations, spectators and realtime presence |
| 3D Uno client | julhang | Three.js scene, split model loading, cards, camera, table, seats and 3D presentation |
| Administration and analytics | The two additional developers | PHP administration workspace, dashboards, audit logs and player analytics |
| Server-authoritative Uno actions | Zwuiix and clfontai | Start, state, draw and play endpoints with turn and ownership checks |

## Modules and points plan

The subject requires at least 14 points. The following plan is intentionally conservative:

| Module | Type | Status / justification |
| --- | --- | --- |
| Frontend framework | Web minor — 1 pt | Implemented with React, TypeScript, Vite and React Router |
| Public API | Web major — 2 pts | API key, rate limiting, documentation and GET/POST/PUT/DELETE routes |
| Custom design system | Web minor — 1 pt | Reusable accessible components, shared tokens and Papicons |
| Standard user management | User Management major — 2 pts | Partial: profiles, avatars, authentication and roles exist; friends and online status remain incomplete |
| Complete web game | Gaming major — 2 pts | Not claimable until the Uno rules, win/loss conditions and live match are complete |
| Remote players | Gaming major — 2 pts | Not claimable until two remote clients can play the same live match reliably |
| Multiplayer 3+ | Gaming major — 2 pts | Not claimable until three or more human players are synchronized in one game |
| Game statistics/history | User Management minor — 1 pt | Not claimable until a complete playable game produces match history |

The currently validated foundation is 4 points. The remaining planned modules are listed for
transparency but must not be claimed until they are fully functional and demonstrable during
peer review. The final project must reach at least 14 validated points.

## Go responsibilities

The Go responsibility is a focused file CDN. Go is a good fit for a small, concurrent HTTP
asset service that can stream files without coupling storage to authentication or game rules.
The CDN uses namespaced keys, atomic uploads, path validation, size limits, cache-friendly URLs
and an internal API key. Avatars use `files/avatars/{userId}/avatar`, while the same contract is
ready for future user assets. Later iterations can add:

1. S3-compatible object storage behind the same namespaced API.
2. Signed, expiring URLs for private attachments.
3. Resumable uploads and image/video thumbnail workers.
4. Prometheus metrics for bytes served, cache hits and upload latency.
5. Antivirus/content validation hooks before a file becomes public.

This separation keeps profile pages independent from the card engine while allowing the web
client to load the project-owned Uno assets and communicate with the authoritative API.

## Resources and AI usage

- The official Transcendence project specification is provided privately to the team and is not
  redistributed in this repository.
- Go `net/http` reference: <https://pkg.go.dev/net/http>.
- SQLite documentation: <https://www.sqlite.org/docs.html>.
- React documentation: <https://react.dev/learn>.
- Go documentation: <https://go.dev/doc/>.

AI was used as an engineering assistant for API and CDN debugging, route and schema review,
Three.js asset integration, frontend iteration, performance checks and compatibility fixes.
Generated suggestions were reviewed, adapted and tested by the team; AI did not replace design
decisions, peer review, security review or responsibility for the final code. Every team member
must understand and be able to explain the changes they submit.

## Individual contributions

- **Zwuiix:** imagined the project, led the team, defined the product direction, contributed to
  the backend, and worked with clfontai on the REST API and CDN integration. Key challenges were
  keeping the browser, API, CDN and 3D client aligned while preserving server-side authority.
- **clfontai:** implemented the REST API and CDN with Zwuiix, including route contracts, file
  delivery and asset-serving integration.
- **julhang:** implemented the complete 3D client and its scene presentation, including the
  Three.js environment, split model loading, cards, table, seats and camera behavior.
- **The two additional team members:** implemented the remaining frontend, administration and
  integration work. Their individual usernames and exact feature lists must be added before
  peer review.
