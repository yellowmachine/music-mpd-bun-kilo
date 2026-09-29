# mpd-bridge

A small HTTP service that runs on the Raspberry Pi next to MPD and Snapserver
and exposes a **closed set** of their actions to the `svelte-mpd` backend on
the VPS, through a Cloudflare tunnel.

- No endpoint forwards raw MPD commands or JSON-RPC calls. Every action is a
  typed endpoint with validated input.
- Its own bearer-token auth, independent of Cloudflare Access.
- It needs no volumes: it never reads `/music`. Every library access goes
  through MPD.
- It uses only the Go standard library, so there are no third-party dependencies.

```
VPS (svelte-mpd) ──HTTPS──▶ Cloudflare Access ──tunnel──▶ cloudflared ──▶ mpd-bridge ──▶ mpd:6600
                                                                                    └──▶ snapserver:1705
```

## Configuration

| Variable         | Default          | Notes                                                                   |
| ---------------- | ---------------- | ----------------------------------------------------------------------- |
| `API_TOKEN`      | —                | **Required**, at least 32 characters (`openssl rand -hex 32`).          |
| `API_TOKEN_FILE` | —                | Read the token from a file (Docker secrets). Use one or the other.      |
| `LISTEN_ADDR`    | `127.0.0.1:8787` | Use `0.0.0.0:8787` inside compose, with no `ports:` published.          |
| `MPD_HOST`       | `127.0.0.1`      |                                                                         |
| `MPD_PORT`       | `6600`           |                                                                         |
| `MPD_PASSWORD`   | —                | Optional.                                                               |
| `SNAP_ENABLED`   | `true`           | `false` makes the `/snap/*` endpoints answer 503.                       |
| `SNAP_HOST`      | `127.0.0.1`      |                                                                         |
| `SNAP_PORT`      | `1705`           | Snapserver's TCP control port (not the 1780 HTTP one).                  |
| `LOG_LEVEL`      | `info`           | `debug` also logs `/healthz` requests.                                  |

## Endpoints

All endpoints except `GET /healthz` require `Authorization: Bearer <token>`.
Command endpoints answer `204 No Content`. Errors are returned as `{"error": "..."}`:

| Status | Meaning                                                         |
| ------ | --------------------------------------------------------------- |
| 400    | Invalid input                                                   |
| 401    | Missing or wrong token                                          |
| 404    | No such song, directory, playlist or Snapcast client            |
| 409    | Conflict (for example, a database update is already running)    |
| 413    | Body larger than 64 KB                                          |
| 503    | MPD or Snapserver is not connected right now                    |

**Status and playback**

| Method | Path                                                                               | Body                                         |
| ------ | ---------------------------------------------------------------------------------- | -------------------------------------------- |
| GET    | `/healthz`                                                                         | → `{ok, mpd, snap}`                          |
| GET    | `/status`                                                                          | → `{status, song}`                           |
| POST   | `/player/play` · `pause` · `resume` · `toggle` · `stop` · `next` · `previous`      | —                                            |
| POST   | `/player/playid`                                                                   | `{"id": 7}`                                  |
| POST   | `/player/seek`                                                                     | `{"seconds": 42.5}`                          |
| PUT    | `/volume`                                                                          | `{"value": 0-100}`                           |
| PUT    | `/options`                                                                         | `{"random"?, "repeat"?, "single"?, "consume"?}` |

**Queue**

| Method | Path          | Body                                                   |
| ------ | ------------- | ------------------------------------------------------ |
| GET    | `/queue`      | → `[{file, title, …, id, pos}]`                        |
| POST   | `/queue`      | `{"uris": [...1-500], "replace"?: bool, "play"?: bool}` |
| DELETE | `/queue`      | Clears the whole queue                                 |
| DELETE | `/queue/{id}` | Removes one song by its song id                        |

A URI can be any of:

- a path relative to the library root, such as `Blur/Parklife/01.flac` (no leading `/` and no `..`);
- `/`, which adds the whole library;
- an `http(s)://` stream URL, for radio.

Any other scheme, such as `file://`, is rejected. With `replace`, the queue is
cleared first. `play` starts playback afterwards, all in a single MPD command list.

**Library**

| Method | Path                     | Notes                                                                                                                                                  |
| ------ | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| GET    | `/library/ls?path=<dir>` | → `{directories: [{directory, name}], files: [...]}`. An empty `path` is the root.                                                                     |
| GET    | `/library/all`           | → `{db_update, songs}`. Supports gzip and an `ETag` based on MPD's `db_update`; send `If-None-Match` to get a `304`. The result is cached until the database changes. |
| POST   | `/library/update`        | → `202 {job}`, starts a rescan.                                                                                                                        |

**Stored playlists** (the `{name}` in the path is URL-encoded)

| Method | Path                           | Body                                   |
| ------ | ------------------------------ | -------------------------------------- |
| GET    | `/playlists`                   | → `[{playlist, last_modified}]`        |
| GET    | `/playlists/{name}`            | → `[song]`                             |
| POST   | `/playlists/{name}/songs`      | `{"uri": "..."}`                       |
| DELETE | `/playlists/{name}/songs/{pos}` | —                                     |
| POST   | `/playlists/{name}/move`       | `{"from": 3, "to": 0}`                 |
| POST   | `/playlists/{name}/load`       | optional `{"replace"?, "play"?}`       |

**Snapcast**

| Method | Path                         | Body                                                              |
| ------ | ---------------------------- | ----------------------------------------------------------------- |
| GET    | `/snap/clients`              | → `[{id, connected, name, host, ip, volume: {percent, muted}}]`   |
| PUT    | `/snap/clients/{id}/volume`  | `{"percent"?: 0-100, "muted"?: bool}`; a field you omit keeps its current value |

**Events** — `GET /events` (Server-Sent Events)

The first event is a `snapshot` with `{status, song, queue, snap_clients, connection}`.
After that, the stream sends the same event names the app already uses internally:

| Event             | Payload                                |
| ----------------- | -------------------------------------- |
| `player`          | `{status, song}`                       |
| `mixer`           | `{volume}`                             |
| `playlist`        | `{queue}`                              |
| `options`         | `{random, repeat, single, consume}`    |
| `database`        | `{}`                                   |
| `stored_playlist` | `{}`                                   |
| `snap_clients`    | `{clients}`                            |
| `connection`      | `{mpd, snap}` — sent when either connection goes up or down |

A `: ping` comment is sent every 25 s so the tunnel keeps the connection open.
A client that falls behind is disconnected. When it reconnects, it gets a fresh snapshot.

### Examples

```sh
B=https://mpd-bridge.example.com
AUTH=(-H "Authorization: Bearer $API_TOKEN"
      -H "CF-Access-Client-Id: $CF_ACCESS_CLIENT_ID"
      -H "CF-Access-Client-Secret: $CF_ACCESS_CLIENT_SECRET")

curl "${AUTH[@]}" $B/status
curl "${AUTH[@]}" -X POST $B/player/toggle
curl "${AUTH[@]}" -X PUT $B/volume -d '{"value": 40}'
curl "${AUTH[@]}" -X POST $B/queue -d '{"uris": ["Blur/Parklife"], "replace": true, "play": true}'
curl "${AUTH[@]}" -X POST "$B/playlists/Road%20trip/load" -d '{"replace": true, "play": true}'
curl "${AUTH[@]}" --compressed $B/library/all -o library.json
curl "${AUTH[@]}" -N $B/events
```

## Deployment on the Pi

`docker-compose.pi.yml` at the repo root runs the full Pi stack:

- mpd, snapserver and snapclient;
- this bridge, with no published ports;
- `cloudflared`.

Put the two secrets in `.env`, next to that file:

```env
BRIDGE_API_TOKEN=<openssl rand -hex 32>
CLOUDFLARE_TUNNEL_TOKEN=<tunnel token>
```

### Cloudflare setup

1. **Tunnel.** Go to Zero Trust → Networks → Tunnels and create a tunnel of type
   *Cloudflared*. Copy its token into `CLOUDFLARE_TUNNEL_TOKEN`. Add a
   *public hostname*, for example `mpd-bridge.example.com`, with service
   `http://mpd-bridge:8787`. If you run the bridge outside compose, with the
   default `LISTEN_ADDR`, use `http://127.0.0.1:8787` instead.
2. **Service token.** Go to Zero Trust → Access → Service Auth → Service Tokens
   and create one. Keep its Client ID and Secret for the app on the VPS.
3. **Access application.** Go to Zero Trust → Access → Applications and add a
   *Self-hosted* application for `mpd-bridge.example.com`. Give it a single policy
   with **Action: Service Auth** and **Include: Service Token = (the token above)**.
   Browsers get blocked, and only requests that carry
   `CF-Access-Client-Id` / `CF-Access-Client-Secret` get through.

On the VPS, the app needs the bridge URL, `API_TOKEN` and the two
`CF-Access-*` values.

### Rotating the token

1. Generate a new token.
2. Update `BRIDGE_API_TOKEN` on the Pi and run `docker compose -f docker-compose.pi.yml up -d mpd-bridge`.
3. Update the app's copy on the VPS.

Requests fail with 401 in the short gap between steps 2 and 3.

## Development

```sh
go test -race ./...
API_TOKEN=$(openssl rand -hex 32) go run ./cmd/mpd-bridge   # expects MPD on 127.0.0.1:6600
```

Layout:

- `cmd/mpd-bridge`: entry point and the `healthcheck` subcommand.
- `internal/mpd`: protocol client, with reconnection, keepalive and the idle watcher.
- `internal/snap`: Snapserver client.
- `internal/httpapi`: routes, validation and SSE.
- `internal/feed`: turns MPD and Snapcast changes into events.
- `internal/events`: fan-out to the SSE subscribers.

### Why not gompd?

The MPD client is a ~450-line implementation of the protocol subset in use,
not `github.com/fhs/gompd`, for two reasons:

- gompd can't set I/O deadlines. An MPD that stops responding would block the
  request, and the connection mutex with it, forever.
- Its `Watcher.Close` can deadlock if an error arrives while it is closing.

Owning the client also means a command can only ever be built from a fixed name
plus quoted arguments. An argument with a line break is rejected, so request
data can't inject MPD commands.
