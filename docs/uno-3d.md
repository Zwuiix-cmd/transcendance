# Uno 3D integration

Uno 3D is an integral part of the platform. The compiled game and its local runtime assets are
served by the project CDN at `/files/uno`; Vite does not extract or serve these assets.

The complete asset bundle is stored in `apps/cdn/assets/uno/` as project assets. The large world
model is split into CDN-served parts under `models/uno_parts/`, so no source archive is needed at
runtime. Docker Compose mounts this directory read-only into the CDN container.

## Local access

After provisioning the assets, start the stack with `make run-d`, then open:

```text
https://localhost/uno
```

From an authenticated lobby, the `Start Uno 3D` button moves the table to `in_progress`, then
`Open Uno 3D` opens:

```text
https://localhost/uno?game=<game-id>
```

No asset extraction step is required. After cloning the repository, `make run-d` serves the
versioned files already present in `apps/cdn/assets/uno/`.

## Prepared match requests

- `GET /api/v1/games`: paginated table listing
- `POST /api/v1/games`: create a table and its settings
- `GET /api/v1/games/{id}`: table state and participants
- `POST /api/v1/games/{id}/join` or `POST /api/v1/games/join`: join a table
- `POST /api/v1/games/{id}/leave`: leave a table
- `PUT /api/v1/games/{id}/status`: start a table (`in_progress`)
- `POST /api/v1/games/{id}/result`: final result sent by the future game service

The platform wrapper sends the game a `postMessage` message of type `uno.bootstrap` containing
`gameId`, the authenticated player and the table details. The 3D client uses the server-owned
Uno state endpoints for start, state, draw and play actions; the server remains authoritative.
