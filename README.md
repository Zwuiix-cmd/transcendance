*This activity has been created as part of the 42 curriculum by login1, login2, login3, login4.*

# Uno Ranked

## Description

Uno Ranked is a secure multiplayer Uno platform. The current milestone prepares the shared
foundation: account creation and login, player profiles, waiting-game views, a leaderboard,
and an API contract that the game implementation can use later. The 3D Uno client is now
integrated and can be launched from custom tables; the Uno rules and live game board remain
outside this milestone and will be implemented by another team member.

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

Not implemented yet:

- Uno deck, turn rules, penalties, scoring and the live game board.
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
make uno-assets                         # extract the local Uno 3D CDN assets
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
table configuration, including the optional `botsEnabled` flag, is stored with the lobby and
returned to the future Uno game service. A custom-game owner can launch an incomplete table; the
future game service decides how enabled bot seats behave.

The internal game service submits a result with `X-Game-Service-Key`, configured through
`GAME_SERVICE_KEY`. A result must contain every participant exactly once, with contiguous
positions starting at 1. The API settles the game once, updates participant scores and Uno
statistics in one SQLite transaction, and records ranked ELO history. Repeating the same request
is safe and returns the already-settled result.

The complete request/response notes are in [`docs/api.md`](docs/api.md).

## Team information

Replace the placeholder logins above before the peer review. The proposed roles are:

| Member | Role(s) | Responsibilities |
| --- | --- | --- |
| `login1` | Product Owner / Developer | Product scope, backlog and API contracts |
| `login2` | Project Manager / Developer | Planning, integration and delivery tracking |
| `login3` | Technical Lead / Developer | Architecture, security and database |
| `login4` | Developer | Uno game engine, real-time play and game UX |

The exact assignment and individual contribution section must be updated by the team as work
is merged. No contribution should be attributed automatically.

## Project management

The team should track work with GitHub Issues and a small Kanban board, review important pull
requests before merging, and hold a weekly planning/check-in meeting. Day-to-day coordination
can happen in the team Discord or Slack channel. Every member must create meaningful commits
and be able to explain the code they contributed during peer review.

## Technical stack

- Frontend: React, TypeScript, Vite, React Router and a small reusable CSS design system.
- Administration: PHP 8.3 server-rendered shell, vanilla JavaScript charts and a restricted
  server-side proxy to the authenticated API.
- API: Go 1.24, the standard-library `net/http` package, BCrypt, SQLite and an in-memory ELO queue.
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
  │             
  ├──────< auth_sessions
  │
  └──────< games (created_by)
                 │
                 ├────< game_participants >──── users
                 └────< game_whitelist >─────── users
```

`users` stores the account identity, BCrypt hash, CDN avatar URL, ELO, persistent `player`/`staff`/`developer`/`admin` role and timestamps. `user_stats` stores Uno
statistics without coupling them to authentication. `games` stores the lobby/history lifecycle,
and `game_participants` is the composite-key link table for game scores and signed ELO deltas. Private lobby access
codes and host-managed restrictions live in `game_whitelist`; `auth_sessions` is an additional
security table used to revoke opaque login tokens.

The canonical schema is [`apps/api/db/schema.sql`](apps/api/db/schema.sql).

## Features and ownership

The foundation is deliberately split into reviewable tasks:

- API security and persistence: Technical Lead / API developer.
- Authentication and profile screens: Frontend/API developers.
- Games and leaderboard views: Frontend/API developers.
- File CDN and asset delivery: Go developer.
- Uno rules, live synchronization and game UX: game developer (next milestone).

Update this section with the real Git authors and pull requests when the team has merged the
work.

## Modules and points plan

The subject requires at least 14 points. The following plan is intentionally conservative:

| Module | Type | Status / justification |
| --- | --- | --- |
| Frontend framework | Web minor | React + TypeScript frontend |
| Public API | Web major | API key, rate limiting, documentation, and GET/POST/PUT/DELETE routes |
| Custom design system | Web minor | Reusable accessible components and shared tokens |
| Standard user management | User Management major | In progress: profile, CDN avatar storage, password hashing and sessions are ready; friends and online presence remain to be implemented |
| Complete web game | Gaming major | Planned; cannot be claimed before Uno is playable |
| Remote players | Gaming major | Planned after the real-time game protocol |
| Multiplayer 3+ | Gaming major | Planned after the game exists |
| Game statistics/history | User Management minor | Planned; dependency is the completed game |

The project currently has a working foundation, not 14 validated points. The team must only
claim modules that are fully demonstrable during peer review.

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

This separation lets the teammate implement Uno behind the Go API's stable match ticket without
making the profile pages depend on the card engine or the file storage implementation.

## Resources and AI usage

- The official Transcendence project specification is provided privately to the team and is not
  redistributed in this repository.
- Go `net/http` reference: <https://pkg.go.dev/net/http>.
- SQLite documentation: <https://www.sqlite.org/docs.html>.
- React documentation: <https://react.dev/learn>.
- Go documentation: <https://go.dev/doc/>.

AI was used as a design and implementation assistant for the initial folder structure, API
contract, security checklist, schema review, UI scaffolding and test planning. Team members must
read, run, review and be able to explain every generated change. AI output is not a substitute for
peer review, security review or the final contribution record.

## Individual contributions

This section must be completed as the project evolves. For every member, record the concrete
features/modules they implemented, the decisions they made, and the technical challenges they
solved. Keep it aligned with the Git history rather than using generic descriptions.
