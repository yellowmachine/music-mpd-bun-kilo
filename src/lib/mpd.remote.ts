import { command, query } from '$app/server';
import { getClient, search as searchIndex, getSearchState } from '$lib/server/mpd';
import { setClientVolume, setClientMute, getClients } from '$lib/server/snap';
import type { Song } from '$lib/server/mpd';
import type { LibraryListing } from '$lib/mpd.types';

export type { Song };

// --- Types ---

export interface MpdStatus {
	state: 'play' | 'pause' | 'stop';
	volume: number;
	elapsed: number;
	duration: number;
	random: boolean;
	repeat: boolean;
	single: boolean;
	consume: boolean;
}

export interface MpdSong {
	file: string;
	title?: string;
	artist?: string;
	album?: string;
	track?: string;
	duration?: number;
}

export interface MpdQueueItem extends MpdSong {
	id: number;
	pos: number;
}

// --- Queries ---

export const getStatus = query(async () => {
	const mpd = await getClient();
	return mpd.api.status.get<MpdStatus>();
});

export const getCurrentSong = query(async () => {
	const mpd = await getClient();
	return mpd.api.status.currentsong<MpdSong>();
});

export const getQueue = query(async () => {
	const mpd = await getClient();
	return mpd.api.queue.info<MpdQueueItem>();
});

// --- Commands (no args) ---

export const play = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.play();
});

export const pause = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.pause();
});

export const resume = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.resume();
});

export const togglePlayback = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.toggle();
});

export const stop = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.stop();
});

export const next = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.next();
});

export const prev = command(async () => {
	const mpd = await getClient();
	await mpd.api.playback.prev();
});

export const clearQueue = command(async () => {
	const mpd = await getClient();
	await mpd.api.queue.clear();
});

// --- Commands (with args, using "unchecked") ---

export const playId = command('unchecked', async (id: number) => {
	const mpd = await getClient();
	await mpd.api.playback.playid(String(id));
});

export const setVolume = command('unchecked', async (volume: number) => {
	const mpd = await getClient();
	await mpd.api.playback.setvol(String(volume));
});

export const seek = command('unchecked', async (seconds: number) => {
	const mpd = await getClient();
	await mpd.api.playback.seekcur(String(seconds));
});

export const setRandom = command('unchecked', async (enabled: boolean) => {
	const mpd = await getClient();
	await mpd.api.playback.random(enabled ? '1' : '0');
});

export const setRepeat = command('unchecked', async (enabled: boolean) => {
	const mpd = await getClient();
	await mpd.api.playback.repeat(enabled ? '1' : '0');
});

export const addToQueue = command('unchecked', async (uri: string) => {
	const mpd = await getClient();
	await mpd.api.queue.add(uri);
});

export const playNow = command('unchecked', async (uri: string) => {
	const mpd = await getClient();
	await mpd.api.queue.clear();
	await mpd.api.queue.add(uri);
	await mpd.api.playback.play();
});

export const removeFromQueue = command('unchecked', async (id: number) => {
	const mpd = await getClient();
	await mpd.api.queue.deleteid(String(id));
});

// --- Library ---

export const lsinfo = query('unchecked', async (path: string): Promise<LibraryListing> => {
	const mpd = await getClient();
	const result = await mpd.api.db.lsinfo<{
		directory: { directory: string }[];
		file: Record<string, unknown>[];
		playlist: unknown[];
	}>(path || '');

	return {
		directories: (result.directory ?? []).map((d) => ({
			directory: d.directory,
			name: d.directory.split('/').at(-1) ?? d.directory
		})),
		files: (result.file ?? []).map((f) => ({
			file: String(f.file ?? ''),
			title: f.title ? String(f.title) : undefined,
			artist: f.artist ? String(f.artist) : undefined,
			album: f.album ? String(f.album) : undefined,
			track: f.track ? String(f.track) : undefined,
			date: f.date ? String(f.date) : undefined,
			duration: f.duration ? Number(f.duration) : undefined
		}))
	};
});

// --- Admin ---

export const mpdUpdate = command(async () => {
	const mpd = await getClient();
	await mpd.api.db.update();
	// buildIndex is triggered automatically via system-database SSE event
});

export const systemReboot = command(async () => {
	// Small delay so the response reaches the client before the machine goes down
	setTimeout(() => {
		const { spawn } = require('child_process') as typeof import('child_process');
		spawn('reboot', [], { detached: true, stdio: 'ignore' }).unref();
	}, 500);
});

export const systemShutdown = command(async () => {
	setTimeout(() => {
		const { spawn } = require('child_process') as typeof import('child_process');
		spawn('shutdown', ['now'], { detached: true, stdio: 'ignore' }).unref();
	}, 500);
});

// --- Snapserver ---

export const getSnapClients = query(async () => {
	return getClients();
});

export const snapSetVolume = command(
	'unchecked',
	async ({ id, percent }: { id: string; percent: number }) => {
		const clients = getClients();
		const client = clients.find((c) => c.id === id);
		await setClientVolume(id, percent, client?.volume.muted ?? false);
	}
);

export const snapSetMute = command(
	'unchecked',
	async ({ id, muted }: { id: string; muted: boolean }) => {
		await setClientMute(id, muted);
	}
);

// --- Playlists (stored) ---

export interface StoredPlaylist {
	playlist: string;
	last_modified?: string;
}

export interface PlaylistSong {
	file: string;
	title?: string;
	artist?: string;
	album?: string;
	track?: string;
	date?: string;
	duration?: number;
}

export const getPlaylists = query(async () => {
	const mpd = await getClient();
	return mpd.api.playlists.get<StoredPlaylist>();
});

export const getPlaylistSongs = query('unchecked', async (name: string) => {
	const mpd = await getClient();
	return mpd.api.playlists.listinfo<PlaylistSong>(name);
});

export const addSongToPlaylist = command(
	'unchecked',
	async ({ uri, playlist }: { uri: string; playlist: string }) => {
		const mpd = await getClient();
		await mpd.api.playlists.add(playlist, uri);
	}
);

export const addPlaylistToQueue = command('unchecked', async (name: string) => {
	const mpd = await getClient();
	await mpd.api.playlists.load(name);
});

export const playPlaylist = command('unchecked', async (name: string) => {
	const mpd = await getClient();
	await mpd.api.queue.clear();
	await mpd.api.playlists.load(name);
	await mpd.api.playback.play();
});

export const removeFromPlaylist = command(
	'unchecked',
	async ({ playlist, pos }: { playlist: string; pos: number }) => {
		const mpd = await getClient();
		await mpd.api.playlists.deleteAt(playlist, String(pos));
	}
);

export const moveInPlaylist = command(
	'unchecked',
	async ({ playlist, from, to }: { playlist: string; from: number; to: number }) => {
		const mpd = await getClient();
		await mpd.api.playlists.moveAt(playlist, String(from), String(to));
	}
);

// --- Search ---

export const searchSongs = query('unchecked', async (q: string) => {
	// Ensure client is initialized (triggers index build on first call)
	await getClient();
	return searchIndex(q);
});

export const getSearchStatus = query(async () => {
	await getClient();
	return getSearchState();
});
