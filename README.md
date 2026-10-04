# svelte-mpd

A self-hosted web frontend for [MPD (Music Player Daemon)](https://www.musicpd.org/) with real-time synchronization and multi-room audio via [Snapserver](https://github.com/badaix/snapcast). Designed to run on a Raspberry Pi or any Linux server on a local network.

Built with Bun, SvelteKit 2, Svelte 5 (runes), TailwindCSS v4, and a typewriter/terminal aesthetic.

## Features

- **Playback control** — play, pause, stop, next/prev, seek, volume, random, repeat
- **Queue management** — view current queue, jump to track, remove tracks, clear queue
- **Library browser** — hierarchical filesystem navigation via MPD `lsinfo`; add folders or files to queue
- **Fuzzy search** — instant full-library search powered by a server-side MiniSearch index
- **Multi-room audio** — per-client volume and mute control via Snapserver
- **Real-time sync** — SSE keeps all open browser tabs in sync (player, queue, volume, options)
- **Admin panel** — trigger MPD database rescan
- **Runs anywhere** — the app talks to MPD and Snapserver through [`mpd-bridge`](mpd-bridge/README.md), so it can live on the Pi next to the speakers or on a VPS, reaching the Pi through a Cloudflare tunnel
- **PWA** — installable as a standalone app

---

## Stack

| Layer             | Technology                             |
| ----------------- | -------------------------------------- |
| Runtime           | Bun 1.3                                |
| Frontend          | SvelteKit 2 + Svelte 5 (runes)         |
| Adapter           | `@sveltejs/adapter-node`               |
| Styling           | TailwindCSS v4                         |
| Icons             | `phosphor-svelte` v3                   |
| MPD / Snapcast    | HTTP + SSE via `mpd-bridge` (Go)       |
| Snapserver client | WebSocket + HTTP JSON-RPC              |
| Search            | MiniSearch v7 (in-memory, server-side) |
| Real-time         | Server-Sent Events (SSE)               |
| Containerization  | Docker + Docker Compose                |

---

## Quick start (Docker Compose)

Recommended for production. Runs MPD + Snapserver + the web app as a single stack.

### Prerequisites

- Docker and Docker Compose installed
- Your music files accessible on the host

### 1. Clone and configure

```sh
git clone <repo-url>
cd music-bun-kilo
```

Create a `.env` file at the project root:

```env
# REQUIRED: password for the web UI login screen (see "Login" below)
ADMIN_PASSWORD=<a password of your choice>

# REQUIRED: token shared by the app and mpd-bridge (openssl rand -hex 32)
BRIDGE_API_TOKEN=<a long random string>

# REQUIRED: LAN IP (or hostname) of the server + web app port
# Must be reachable by other devices on your network
ORIGIN=http://192.168.0.x:3000

# Web app port
PORT=3000

# AI assistant (optional — see "AI assistant" section below)
ANTHROPIC_API_KEY=sk-ant-...
ASSISTANT_TOKEN=<a long random string, e.g. `openssl rand -hex 32`>
WHISPER_HOST=whisper
WHISPER_PORT=5001

# Optional: use OpenAI's cloud STT instead of the local Whisper sidecar.
# Much faster on weak hardware (e.g. Raspberry Pi) since transcription no
# longer runs on-device. If set, this takes priority over WHISPER_HOST/PORT.
OPENAI_API_KEY=sk-...
```

> **`ORIGIN` is required.** SvelteKit uses it to validate `Host` headers for remote function calls. Set it to the actual LAN address of your server.

### 2. Mount your music

By default, `docker-compose.yml` mounts `./music` into the MPD container. Create that directory and place (or symlink) your music collection there:

```sh
mkdir -p music
# copy or symlink your music into ./music/
```

To use a different path, edit the `mpd` service volumes in `docker-compose.yml`:

```yaml
volumes:
  - /your/actual/music/path:/var/lib/mpd/music:ro
```

### 3. Start the stack

```sh
docker compose up --build   # first run (builds images)
docker compose up           # subsequent runs
docker compose up -d        # detached / background
```

### 4. Access the app

Open `http://<your-server-ip>:3000` from any device on your local network.

After adding new music files, go to `/admin` and click **Update database** to trigger an MPD library rescan.

---

## Ports

| Service    | Port | Protocol | Description                               |
| ---------- | ---- | -------- | ----------------------------------------- |
| svelte-mpd | 3000 | HTTP     | Web application                           |
| mpd        | 6600 | TCP      | MPD protocol (for external MPD clients)   |
| snapserver | 1704 | TCP      | Snapcast audio stream                     |
| snapserver | 1705 | TCP      | Snapcast control (used by mpd-bridge)     |
| snapserver | 1780 | HTTP     | Snapserver JSON-RPC API + built-in web UI |

`mpd-bridge` listens on 8787. On the Pi it is published on port 8788 of the LAN (`BRIDGE_PORT` in `.env`), for `cloudflared` running on another machine.

---

## Audio pipeline

```
Music files
    └── MPD reads and decodes audio
         └── writes PCM → /shared/snapfifo  (named FIFO pipe)
              └── Snapserver reads the FIFO
                   └── streams to Snapcast clients (speakers, phones, other Pis, etc.)
```

The FIFO (`snapfifo`) is created by `entrypoint.sh` and lives in a named Docker volume (`shared-fifo`) mounted into both the `mpd` and `snapserver` containers.

### Snapcast clients

Install a [Snapcast client](https://github.com/badaix/snapcast#client) on any device (Linux, Android, macOS, Windows, Raspberry Pi) and connect it to port `1704` of your server. The `/snap` page in the web app shows all connected clients with individual volume and mute controls.

---

## Login

The whole app sits behind a single shared password, checked against `ADMIN_PASSWORD` in `.env`. Visiting any page while unauthenticated redirects to `/login`; entering the correct password sets an `httpOnly` cookie that never expires (log back out to clear it). Log out from the **Session** section on `/admin`, which hits `/logout` and clears the cookie.

`POST /api/assistant/voice` is exempt from this gate since it's called by an external device that authenticates with its own `X-Assistant-Token` header instead (see "AI assistant" below).

---

## Configuration files

| File                 | Description                                                             |
| -------------------- | ----------------------------------------------------------------------- |
| `.env`               | Runtime environment variables (see above)                               |
| `mpd.conf`           | MPD config — music dir, FIFO output to `snapfifo`, disables local audio |
| `snapserver.conf`    | Snapserver config — reads from `snapfifo`, sets stream name             |
| `docker-compose.yml` | Service definitions, volumes, port bindings                             |
| `entrypoint.sh`      | Creates the FIFO and starts MPD inside the container                    |

### Persisting MPD state

`docker-compose.yml` mounts `./mpd-db` and `./playlists` to persist the MPD database and playlists across container restarts. These directories are created automatically on first run.

---

## Deployment on Raspberry Pi

1. Install Docker:
   ```sh
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker $USER
   ```
2. Clone the repo and follow the **Quick start** steps above.
3. Set `ORIGIN` in `.env` to the Pi's LAN IP.
4. Start in detached mode: `docker compose up -d`

The `restart: unless-stopped` policy (set in `docker-compose.yml`) ensures the stack comes back up after a reboot.

## Deployment on a VPS

The app can run on a VPS while MPD, Snapcast and the music stay on the Pi. The two halves talk only through `mpd-bridge`, published by a Cloudflare tunnel and protected by Cloudflare Access plus the bridge's own token.

```
browser ──HTTPS──▶ Traefik (Dokploy) ─▶ svelte-mpd (VPS) ──HTTPS──▶ Cloudflare Access ─▶ tunnel ─▶ mpd-bridge (Pi) ─▶ MPD / Snapserver
```

1. **On the Pi:** run `docker-compose.pi.yml` (MPD, Snapcast, `mpd-bridge`) and point your own `cloudflared` tunnel at `http://<Pi's LAN IP>:8788`. The [bridge README](mpd-bridge/README.md#cloudflare-setup) covers the tunnel, the Access application and its service token.
2. **On the VPS (Dokploy):** point a DNS record at the VPS and create a _Compose_ service from `docker-compose.vps.yml`. Set the variables listed at the top of that file under _Environment_ (domain, bridge URL and token, `CF_ACCESS_CLIENT_ID`/`CF_ACCESS_CLIENT_SECRET`, admin password), and under _Domains_ add the domain for service `svelte-mpd`, port `3000`. Dokploy's Traefik handles HTTPS. `DOMAIN` must match that domain, since `ORIGIN` is built from it.

Images are pulled on every deploy (`pull_policy: always`), and app data lives in the `app-data` named volume so it survives redeploys. To deploy automatically, copy the service's deploy webhook URL from Dokploy into a `DOKPLOY_WEBHOOK_URL` repository secret: the CI calls it after publishing a new app image from `main`.

Article audio is generated on the VPS, and MPD on the Pi fetches it from `https://<domain>/audio/<id>`. Those URLs carry an HMAC signature so they work without the login cookie. Set `AUDIO_URL_SECRET` to sign them with a dedicated key; otherwise the key is derived from `ADMIN_PASSWORD`, and changing the password invalidates article audio already saved in playlists.

The voice client (`voice-client/`) must then point at the VPS URL instead of the Pi.

### Updating

Pull the new images and recreate the containers that changed:

```sh
docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d
```

The same applies to `docker-compose.pi.yml`. Playback is only interrupted if the `mpd` or `snapserver` images changed.

---

## AI assistant

An optional tool-calling assistant (Claude, via `@anthropic-ai/sdk`) that can control playback, search the library and adjust Snapserver rooms from natural language.

- `src/lib/server/assistant.ts` — the agent loop and tool implementations (wraps the same functions used by `mpd.remote.ts`/`snap.ts`, no separate MPD logic).
- `src/lib/assistant.remote.ts` — `sendMessage(text)` remote command, used by the `/assistant` text/voice chat page in the web app.
- `POST /api/assistant/voice` — binary endpoint (raw WAV body) for an external voice device (e.g. a wake-word client on a mic array): transcribes the audio, runs the agent, synthesizes the reply with Piper, and returns WAV audio. Requires an `X-Assistant-Token` header matching `ASSISTANT_TOKEN`.
- `POST /api/assistant/voice-ui` — same pipeline, used by the mic button on `/assistant`; no token check since it's only reachable from the app's own frontend.

Requires `ANTHROPIC_API_KEY` and `ASSISTANT_TOKEN`. Transcription uses OpenAI's cloud STT (`OPENAI_API_KEY`, model `gpt-4o-mini-transcribe`) when that key is set — recommended on weak hardware like a Raspberry Pi, since local Whisper inference (`Dockerfile.whisper`, `whisper_server.py`) is CPU-bound and ARM lacks the SIMD instructions `faster-whisper` is optimized for. Without `OPENAI_API_KEY`, it falls back to the local Whisper sidecar at `WHISPER_HOST`/`WHISPER_PORT`.

---

## Pages

| Route                | Description                                                    |
| -------------------- | -------------------------------------------------------------- |
| `/`                  | Current playback queue                                         |
| `/search`            | Full-library fuzzy search                                      |
| `/library`           | Music library filesystem browser                               |
| `/library/[...path]` | Nested directory navigation                                    |
| `/snap`              | Snapserver multi-room client volume control                    |
| `/admin`             | MPD database update, log out                                   |
| `/login`             | Password login screen (redirected here when not authenticated) |

### Player bar (persistent, all pages)

- Prev / play-pause / stop / next
- Seek bar — click or ←/→ keys (±5 s), interpolates locally between SSE updates
- Volume slider
- Random and repeat toggles
- Displays current song, artist, album, elapsed and total time

### `/` — Queue

- Lists the current MPD queue; active song highlighted
- Click track number → jump to that track (`playId`)
- Hover `✕` → remove from queue
- "Clear queue" button
- Updates in real time via SSE

### `/search` — Search

- Fuzzy + prefix search against a full in-memory MiniSearch index
- Results show title, artist, album, year, duration
- `+` button → add to queue (with ✓ confirmation)
- Index is built on server startup and rebuilt automatically after MPD database updates

### `/library` — Library browser

- Navigates MPD's virtual filesystem via `lsinfo`
- Clickable breadcrumb navigation
- Directories: click to enter, `+` to add entire folder to queue
- Files: `+` to add to queue, play-now to clear queue and play immediately
- `+ all` adds the entire current directory to the queue

### `/snap` — Snapserver

- Lists all connected Snapcast clients by name
- Per-client volume slider and mute toggle
- Updates in real time via SSE

### `/admin`

- **Update database** — triggers MPD `db.update()` to rescan the music directory
- **Reboot** / **Shutdown** — two-step confirmation before executing system commands

---

## Real-time events (SSE `/sse`)

| Event          | Payload                                                           |
| -------------- | ----------------------------------------------------------------- |
| `snapshot`     | Full state on connect (status, current song, queue, snap clients) |
| `player`       | Playback state, current song, elapsed time                        |
| `mixer`        | Volume level                                                      |
| `playlist`     | Queue changes                                                     |
| `options`      | Random, repeat, single, consume                                   |
| `snap_clients` | Snapserver client list                                            |
| `ping`         | Keepalive every 30 s                                              |

---

## Development

### Prerequisites

- [Bun](https://bun.sh) 1.3+
- A running MPD instance (local or remote) and [`mpd-bridge`](mpd-bridge/README.md) in front of it
- (Optional) A running Snapserver instance

### Setup

```sh
bun install
```

Create a `.env` file:

```env
MPD_BRIDGE_URL=http://127.0.0.1:8787
MPD_BRIDGE_TOKEN=<the bridge's API_TOKEN>
ORIGIN=http://localhost:3000
PORT=3000
```

If you enable the AI assistant locally, `PIPER_HOST`/`PIPER_PORT` (and `WHISPER_HOST`/`WHISPER_PORT`, unless `OPENAI_API_KEY` is set) need to point at running sidecars — `localhost` and their ports if you're running them yourself, or the Docker Compose service names otherwise.

### Scripts

| Command             | Description                           |
| ------------------- | ------------------------------------- |
| `bun dev`           | Start Vite dev server with hot reload |
| `bun run build`     | Production build                      |
| `bun start`         | Run the production build              |
| `bun run check`     | Svelte + TypeScript type check        |
| `bun run lint`      | Prettier + ESLint check               |
| `bun run format`    | Auto-format with Prettier             |
| `bun run test`      | Run tests once                        |
| `bun run test:unit` | Run tests in watch mode               |

### Architecture notes

- **mpd-bridge** — the app never speaks the MPD protocol. `src/lib/server/bridge.ts` calls the bridge's HTTP API and keeps one SSE connection to its `/events`, which `src/lib/server/mpd.ts` relays to the browsers and reconnects with backoff (and after 60 s of silence).
- **Remote functions** — SvelteKit's experimental `$app/server` `command`/`query` API is used instead of `+server.ts` routes, co-locating server logic with UI (`src/lib/mpd.remote.ts`).
- **Svelte 5 runes** — the global store (`MpdStore`) is a class with `$state` properties. No legacy store API (`src/lib/mpd.svelte.ts`).
- **In-memory MiniSearch index** — built from the bridge's `/library/all` when it connects, rebuilt on `database` events; the bridge answers `304` when the library hasn't changed.
- **Optimistic UI** — seek bar interpolates locally between SSE updates; Snapserver sliders update immediately without waiting for RPC confirmation.
- **SSE keepalive** — `X-Accel-Buffering: no` header prevents Nginx from buffering the event stream.
