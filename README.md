# svelte-mpd

SvelteKit frontend for [MPD](https://www.musicpd.org/) (Music Player Daemon). Built with Bun, Svelte 5 (runes), TailwindCSS v4, and a typewriter aesthetic.

## Stack

- **SvelteKit** with `adapter-node`
- **Bun** as runtime and package manager
- **Svelte 5** — runes only (`$state`, `$derived`, `$effect`)
- **TailwindCSS v4** (Vite plugin)
- **Phosphor Icons** (`phosphor-svelte` v3, `*Icon` naming)
- **mpd-api** (wraps `mpd2`) for MPD communication
- **MiniSearch** for fuzzy client-side search over the MPD database
- **SSE** (Server-Sent Events) for real-time state updates

## Architecture

### Server (`src/lib/server/mpd.ts`)

Two separate MPD TCP connections:

- **Command connection** (`getClient`) — shared, used by remote functions for playback control, queue management, etc.
- **Idle connection** (`startIdle`) — dedicated, listens to MPD subsystem events (`player`, `mixer`, `playlist`, `options`, `database`) and broadcasts to all connected SSE clients.

On first connection, a MiniSearch index is built from `listallinfo` in the background. It rebuilds automatically on `system-database` events.

### Real-time (SSE)

`/sse` endpoint (`src/routes/sse/+server.ts`):

- On connect: sends a `snapshot` event with current status, current song, and full queue
- Ongoing: forwards `player`, `mixer`, `playlist`, `options` broadcasts from the idle connection
- Keepalive: `ping` every 30s

The client (`+layout.svelte`) connects to `/sse` and writes directly to `mpdStore` (Svelte 5 class with `$state` properties).

### Remote Functions

`src/lib/mpd.remote.ts` — SvelteKit experimental remote functions (`$app/server`):

- **Queries**: `getStatus`, `getCurrentSong`, `getQueue`, `lsinfo`, `searchSongs`, `getSearchStatus`
- **Commands**: `play`, `pause`, `resume`, `togglePlayback`, `stop`, `next`, `prev`, `playId`, `setVolume`, `seek`, `setRandom`, `setRepeat`, `addToQueue`, `removeFromQueue`, `clearQueue`

### State

`src/lib/mpd.svelte.ts` — singleton `MpdStore` class with granular `$state` properties:
`connected`, `playing`, `paused`, `stopped`, `currentSong`, `volume`, `elapsed`, `duration`, `random`, `repeat`, `single`, `consume`, `queue`.

## Configuration

```env
# .env
MPD_HOST=localhost
MPD_PORT=6600
ORIGIN=http://192.168.0.169:3000   # required for remote functions from other devices
PORT=3000
```

## Routes

| Route                | Description                      |
| -------------------- | -------------------------------- |
| `/`                  | Current queue                    |
| `/search`            | Fuzzy search via MiniSearch      |
| `/library`           | Browse MPD filesystem (`lsinfo`) |
| `/library/[...path]` | Nested directory navigation      |
| `/sse`               | SSE endpoint (internal)          |

## Current functionality

### Player bar (all pages)

**Working:**

- prev / play / pause / stop / next
- Seek bar (click or ←/→ keyboard ±5s)
- Volume slider
- Random and repeat toggles with visual state
- Displays current song, artist, album, elapsed and total time

**Missing:**

- Elapsed time does not advance in real time — only updates on MPD `player` events (state changes). A client-side `setInterval` or a more frequent SSE `elapsed` event is needed for smooth progress.

### `/` — Queue

**Working:**

- Lists current queue; active song highlighted (inverted colours)
- Click position number → `playId` (jump to that song)
- Hover `✕` button → `removeFromQueue`
- List updates in real time via SSE on queue changes

**Missing:**

- No button to clear the entire queue
- No drag & drop reordering

### `/search` — Search

**Working:**

- Reactive text input, searches MiniSearch index (fuzzy + prefix)
- Results show title, artist, album, year, duration
- Hover `+` button → `addToQueue` with `✓` feedback for 2s

**Missing:**

- No direct play option (add to queue only)
- No loading indicator while the MiniSearch index is being built on startup (searches return empty until ready)

### `/library` — Library browser

**Working:**

- Directory navigation via MPD `lsinfo`
- Clickable breadcrumb to go up levels
- Directories: click to navigate in, hover `+` adds entire folder to queue
- Files: hover `+` adds song to queue
- `+ all` button in breadcrumb adds current directory to queue

**Missing:**

- No direct play option for files (add to queue only)
- No visual indication of which songs are already in the queue or currently playing

## Development

```sh
bun install
bun dev
```

## Build

```sh
bun run build
bun start
```
