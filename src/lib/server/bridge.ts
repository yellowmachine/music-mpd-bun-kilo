// HTTP client for mpd-bridge, the service on the Raspberry Pi that fronts MPD
// and Snapserver. See mpd-bridge/README.md for the API.
import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type {
	MpdStatus,
	MpdSong,
	MpdQueueItem,
	LibraryListing,
	LibraryFile,
	SnapClient
} from '$lib/mpd.types';

const DEFAULT_TIMEOUT_MS = 10_000;
const LIBRARY_TIMEOUT_MS = 90_000;

function baseUrl(): string {
	const url = env.MPD_BRIDGE_URL;
	if (!url) throw new Error('MPD_BRIDGE_URL is not set');
	return url.replace(/\/+$/, '');
}

function authHeaders(): Record<string, string> {
	const headers: Record<string, string> = { Authorization: `Bearer ${env.MPD_BRIDGE_TOKEN ?? ''}` };
	// Cloudflare Access service token, when the bridge is behind a tunnel.
	if (env.CF_ACCESS_CLIENT_ID && env.CF_ACCESS_CLIENT_SECRET) {
		headers['CF-Access-Client-Id'] = env.CF_ACCESS_CLIENT_ID;
		headers['CF-Access-Client-Secret'] = env.CF_ACCESS_CLIENT_SECRET;
	}
	return headers;
}

async function send(
	method: string,
	path: string,
	init: {
		body?: unknown;
		headers?: Record<string, string>;
		timeoutMs?: number;
		signal?: AbortSignal;
	} = {}
): Promise<Response> {
	const headers = { ...authHeaders(), ...init.headers };
	if (init.body !== undefined) headers['Content-Type'] = 'application/json';

	let res: Response;
	try {
		res = await fetch(baseUrl() + path, {
			method,
			headers,
			body: init.body === undefined ? undefined : JSON.stringify(init.body),
			signal: init.signal ?? AbortSignal.timeout(init.timeoutMs ?? DEFAULT_TIMEOUT_MS),
			// Cloudflare Access answers a rejected service token with a redirect
			// to its login page; following it would hide the failure.
			redirect: 'manual'
		});
	} catch (err) {
		console.error(`[bridge] ${method} ${path} failed:`, err);
		error(503, 'music server unreachable');
	}

	if (res.ok || res.status === 304) return res;

	if (res.status >= 300 && res.status < 400) {
		console.error(
			`[bridge] ${method} ${path}: redirected (Cloudflare Access rejected the service token?)`
		);
		error(502, 'music server rejected the request');
	}
	let message = res.statusText;
	try {
		message = ((await res.json()) as { error?: string }).error ?? message;
	} catch {
		// not JSON (e.g. an error page from the tunnel)
	}
	if (res.status === 401) {
		console.error('[bridge] 401: MPD_BRIDGE_TOKEN does not match the bridge API_TOKEN');
	}
	error(res.status, message);
}

async function json<T>(method: string, path: string, body?: unknown): Promise<T> {
	return (await send(method, path, { body })).json() as Promise<T>;
}

async function ok(method: string, path: string, body?: unknown): Promise<void> {
	await send(method, path, { body });
}

const seg = encodeURIComponent;

// --- Player ---

export interface PlayerState {
	status: MpdStatus;
	song: MpdSong | null;
}

export type TransportAction = 'play' | 'pause' | 'resume' | 'toggle' | 'stop' | 'next' | 'previous';

export const getPlayerState = () => json<PlayerState>('GET', '/status');
export const transport = (action: TransportAction) => ok('POST', `/player/${action}`);
export const playId = (id: number) => ok('POST', '/player/playid', { id });
export const seek = (seconds: number) => ok('POST', '/player/seek', { seconds });
export const setVolume = (value: number) =>
	ok('PUT', '/volume', { value: Math.round(Math.min(100, Math.max(0, value))) });

export const setOptions = (opts: {
	random?: boolean;
	repeat?: boolean;
	single?: boolean;
	consume?: boolean;
}) => ok('PUT', '/options', opts);

// --- Queue ---

export const getQueue = () => json<MpdQueueItem[]>('GET', '/queue');
export const clearQueue = () => ok('DELETE', '/queue');
export const deleteId = (id: number) => ok('DELETE', `/queue/${id}`);

/** Adds uris in one atomic step; `replace` clears the queue first, `play` starts playback. */
export const addToQueue = (uris: string[], opts: { replace?: boolean; play?: boolean } = {}) =>
	ok('POST', '/queue', { uris, replace: opts.replace ?? false, play: opts.play ?? false });

// --- Library ---

export const listInfo = (path: string) =>
	json<LibraryListing>('GET', `/library/ls?path=${seg(path)}`);

export const updateDatabase = () => json<{ job: number }>('POST', '/library/update');

export interface LibrarySong extends LibraryFile {
	albumArtist?: string;
}

/**
 * Fetches the whole library. Pass the ETag of the copy you already have to
 * get `null` back when the database hasn't changed since.
 */
export async function getAllSongs(
	etag: string | null
): Promise<{ etag: string | null; songs: LibrarySong[] } | null> {
	const res = await send('GET', '/library/all', {
		headers: etag ? { 'If-None-Match': etag } : {},
		timeoutMs: LIBRARY_TIMEOUT_MS
	});
	if (res.status === 304) return null;
	const body = (await res.json()) as { songs: LibrarySong[] };
	return { etag: res.headers.get('ETag'), songs: body.songs };
}

// --- Stored playlists ---

export interface StoredPlaylist {
	playlist: string;
	last_modified?: string;
}

export const getPlaylists = () => json<StoredPlaylist[]>('GET', '/playlists');
export const getPlaylistSongs = (name: string) =>
	json<LibrarySong[]>('GET', `/playlists/${seg(name)}`);
export const playlistAdd = (name: string, uri: string) =>
	ok('POST', `/playlists/${seg(name)}/songs`, { uri });
export const playlistDelete = (name: string, pos: number) =>
	ok('DELETE', `/playlists/${seg(name)}/songs/${pos}`);
export const playlistMove = (name: string, from: number, to: number) =>
	ok('POST', `/playlists/${seg(name)}/move`, { from, to });
export const playlistLoad = (name: string, opts: { replace?: boolean; play?: boolean } = {}) =>
	ok('POST', `/playlists/${seg(name)}/load`, {
		replace: opts.replace ?? false,
		play: opts.play ?? false
	});

// --- Snapcast ---

export const getSnapClients = () => json<SnapClient[]>('GET', '/snap/clients');
export const setSnapVolume = (id: string, volume: { percent?: number; muted?: boolean }) =>
	ok('PUT', `/snap/clients/${seg(id)}/volume`, volume);

// --- Events ---

export type BridgeEvent = { event: string; data: unknown };

/**
 * Opens the bridge's SSE stream and calls onEvent for each event until the
 * stream ends or `signal` aborts. onActivity fires on any bytes received,
 * including the bridge's 25 s heartbeat, so callers can detect a stalled
 * connection.
 */
export async function streamEvents(
	signal: AbortSignal,
	onEvent: (e: BridgeEvent) => void,
	onActivity: () => void
): Promise<void> {
	const res = await send('GET', '/events', {
		headers: { Accept: 'text/event-stream' },
		signal
	});
	if (!res.body) throw new Error('event stream has no body');

	const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
	let buffer = '';
	for (;;) {
		const { value, done } = await reader.read();
		if (done) return;
		onActivity();
		buffer += value.replace(/\r\n/g, '\n');

		let end: number;
		while ((end = buffer.indexOf('\n\n')) >= 0) {
			const block = buffer.slice(0, end);
			buffer = buffer.slice(end + 2);

			let event = 'message';
			const data: string[] = [];
			for (const line of block.split('\n')) {
				if (line.startsWith('event:')) event = line.slice(6).trim();
				else if (line.startsWith('data:')) data.push(line.slice(5).trimStart());
				// lines starting with ':' are comments (heartbeats)
			}
			if (data.length === 0) continue;
			try {
				onEvent({ event, data: JSON.parse(data.join('\n')) });
			} catch (err) {
				console.error(`[bridge] bad event '${event}':`, err);
			}
		}
	}
}
