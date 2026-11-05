# Uno 3D integration

Uno 3D is an integral part of the platform. The compiled game and its local runtime assets are
served by the project CDN at `/files/uno`; Vite does not extract or serve the archive.

The assets are intentionally ignored by Git because the main model is approximately 805 MB.
Deployments must provision the asset directory before starting Docker Compose.

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

Provision the local CDN directory with:

```sh
make uno-assets
```

The target extracts `Uno.zip`, copies the installed Three.js and GSAP runtime into the asset
directory, and rewrites the archive import map to use `/files/uno/vendor/...` on the project CDN.

## Prepared match requests

- `GET /api/v1/games`: paginated table listing
- `POST /api/v1/games`: create a table and its settings
- `GET /api/v1/games/{id}`: table state and participants
- `POST /api/v1/games/{id}/join` or `POST /api/v1/games/join`: join a table
- `POST /api/v1/games/{id}/leave`: leave a table
- `PUT /api/v1/games/{id}/status`: start a table (`in_progress`)
- `POST /api/v1/games/{id}/result`: final result sent by the future game service

The platform wrapper sends the game a `postMessage` message of type `uno.bootstrap` containing
`gameId`, the authenticated player and the table details. The archived game is currently an
autonomous 3D client; the real-time backend can consume this context once it is connected.
