import mpdApi from 'mpd-api';
import type { MPDApi } from 'mpd-api';
import MiniSearch from 'minisearch';
import { MPD_HOST, MPD_PORT as MPD_PORT_STR } from '$env/static/private';
import type { MpdStatus, MpdSong, MpdQueueItem } from '$lib/mpd.types';
export type { MpdStatus, MpdSong, MpdQueueItem } from '$lib/mpd.types';

const host = MPD_HOST ?? 'localhost';
const port = parseInt(MPD_PORT_STR ?? '6600', 10);

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

// --- Command connection (shared, used by remote functions) ---

let cmdClient: MPDApi.ClientAPI | null = null;
let cmdConnecting: Promise<MPDApi.ClientAPI> | null = null;

export async function getClient(): Promise<MPDApi.ClientAPI> {
	if (cmdClient) return cmdClient;
	if (cmdConnecting) return cmdConnecting;

	cmdConnecting = mpdApi.connect({ host, port }).then((c) => {
		cmdClient = c;
		cmdConnecting = null;

		c.on('close', () => {
			cmdClient = null;
			cmdConnecting = null;
		});
		c.on('error', () => {
			cmdClient = null;
			cmdConnecting = null;
		});

		return c;
	});

	return cmdConnecting;
}

// --- Idle connection (dedicated, listens for MPD subsystem events) ---

let idleClient: MPDApi.ClientAPI | null = null;

async function startIdle() {
	if (idleClient) return;

	try {
		idleClient = await mpdApi.connect({ host, port });
	} catch (err) {
		console.error('[mpd idle] Failed to connect:', err);
		idleClient = null;
		setTimeout(startIdle, 5000);
		return;
	}

	idleClient.on('system-player', async () => {
		try {
			const mpd = await getClient();
			const [status, song] = await Promise.all([
				mpd.api.status.get<MpdStatus>(),
				mpd.api.status.currentsong<MpdSong>()
			]);
			broadcast('player', { status, song });
		} catch {}
	});

	idleClient.on('system-mixer', async () => {
		try {
			const mpd = await getClient();
			const status = await mpd.api.status.get<MpdStatus>();
			broadcast('mixer', { volume: status.volume });
		} catch {}
	});

	idleClient.on('system-playlist', async () => {
		try {
			const mpd = await getClient();
			const queue = await mpd.api.queue.info<MpdQueueItem>();
			broadcast('playlist', { queue });
		} catch {}
	});

	idleClient.on('system-options', async () => {
		try {
			const mpd = await getClient();
			const status = await mpd.api.status.get<MpdStatus>();
			broadcast('options', {
				random: status.random,
				repeat: status.repeat,
				single: status.single,
				consume: status.consume
			});
		} catch {}
	});

	idleClient.on('system-database', () => {
		buildIndex();
	});

	idleClient.on('close', () => {
		idleClient = null;
		setTimeout(startIdle, 5000);
	});

	idleClient.on('error', () => {
		idleClient = null;
		setTimeout(startIdle, 5000);
	});
}

// Start idle connection eagerly at module load (skip during build)
if (!process.env.BUILDING) startIdle();

// --- Initial state snapshot (for new SSE clients) ---

export async function getSnapshot(): Promise<{
	status: MpdStatus;
	song: MpdSong | null;
	queue: MpdQueueItem[];
}> {
	const mpd = await getClient();
	const [status, song, queue] = await Promise.all([
		mpd.api.status.get<MpdStatus>(),
		mpd.api.status.currentsong<MpdSong>(),
		mpd.api.queue.info<MpdQueueItem>()
	]);
	return { status, song: song ?? null, queue };
}

// --- MiniSearch ---

let miniSearch: MiniSearch<Song> | null = null;
let searchState: SearchState = { ready: false, indexing: false, total: 0 };

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

// listallinfo returns a nested structure: [{ directory, file: [...songObjects] }, ...]
// This flattens all nested file arrays into a single list of song objects.
type RawDirEntry = {
	directory?: string;
	file?: RawFileEntry | RawFileEntry[];
};
type RawFileEntry = Record<string, unknown>;

function flattenListallinfo(entries: RawDirEntry[]): RawFileEntry[] {
	const result: RawFileEntry[] = [];
	for (const entry of entries) {
		if (!entry.file) continue;
		const files = Array.isArray(entry.file) ? entry.file : [entry.file];
		for (const f of files) {
			if (f && typeof f.file === 'string') result.push(f);
		}
	}
	return result;
}

async function buildIndex(): Promise<void> {
	if (searchState.indexing) return;
	searchState = { ready: false, indexing: true, total: 0 };

	try {
		const mpd = await getClient();
		const raw = await mpd.api.db.listallinfo<RawDirEntry>();
		const files = flattenListallinfo(raw);

		const documents: Song[] = files.map((f) => ({
			id: String(f.file),
			file: String(f.file),
			title: f.title ? String(f.title) : undefined,
			artist: f.artist ? String(f.artist) : undefined,
			album: f.album ? String(f.album) : undefined,
			albumArtist: f.albumartist ? String(f.albumartist) : undefined,
			track: f.track ? String(f.track) : undefined,
			date: f.date ? String(f.date) : undefined,
			duration: f.duration ? Number(f.duration) : undefined
		}));

		const index = createIndex();
		index.addAll(documents);
		miniSearch = index;
		searchState = { ready: true, indexing: false, total: documents.length };
		console.log(`[mpd] Search index built: ${documents.length} songs`);
	} catch (err) {
		console.error('[mpd] Failed to build search index:', err);
		searchState = { ready: false, indexing: false, total: 0 };
	}
}

// Build index once the idle connection is up (skip during build)
if (!process.env.BUILDING) startIdle().then(() => buildIndex());

export function getSearchState(): SearchState {
	return searchState;
}

export function search(query: string, limit = 20): Song[] {
	if (!miniSearch || !query.trim()) return [];
	return miniSearch.search(query).slice(0, limit) as unknown as Song[];
}
