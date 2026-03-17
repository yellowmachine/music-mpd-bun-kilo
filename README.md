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
- **Admin panel** — trigger MPD database rescan, reboot or shut down the host machine
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
| MPD client        | `mpd-api` (TCP)                        |
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
# MPD connection (use service name inside Docker Compose)
MPD_HOST=mpd
MPD_PORT=6600

# Snapserver connection (use service name inside Docker Compose)
SNAP_HOST=snapserver
SNAP_PORT=1780

# REQUIRED: LAN IP (or hostname) of the server + web app port
# Must be reachable by other devices on your network
ORIGIN=http://192.168.0.x:3000

# Web app port
PORT=3000
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
| snapserver | 1705 | TCP      | Snapcast control                          |
| snapserver | 1780 | HTTP     | Snapserver JSON-RPC API + built-in web UI |

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

The `/admin` page lets you reboot or shut down the Pi directly from the web UI. This requires the `svelte-mpd` container to run with `privileged: true` (already configured).

---

## Pages

| Route                | Description                                  |
| -------------------- | -------------------------------------------- |
| `/`                  | Current playback queue                       |
| `/search`            | Full-library fuzzy search                    |
| `/library`           | Music library filesystem browser             |
| `/library/[...path]` | Nested directory navigation                  |
| `/snap`              | Snapserver multi-room client volume control  |
| `/admin`             | MPD database update + system reboot/shutdown |

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
- A running MPD instance (local or remote)
- (Optional) A running Snapserver instance

### Setup

```sh
bun install
```

Create a `.env` file:

```env
MPD_HOST=localhost
MPD_PORT=6600
SNAP_HOST=localhost
SNAP_PORT=1780
ORIGIN=http://localhost:3000
PORT=3000
```

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

- **Two MPD TCP connections** — a dedicated idle connection listens for MPD subsystem events without blocking; a separate command connection handles all mutations. Both auto-reconnect with 5-second backoff (`src/lib/server/mpd.ts`).
- **Remote functions** — SvelteKit's experimental `$app/server` `command`/`query` API is used instead of `+server.ts` routes, co-locating server logic with UI (`src/lib/mpd.remote.ts`).
- **Svelte 5 runes** — the global store (`MpdStore`) is a class with `$state` properties. No legacy store API (`src/lib/mpd.svelte.ts`).
- **In-memory MiniSearch index** — built at startup from `mpd.db.listallinfo()`, rebuilt automatically on `system-database` MPD events.
- **Optimistic UI** — seek bar interpolates locally between SSE updates; Snapserver sliders update immediately without waiting for RPC confirmation.
- **SSE keepalive** — `X-Accel-Buffering: no` header prevents Nginx from buffering the event stream.
