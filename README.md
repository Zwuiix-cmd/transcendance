*This activity has been created as part of the 42 curriculum by ecochin, clfontai, julhuang, mschyns, halexand.*

# Uno Ranked

A secure, server-authoritative multiplayer Uno platform with a 3D client, ranked matchmaking,
AI opponents and a real-time administration workspace.

## Table of contents

1. [Description](#description)
2. [Instructions](#instructions)
3. [Usage](#usage)
4. [Team information](#team-information)
5. [Project management](#project-management)
6. [Technical stack](#technical-stack)
7. [Database schema](#database-schema)
8. [Features list](#features-list)
9. [Modules](#modules)
10. [Individual contributions](#individual-contributions)
11. [Known limitations](#known-limitations)
12. [Resources](#resources)
13. [Credits and license](#credits-and-license)

## Description

**Uno Ranked** is the 42 Transcendence project of a five-person team. The goal is a complete,
secure web application in which players create an account, meet in ranked, unranked or custom
tables and play Uno against each other or against AI opponents, in a 3D scene rendered in the
browser. Every rule is enforced by the server: the browser only displays the state it receives
and sends intentions (draw, play, call Uno, challenge, swap).

### Key features

- **Accounts and profiles**: registration, login, revocable sessions, avatars served by a
  dedicated CDN, public profiles, password change and account deletion.
- **Complete Uno engine**: the official Mattel rules plus seven optional house rules
  (configurable draw penalty, stacking of +2/+4, jump-in, seven-zero, draw until playable,
  force play, +4 challenge), end-of-round scoring and ranked results.
- **AI opponents**: three bot levels (easy, intermediate, expert) that respect the table rules
  and take their turns on the server.
- **Matchmaking**: ranked queue with a widening ELO window, unranked queue, and automatic
  game creation when a match is found.
- **Custom tables**: public or private lobbies with invite codes, whitelist, capacity, hand
  size, house rules and bots.
- **Real time**: a WebSocket hub pushes lobby, table, matchmaking, leaderboard and profile
  updates; the socket also carries presence, reconnection and spectator views.
- **3D client**: a Three.js scene with volumetric ray-marched shaders, animated cards, split
  model streaming and a performance mode for modest machines.
- **Rankings and statistics**: ELO leaderboard, per-player statistics, ELO history and
  analytics charts.
- **Administration**: a separate PHP workspace with KPIs, activity graphs, user and role
  management, ELO corrections and searchable audit logs.

### Architecture

The application is split into five containers orchestrated by Docker Compose:

| Service | Folder | Role |
| --- | --- | --- |
| `web` | `apps/web/` | React + TypeScript single-page application built by Vite and served by Caddy, which terminates HTTPS and proxies the API, the CDN and the admin workspace |
| `api` | `apps/api/` | Go REST and WebSocket API, SQLite database, Uno engine, bots, matchmaking, settlement |
| `cdn` | `apps/cdn/` | Go file CDN for avatars and the 3D client assets (models, textures, shaders, vendor libraries) |
| `admin` | `apps/admin/` | PHP administration workspace with a server-side proxy to the API |
| `redis` | – | Cache used by the API for leaderboard and analytics responses |

Caddy injects the API key server-side, so the browser never sees it. The 3D client is a
static page delivered by the CDN and embedded by the web application; it talks to the same
authoritative API and WebSocket as the rest of the site.

## Instructions

### Prerequisites

- Docker Engine 24+ with Docker Compose v2 (Docker Desktop on macOS/Windows).
- GNU Make.
- A modern browser with WebGL 2 (the project is validated on the latest stable Google Chrome).

For local development outside Docker you also need Go 1.24+, Node.js 20+ with npm 9+,
PHP 8.3+ and Python 3.11+.

### Configuration

All configuration lives in a single `.env` file at the repository root. `make setup` copies
`.env.example` to `.env` when it does not exist. Before the first run, replace the three
placeholder secrets with random values; `make secrets` prints fresh ones:

| Variable | Purpose |
| --- | --- |
| `API_KEY` | Key required on every `/api/*` request. Injected by Caddy, never exposed to the browser |
| `INTERNAL_CDN_API_KEY` | Key used by the API to upload files to the CDN |
| `GAME_SERVICE_KEY` | Key for the internal result-settlement endpoint |
| `HTTP_PORT`, `HTTPS_PORT` | Host ports exposed by Caddy (defaults `1080` and `1443`, chosen to run without root on 42 workstations) |
| `SITE_ADDRESS`, `CORS_ALLOWED_ORIGINS` | Hostname served by Caddy and origins accepted by the API |
| `APP_ENV` | `development` disables API rate limiting for local work; set `production` for an evaluation or a deployment |
| `ADMIN_USERNAMES`, `ADMIN_EMAILS` | Accounts that receive the admin role at startup |
| `DEBUG_AUTH_*`, `VITE_DEBUG_AUTH` | Explicit local-only login bypass, used only by `make debug` |

Never commit `.env`, database files or secrets. `.env` is ignored by Git.

### Run the complete stack

```sh
git clone git@github.com:ecochin-cmd/transcendance.git
cd transcendance
make setup          # creates .env from .env.example, installs tooling
# edit .env: replace API_KEY, INTERNAL_CDN_API_KEY and GAME_SERVICE_KEY
make run-d          # builds the images, starts the five services in the background
```

Then open <https://localhost:1443>. Caddy uses an internal development certificate; the
browser asks for a one-time trust exception, or you can import the root certificate exported
by `make run-d` to `.caddy/root.crt`. Plain HTTP on port 1080 is redirected to HTTPS.

The first admin account is the one named in `ADMIN_USERNAMES` or `ADMIN_EMAILS`. You can also
promote an existing account at any time:

```sh
make admin-grant-email EMAIL=you@learner.42.tech
```

The admin workspace is available at <https://localhost:1443/admin/>.

### Useful commands

```sh
make help                        # list every target
make run                         # same as run-d, in the foreground
make ps                          # container status
make logs SERVICE=api TAIL=100   # follow one service (api, web, cdn, admin, redis)
make test                        # Go tests (API, CDN), TypeScript typecheck, PHP lint
make lint                        # frontend and admin static checks
make format                      # gofmt on the API and the CDN
make audit                       # npm audit of the frontend lockfile
make down                        # stop containers, keep volumes
make fclean                      # remove containers, images and local dependencies
make reset-data CONFIRM=1        # also remove database and file volumes
```

### Local development

```sh
make install                                    # npm ci, go mod download, Python venv
cd apps/api && go test ./... && go vet ./...    # backend tests
cd apps/web && npm run dev                      # Vite dev server
cd apps/web && npx tsc --noEmit -p .            # typecheck
```

`make debug` starts the stack with the explicit login bypass for `DEBUG_AUTH_USERNAME`. It is
opt-in and must never be enabled outside a development machine.

## Usage

### Playing

1. Register with an email, a username and a password of 12 to 128 characters.
2. From the dashboard, either join the **ranked** or **unranked** queue, or open **Games** to
   create a custom table: capacity, hand size, house rules, bots and their difficulty, private
   code and whitelist.
3. When a match is found or the host launches the table, every participant is sent to the 3D
   client. The server deals, enforces turns, penalties and house rules, and plays the bots.
4. At the end of the round the results screen shows the ranking, cards left, hand points and
   points earned. Ranked games update ELO and every game updates statistics.
5. Spectators can watch an in-progress table in read-only mode. A player who loses the
   connection is restored to the table through the reconnect endpoint.

Keyboard shortcuts in the 3D client: `F2` toggles the volumetric shader updates for low-end
machines.

### Public API

Every `/api/*` request requires the `X-API-Key` header. Authenticated requests also send
`Authorization: Bearer <token>`. Rate limiting applies per client outside `development`.
Responses are JSON; errors use `{ "error": "<code>", "message": "<text>" }`.

| Method | Route | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Create an account and open a session |
| `POST` | `/api/v1/auth/login` | Authenticate and open a session |
| `POST` | `/api/v1/auth/logout` | Revoke the current session |
| `GET` | `/api/v1/auth/me` | Current profile and statistics |
| `GET` | `/api/v1/users/{id}` | Public profile |
| `GET` | `/api/v1/users/{id}/analytics` | Public analytics series of a player |
| `GET` | `/api/v1/users/search?q=` | Search players by username |
| `PUT` | `/api/v1/users/me` | Update username, bio or email |
| `PUT` | `/api/v1/users/me/password` | Change the password |
| `PUT` | `/api/v1/users/me/avatar` | Upload a PNG, JPEG or WebP avatar (5 MB max) |
| `DELETE` | `/api/v1/users/me` | Delete the account |
| `GET` | `/api/v1/games` | Waiting, running and finished games |
| `POST` | `/api/v1/games` | Create a custom table |
| `POST` | `/api/v1/games/join` | Join a private table with its code |
| `GET` | `/api/v1/games/{id}` | Lobby view: participants, settings, host controls |
| `POST` | `/api/v1/games/{id}/join` | Join a table |
| `POST` | `/api/v1/games/{id}/leave` | Leave a table |
| `POST` | `/api/v1/games/{id}/spectate` | Watch a table |
| `POST` | `/api/v1/games/{id}/reconnect` | Rejoin a running table after a disconnection |
| `PUT` | `/api/v1/games/{id}/status` | Change a lobby status as the host |
| `DELETE` | `/api/v1/games/{id}` | Delete a table you created |
| `DELETE` | `/api/v1/games/{id}/participants/{userId}` | Kick a participant as the host |
| `POST` | `/api/v1/games/{id}/whitelist` | Add a username to the whitelist |
| `DELETE` | `/api/v1/games/{id}/whitelist/{userId}` | Remove a whitelist entry |
| `POST` | `/api/v1/games/{id}/code/rotate` | Rotate the private code |
| `POST` | `/api/v1/games/{id}/uno/start` | Start the game as the host |
| `GET` | `/api/v1/games/{id}/uno/state` | Server-authoritative snapshot for the caller |
| `POST` | `/api/v1/games/{id}/uno/draw` | Draw a card |
| `POST` | `/api/v1/games/{id}/uno/play` | Play a card (with colour for wilds, optional Uno call) |
| `POST` | `/api/v1/games/{id}/uno/action` | Other actions: `pass`, `uno`, `accept` a penalty, `challenge` a +4, `swap` hands, `catch` a missed Uno |
| `POST` | `/api/v1/games/{id}/result` | Settle a result (internal, `X-Game-Service-Key`) |
| `POST` | `/api/v1/matchmaking/queue` | Join the ranked or unranked queue |
| `GET` | `/api/v1/matchmaking/queue` | Queue position or match ticket with the created `gameId` |
| `DELETE` | `/api/v1/matchmaking/queue` | Leave the queue |
| `GET` | `/api/v1/leaderboard` | ELO leaderboard |
| `GET` | `/api/v1/ws` | WebSocket upgrade (see below) |
| `POST` | `/api/v1/telemetry/vitals` | Web Vitals beacon from the frontend |
| `GET` | `/api/v1/admin/overview` | Platform statistics (admin) |
| `GET` | `/api/v1/admin/dashboard` | KPIs, activity series, distributions, recent audit events (admin) |
| `GET` | `/api/v1/admin/logs` | Search and filter audit events (admin) |
| `GET` | `/api/v1/admin/logs/stats` | Aggregated audit statistics (admin) |
| `GET` | `/api/v1/admin/users` | User list (admin) |
| `GET` | `/api/v1/admin/users/{id}/analytics` | Private analytics of a user (admin) |
| `PUT` | `/api/v1/admin/users/{id}/elo` | Correct a user's ELO (admin) |
| `PUT` | `/api/v1/admin/users/{id}/role` | Set the `player`, `staff`, `developer` or `admin` role (admin) |
| `DELETE` | `/api/v1/admin/users/{id}` | Delete a user and linked data (admin) |

### WebSocket

The client opens `/api/v1/ws` with its bearer token, then subscribes to topics: `lobbies`,
`game:{id}`, `user:{id}`, `leaderboard`, `profile:{id}`. The hub pushes `game.updated`,
`game.deleted`, `uno.state`, `lobbies.changed`, `leaderboard.changed`, `profile.changed` and
`matchmaking.updated`. Each `uno.state` payload is rendered per viewer, so a player only sees
their own hand while spectators see the public table. The socket is also the presence signal
used by lobbies and reconnection.

## Team information

| Member (42 login) | Role(s) | Responsibilities |
| --- | --- | --- |
| **ecochin** | Product Owner, Tech Lead | Project concept and product direction, overall architecture (API, web, admin, CDN, schema), platform foundation, integration decisions, code review |
| **mschyns** | Developer – gameplay and matchmaking | Card effects, scoring, house rules, end-of-game results page, matchmaking-to-game pipeline, settlement wiring, pull request reviews |
| **halexand** | Developer – real time, AI and performance | WebSocket hub and real-time client, official Uno rules on the server, AI opponents, shader optimisation, bug fixes |
| **clfontai** | Developer – API, CDN and deployment | REST API and CDN implementation with ecochin, HTTPS certificate export, port configuration for 42 workstations, settlement fixes |
| **julhuang** | Developer – 3D client | Three.js scene, models, cards, camera, animations, networked animation logic, ray-marched shaders, performance toggles |

## Project management

- **Organisation.** ecochin laid down the platform foundation and the service boundaries, then
  the work was split by area: gameplay and matchmaking (mschyns), real time and AI (halexand),
  API, CDN and deployment (clfontai), 3D client (julhuang). ecochin arbitrated integration
  questions and kept the product direction.
- **Workflow.** Every feature lives on its own branch (`feature/websocketRefonte`,
  `feature/cards_effects`, `feature/ai`, `feature/matchmaking`, …) and reaches `main` through a
  GitHub pull request reviewed and merged by another team member. Go tests, `go vet`, the
  TypeScript typecheck and the PHP lint are run before merging. Short synchronous sessions were
  held when two areas overlapped, for example the Uno rules written by mschyns and halexand
  together.
- **Tools.** GitHub for the repository, branches and pull requests; Docker Compose so every
  member runs the identical stack; `make test` and `make lint` as the shared quality gate.
- **Communication.** A team Discord server for daily coordination, voice sessions and sharing
  screenshots, plus in-person work at the campus.

## Technical stack

### Frontend

- **React 19 + TypeScript 5**, built with **Vite 8**, routed by **React Router 7**.
- A small in-house **design system** (`apps/web/src/components/ui.tsx`): Button, Input,
  EmailInput, TextArea, Card, Badge, StatCard, PageHeader, EmptyState, LoadingState,
  UserAvatar, UnoLogo, with shared CSS tokens, dark theme, responsive breakpoints and ARIA
  attributes. Icons come from Papicons.
- **Web Vitals** (LCP, CLS, INP, FCP, TTFB) measured with `PerformanceObserver` and sent to the
  API telemetry endpoint.
- **3D client**: **Three.js r180** and **GSAP 3** served by the CDN, with custom GLSL
  ray-marching shaders (volumetric fog, ground mist, orb heads, card trails, a compute pass for
  the trail stamps).

### Backend

- **Go 1.24** with the standard-library `net/http`, directory-based routing under
  `internal/api/v1`, middleware for API key, bearer sessions, rate limiting, CORS, security
  headers and audit logging.
- **WebSockets** with `github.com/coder/websocket` and an in-process hub with per-viewer
  rendering.
- **Uno engine** (`internal/uno`): rules, bots and engine with 7 test files run with `-race`.
- **Matchmaking** (`internal/matchmaking`): in-memory ranked/unranked queue with widening ELO
  windows that creates the game when a match is found.
- **Settlement** (`internal/settlement`) and **rating** (`internal/rating`): idempotent ELO
  calculation and statistics in a single transaction.
- **OpenTelemetry** HTTP instrumentation with an optional stdout trace exporter.
- **Admin workspace**: **PHP 8.3** (`php:8.3-cli-alpine`), server-rendered shell, vanilla
  JavaScript charts and a server-side proxy that adds the API key and checks the admin role.
- **CDN**: a second Go service with namespaced keys, atomic uploads, size and format checks,
  cache headers and split GLB streaming for the 800 MB of 3D models.

### Database

**SQLite** through `mattn/go-sqlite3`, with foreign keys, WAL journal, busy timeout and
explicit transactions. It was chosen because the project has one durable data store owned by
one API process, which SQLite handles with zero operational cost: no extra container, a single
file in a Docker volume, trivial backups and `make reset-data`. Transactions keep concurrent
requests consistent, and the schema is plain SQL that would move to PostgreSQL unchanged if the
platform ever needed several API instances. **Redis 7** caches the leaderboard and analytics
responses.

### Infrastructure

**Docker Compose** with five services, **Caddy 2.9** as the only exposed entry point (HTTPS
with `tls internal`, HTTP to HTTPS redirect, gzip, HSTS, CSP, `X-Frame-Options`, long-lived
cache headers for hashed assets). GNU Make wraps every command.

### Justification of the main choices

- **Go instead of a framework-heavy backend**: the subject asks for a secure, concurrent
  server; Go's standard library gives HTTP, TLS-ready handlers, goroutines for bots and timers,
  and a single static binary per service. It also allowed the CDN and the API to share
  conventions.
- **Server authority**: every card is dealt, validated and scored on the server. The 3D client
  and the React pages are views over the same snapshot, which prevents cheating and makes
  spectators and reconnection trivial.
- **WebSockets rather than polling**: a four-player table with bots produces many small state
  changes per second; one socket per client with per-viewer rendering costs less than polling
  and doubles as presence.
- **Separate PHP admin**: the subject expects the admin area to be isolated from the player
  application; a distinct container with its own session and a server-side proxy means the
  browser never holds an admin API key.
- **Dedicated CDN**: avatars and the 3D assets (models, textures, shaders) are large and
  cacheable; isolating them keeps the API small and lets Caddy cache aggressively.

## Database schema

```text
users 1 ──── 1 user_stats
  │
  ├──< auth_sessions
  ├──< audit_logs (actor_user_id)
  ├──< user_elo_history >──┐
  ├──< games (created_by)  │
  │       │                │
  │       ├── 1 game_settings
  │       ├── 1 uno_game_states
  │       ├──< game_participants >──┤
  │       ├──< game_spectators  >──┤
  │       └──< game_whitelist   >──┘
  └────────────────────────────────┘  (every >── side references users.id)
```

| Table | Purpose | Key fields |
| --- | --- | --- |
| `users` | Account identity | `id TEXT PK`, `username` and `email` (unique, case-insensitive), `password_hash` (bcrypt), `avatar_url`, `elo INTEGER` (default 1000), `role` (`player`, `staff`, `developer`, `admin`), `created_at`, `last_login`, `deleted_at` |
| `user_stats` | One row per user | `games_played`, `games_won`, `current_streak`, `max_streak`, `unos_called`, `uno_penalties`, `draw_fours_played`, `cards_drawn` |
| `games` | Table lifecycle | `id TEXT PK`, `status` (`waiting`, `in_progress`, `finished`), `created_by → users`, `mode` (`ranked`, `unranked`, `custom`), `is_private`, `join_code`, `turn_time_limit`, `started_at`, `ended_at` |
| `game_settings` | Rules of a table | `max_players` (2–8), `starting_hand_size` (3–14), `draw_penalty` (1–4), `stack_draw_cards`, `jump_in`, `seven_zero`, `draw_until_playable`, `force_play`, `challenge_draw_four`, `allow_multiple_decks`, `bots_enabled`, `bot_difficulty` |
| `game_participants` | Players of a table | composite PK `(game_id, user_id)`, `position`, `score`, `elo_delta`, `last_seen_at` |
| `game_spectators` | Viewers of a table | composite PK `(game_id, user_id)`, `joined_at`, `last_seen_at` |
| `game_whitelist` | Allowed accounts of a private table | composite PK `(game_id, user_id)` |
| `uno_game_states` | Authoritative game snapshot | `game_id PK`, `version`, `state_json`, `updated_at` |
| `user_elo_history` | One immutable row per rating change | `previous_elo`, `new_elo`, `delta`, `reason`, `game_id` (nullable for admin corrections) |
| `auth_sessions` | Revocable bearer sessions | `token_hash` (unique), `expires_at`, `revoked_at`, `user_agent`, `ip_address` |
| `audit_logs` | Security and admin events | `actor_user_id`, `action`, `target_type`, `target_id`, `severity` (`info`, `warning`, `critical`) |

All foreign keys cascade on delete except `games.created_by`, `user_elo_history.game_id` and
`audit_logs.actor_user_id`, which are set to `NULL` so history survives account deletion. The
canonical schema is [`apps/api/db/schema.sql`](apps/api/db/schema.sql).

## Features list

| Feature | Owner(s) | Description |
| --- | --- | --- |
| Platform foundation and architecture | ecochin | Service split, Docker Compose, Caddy, Makefile, SQLite schema, routing conventions |
| Authentication and sessions | ecochin | Registration, login, logout, bcrypt hashing, revocable bearer sessions, server and client validation |
| Profiles and avatars | ecochin, clfontai | Profile page, bio, email and password change, avatar upload through the CDN, generated default avatars, public profiles and search |
| Custom tables and lobbies | ecochin | Public and private tables, codes, whitelist, kick, status, settings form with house rules and bots |
| Real-time layer | halexand | WebSocket hub, topics, per-viewer rendering, presence, React hooks replacing polling, 3D client subscription |
| Official Uno rules on the server | halexand, mschyns | 108-card deck, turn order, action cards, wild cards, Uno call and catch, scoring from the official Mattel rules |
| Card effects and house rules | mschyns, halexand | +2/+4 penalties and stacking, jump-in, seven-zero swaps and rotations, draw until playable, force play, +4 challenge |
| Scoring and results page | mschyns | Hand points, points earned, ranked `results` list and the end-of-game results page |
| AI opponents | halexand | Easy, intermediate and expert bots with distinct strategies, server-side turns, rule-aware decisions |
| Matchmaking | ecochin, mschyns | Ranked and unranked queues with ELO windows, automatic game creation and redirection |
| Settlement, ELO and statistics | ecochin, mschyns, clfontai | Idempotent ELO calculation, statistics counters, ELO history, bot-game handling |
| Reconnection and spectators | ecochin, halexand | Reconnect endpoint, spectator endpoint and read-only table view |
| Leaderboard and player analytics | ecochin | ELO leaderboard, player statistics, chart geometry computed in Go |
| 3D client | julhuang | Three.js scene, table, chairs and glasses models, split GLB streaming, cards, camera, hands, animations synchronised with the network |
| Volumetric shaders | julhuang, halexand | Ray-marched fog, mist, orb heads and card trails; optimisation passes for low-end GPUs |
| Administration workspace | ecochin | PHP shell, dashboard KPIs, activity graphs, users, roles, ELO corrections, audit logs |
| CDN | ecochin, clfontai | Namespaced uploads, validation, cache headers, 3D asset delivery |
| HTTPS and deployment on 42 workstations | clfontai | Certificate export, rootless ports 1080/1443, Compose and Caddy adjustments |
| Design system and accessibility | ecochin | Reusable components, tokens, responsive layout, ARIA attributes |
| Legal pages | ecochin | Privacy policy and terms of service linked from the footer |

## Modules

Point rule: Major = 2 points, Minor = 1 point. The subject requires 14 points.

| Module | Type | Pts | Owner(s) | Why we chose it and how it is implemented |
| --- | --- | --- | --- | --- |
| Web – Frontend framework | Minor | 1 | ecochin | React 19 and TypeScript give typed, component-based pages. Vite builds a static bundle served by Caddy |
| Web – Public API | Major | 2 | ecochin, clfontai | The API is the single contract for the web client, the 3D client and the admin proxy. API key middleware, per-client rate limiting, GET/POST/PUT/DELETE routes, JSON errors; documented in this README |
| Web – Custom design system | Minor | 1 | ecochin | A small component library in `ui.tsx` with shared tokens guarantees a consistent, accessible UI across player pages |
| Gaming – Complete web game | Major | 2 | mschyns, halexand, julhuang | Uno is the heart of the project. Rules, scoring, win conditions and bots run on the server; the 3D client renders the snapshots |
| Gaming – Remote players | Major | 2 | halexand, ecochin | Each player uses their own machine. Per-viewer snapshots, WebSocket pushes, presence and a reconnect endpoint handle distant clients |
| Gaming – Multiplayer (3+) | Major | 2 | mschyns, halexand | Tables accept 2 to 8 seats; direction, skips, stacks and seven-zero rotations are written for n players |
| Gaming – AI opponent | Major | 2 | halexand | Lets a player train or fill a table. Three profiles with different card selection, colour choice and Uno behaviour, all respecting the house rules |
| Gaming – Game customization | Minor | 1 | mschyns, ecochin | Seven house rules, hand size, capacity and bot difficulty stored per table and enforced by the engine |
| Gaming – Spectator mode | Minor | 1 | ecochin, halexand | Spectators subscribe to the table topic and receive the public view of every hand |
| User – Game statistics and history | Minor | 1 | ecochin, mschyns, clfontai | Settlement fills `user_stats`, `game_participants` and `user_elo_history` after every game; profiles show statistics and ELO history |
| Graphics – Advanced 3D | Major | 2 | julhuang | A Three.js table with streamed models and custom ray-marched volumetric shaders gives the game its identity |
| Realtime – WebSockets | Major | 2 | halexand | Lobbies, tables, queue, leaderboard and profiles update instantly without polling; the socket doubles as presence |
| User – Activity analytics dashboard | Minor | 1 | ecochin | Admin dashboard with KPIs, activity series and distributions; public player analytics charts |

**Total claimed: 20 points** (7 majors = 14, 6 minors = 6).

Modules we deliberately do not claim: *Standard user management* (friends and online-status
list are not implemented) and *Advanced permissions* (the `staff` and `developer` roles exist in
the schema but only `admin` grants access today).

## Individual contributions

### ecochin – Product Owner, Tech Lead

- Imagined the project and set the product direction.
- Built the platform foundation: Go API with directory-based routing, middleware stack, SQLite
  schema, matchmaking queue, settlement and rating packages, React application and design
  system, PHP admin workspace, Go CDN, Docker Compose, Caddy and Makefile.
- Bundled the 3D assets in the CDN and differentiated bot avatars.
- Reviewed and merged pull requests.
- **Challenge**: keeping four runtimes (Go, TypeScript, PHP, GLSL) aligned on one API contract
  while preserving server authority. Solved with a single typed `client.ts`, a documented route
  table and per-viewer snapshot rendering.

### mschyns – Gameplay and matchmaking

- Implemented the card effects and the end-of-game results page.
- Wrote the seven house rules (+2/+4 penalties and stacking, jump-in, seven-zero, draw until
  playable, force play, +4 challenge) and worked with halexand on the official rules.
- Implemented score calculation and the ranked `results` list.
- Connected matchmaking to game creation: match handler, `gameId` in the queue status,
  automatic redirection to the table, and wired the settlement to the end of every game.
- Removed API rate limits in development and improved spawn points in the 3D scene.
- **Challenge**: the rules were first written inside the engine while a parallel branch split
  them into `rules.go`. The team compared both versions, kept the separated layout and
  re-applied the house rules on top, with tests for each of them.

### halexand – Real time, AI and performance

- Designed and implemented the WebSocket layer: Go hub, topics, per-viewer rendering, presence,
  React hooks and 3D client subscription, replacing every polling loop.
- Ported the official Mattel Uno rules to the server with mschyns and added the `penalty` and
  `swap` phases and the generic `uno/action` endpoint.
- Created the AI opponents: three difficulty levels with distinct strategies that respect the
  table rules, with unit tests run under the race detector.
- Reviewed and optimised the volumetric shaders (early exit on optical depth, segment culling of
  the trail compute pass, hoisted trigonometry) so the 3D client runs on modest laptops, with
  no visual regression.
- Fixed bugs across the stack and audited the project against the subject.
- **Challenge**: shader changes cannot be unit-tested like Go code. Every modification was
  checked by assembling the full GLSL programs, parsing them with a scope-aware parser and
  comparing outputs on sampled pixels before and after the change.

### clfontai – API, CDN and deployment

- Implemented the REST API and the CDN with ecochin: route contracts, file delivery and asset
  serving.
- Made the stack run on 42 workstations without root: ports 1080/1443, Caddy and Compose
  adjustments, export of the Caddy root certificate for a trusted HTTPS.
- Fixed the settlement of games against bots so they are always marked finished while ranked
  games keep the two-player minimum.
- **Challenge**: the campus machines forbid privileged ports and custom certificates.
  Solved by moving Caddy to high ports and exporting the internal CA through `make trust-cert`.

### julhuang – 3D client

- Built the complete Three.js client: scene, table and chair models from Poly Haven, glasses,
  split GLB streaming, cards, camera, player heads and seats.
- Wrote the ray-marched volumetric shaders and their integration in Three.js.
- Rewrote the card animation logic so it stays faithful to the network state after the real-time
  layer landed, fixed head identifiers and cleaned warnings and errors.
- Added the `F2` toggle to pause shader updates on low-end machines.
- **Challenge**: animations originally assumed a local deck; once the server became the source
  of truth, each incoming snapshot had to be diffed against the previous one to replay only the
  move that just happened.

## Known limitations

- No server-side turn timer for human players: a player who closes the tab during their turn
  must reconnect for the table to continue.
- Friends list and online-status list are not implemented.
- The `staff` and `developer` roles have no effect yet; only `admin` opens the admin workspace.
- Rate limiting is disabled while `APP_ENV=development`; set `production` for an evaluation.
- The 3D models weigh about 800 MB and are tracked directly in Git, which makes the first clone
  slow.

## Resources

### Documentation and references

- Go `net/http`: <https://pkg.go.dev/net/http>
- Go SQLite driver: <https://github.com/mattn/go-sqlite3>
- SQLite documentation: <https://www.sqlite.org/docs.html>
- `coder/websocket`: <https://pkg.go.dev/github.com/coder/websocket>
- React: <https://react.dev/learn>
- React Router: <https://reactrouter.com/>
- Vite: <https://vite.dev/guide/>
- Three.js manual and examples: <https://threejs.org/manual/> and <https://threejs.org/examples/>
- GSAP: <https://gsap.com/docs/v3/>
- Inigo Quilez on ray marching and distance functions: <https://iquilezles.org/articles/>
- Caddy: <https://caddyserver.com/docs/>
- Docker Compose: <https://docs.docker.com/compose/>
- Elo rating system: <https://en.wikipedia.org/wiki/Elo_rating_system>
- Official Uno rules (Mattel instruction sheet): <https://service.mattel.com/instruction_sheets/53641.pdf>
- OWASP Cheat Sheet Series: <https://cheatsheetseries.owasp.org/>

### AI usage

AI assistants (Claude Code and ChatGPT) were used as engineering assistants, explaining topics and making this README so it fit the subject needs perfectly

## Credits and license

- Table and chair models come from [Poly Haven](https://polyhaven.com/) (CC0).
- Icons from [Papicons](https://github.com/PapillonApp/papicons).
- Uno is a trademark of Mattel; this is a non-commercial student project built for the 42
  curriculum and is not affiliated with Mattel.

The source code is provided for educational purposes as part of the 42 curriculum.
