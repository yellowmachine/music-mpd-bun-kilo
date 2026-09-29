import MiniSearch from 'minisearch';
import * as bridge from './bridge';
import { setClients } from './snap';
import type { MpdStatus, MpdSong, MpdQueueItem, SnapClient } from '$lib/mpd.types';
export type { MpdStatus, MpdSong, MpdQueueItem } from '$lib/mpd.types';

// --- Types ---

export interface Song {
	id: string;
	file: string;
	title?: string;
	artist?: string;
	album?: string;
	albumArtist?: string;
	track?: string;
	date?: string;
	duration?: number;
}

export interface SearchState {
	ready: boolean;
	indexing: boolean;
	total: number;
}

// --- SSE subscribers ---

type SseEvent = { event: string; data: object };
type Subscriber = (e: SseEvent) => void;

const subscribers = new Set<Subscriber>();

export function broadcast(event: string, data: object) {
	for (const sub of subscribers) sub({ event, data });
}

export function subscribe(fn: Subscriber): () => void {
	subscribers.add(fn);
	return () => subscribers.delete(fn);
}

// --- Bridge event stream ---
//
// One long-lived connection to the bridge's /events, relayed to the
// browsers' /sse streams. The bridge already emits the event names and
// payloads the browsers expect (player, mixer, playlist, options,
// snap_clients), so most events pass straight through.

// The bridge sends a heartbeat every 25 s; silence for longer than this
// means the connection is dead even if TCP hasn't noticed.
const STALL_TIMEOUT_MS = 60_000;

let eventsStarted = false;

export function startEvents() {
	if (eventsStarted) return;
	eventsStarted = true;
	void eventLoop();
}

async function eventLoop() {
	let failures = 0;
	for (;;) {
		const ac = new AbortController();
		let stallTimer: ReturnType<typeof setTimeout> | undefined;
		const onActivity = () => {
			clearTimeout(stallTimer);
			stallTimer = setTimeout(() => ac.abort(new Error('stalled')), STALL_TIMEOUT_MS);
		};

		try {
			onActivity();
			await bridge.streamEvents(ac.signal, handleEvent, () => {
				failures = 0;
				onActivity();
			});
			console.warn('[bridge] event stream closed, reconnecting');
		} catch (err) {
			if (failures === 0) console.error('[bridge] event stream error:', err);
			failures++;
		} finally {
			clearTimeout(stallTimer);
			ac.abort();
		}

		broadcast('connection', { mpd: false, snap: false });
		const delay = Math.min(30_000, 1000 * 2 ** Math.min(failures, 5));
		await new Promise((r) => setTimeout(r, delay));
	}
}

function handleEvent({ event, data }: bridge.BridgeEvent) {
	switch (event) {
		case 'snapshot': {
			// Sent on every (re)connect: resync the browsers and the search index.
			const s = data as {
				status: MpdStatus | null;
				song: MpdSong | null;
				queue: MpdQueueItem[];
				snap_clients: SnapClient[];
				connection: { mpd: boolean; snap: boolean };
			};
			setClients(s.snap_clients);
			broadcast('snapshot', { status: s.status, song: s.song, queue: s.queue });
			broadcast('snap_clients', { clients: s.snap_clients });
			broadcast('connection', s.connection);
			if (s.connection.mpd) buildIndex();
			break;
		}
		case 'snap_clients':
			setClients((data as { clients: SnapClient[] }).clients);
			broadcast(event, data as object);
			break;
		case 'database':
			buildIndex();
			break;
		case 'connection':
			if ((data as { mpd: boolean }).mpd) buildIndex();
			broadcast(event, data as object);
			break;
		default:
			// player, mixer, playlist, options, stored_playlist
			broadcast(event, data as object);
	}
}

// --- Initial state snapshot (for new SSE clients) ---

export async function getSnapshot(): Promise<{
	status: MpdStatus;
	song: MpdSong | null;
	queue: MpdQueueItem[];
}> {
	const [{ status, song }, queue] = await Promise.all([bridge.getPlayerState(), bridge.getQueue()]);
	return { status, song, queue };
}

// --- MiniSearch ---

let miniSearch: MiniSearch<Song> | null = null;
let searchState: SearchState = { ready: false, indexing: false, total: 0 };
let libraryEtag: string | null = null;
let rebuildPending = false;

function createIndex(): MiniSearch<Song> {
	return new MiniSearch<Song>({
		idField: 'id',
		fields: ['title', 'artist', 'album', 'albumArtist', 'file'],
		storeFields: ['file', 'title', 'artist', 'album', 'albumArtist', 'track', 'date', 'duration'],
		searchOptions: {
			boost: { title: 3, artist: 2, album: 1.5 },
			fuzzy: 0.2,
			prefix: true
		}
	});
}

// Called on every bridge (re)connect and database change. The bridge answers
// 304 when the library hasn't changed, so this is cheap when nothing did.
async function buildIndex(): Promise<void> {
	if (searchState.indexing) {
		rebuildPending = true;
		return;
	}
	searchState = { ...searchState, indexing: true };

	try {
		const result = await bridge.getAllSongs(miniSearch ? libraryEtag : null);
		if (result) {
			const documents: Song[] = result.songs.map((f) => ({ id: f.file, ...f }));
			const index = createIndex();
			index.addAll(documents);
			miniSearch = index;
			libraryEtag = result.etag;
			searchState = { ready: true, indexing: false, total: documents.length };
			console.log(`[mpd] Search index built: ${documents.length} songs`);
		} else {
			searchState = { ...searchState, indexing: false };
		}
	} catch (err) {
		console.error('[mpd] Failed to build search index:', err);
		searchState = { ...searchState, indexing: false };
	}

	if (rebuildPending) {
		rebuildPending = false;
		void buildIndex();
	}
}

export function getSearchState(): SearchState {
	return searchState;
}

export function search(query: string, limit = 20): Song[] {
	if (!miniSearch || !query.trim()) return [];
	return miniSearch.search(query).slice(0, limit) as unknown as Song[];
}
