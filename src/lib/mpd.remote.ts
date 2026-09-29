import { command, query } from '$app/server';
import * as bridge from '$lib/server/bridge';
import { search as searchIndex, getSearchState } from '$lib/server/mpd';
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
	return (await bridge.getPlayerState()).status;
});

export const getCurrentSong = query(async () => {
	return (await bridge.getPlayerState()).song;
});

export const getQueue = query(async () => {
	return bridge.getQueue();
});

// --- Commands (no args) ---

export const play = command(async () => bridge.transport('play'));
export const pause = command(async () => bridge.transport('pause'));
export const resume = command(async () => bridge.transport('resume'));
export const togglePlayback = command(async () => bridge.transport('toggle'));
export const stop = command(async () => bridge.transport('stop'));
export const next = command(async () => bridge.transport('next'));
export const prev = command(async () => bridge.transport('previous'));
export const clearQueue = command(async () => bridge.clearQueue());

// --- Commands (with args, using "unchecked") ---

export const playId = command('unchecked', async (id: number) => {
	await bridge.playId(id);
});

export const setVolume = command('unchecked', async (volume: number) => {
	await bridge.setVolume(volume);
});

export const seek = command('unchecked', async (seconds: number) => {
	await bridge.seek(seconds);
});

export const toggleRandom = command(async () => {
	const { status } = await bridge.getPlayerState();
	await bridge.setOptions({ random: !status.random });
});

export const toggleRepeat = command(async () => {
	const { status } = await bridge.getPlayerState();
	await bridge.setOptions({ repeat: !status.repeat });
});

export const toggleSingle = command(async () => {
	const { status } = await bridge.getPlayerState();
	await bridge.setOptions({ single: !status.single });
});

// An empty uri is the library root, which the bridge spells "/".
export const addToQueue = command('unchecked', async (uri: string) => {
	await bridge.addToQueue([uri || '/']);
});

export const playNow = command('unchecked', async (uri: string) => {
	await bridge.addToQueue([uri || '/'], { replace: true, play: true });
});

export const removeFromQueue = command('unchecked', async (id: number) => {
	await bridge.deleteId(id);
});

// --- Library ---

export const lsinfo = query('unchecked', async (path: string): Promise<LibraryListing> => {
	return bridge.listInfo(path || '');
});

// --- Admin ---

export const mpdUpdate = command(async () => {
	await bridge.updateDatabase();
	// The search index is rebuilt when the bridge reports the database change
});

// --- Snapserver ---

export const getSnapClients = query(async () => {
	return getClients();
});

export const snapSetVolume = command(
	'unchecked',
	async ({ id, percent }: { id: string; percent: number }) => {
		await setClientVolume(id, percent);
	}
);

export const snapSetMute = command(
	'unchecked',
	async ({ id, muted }: { id: string; muted: boolean }) => {
		await setClientMute(id, muted);
	}
);

// --- Playlists (stored) ---

export type StoredPlaylist = bridge.StoredPlaylist;

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
	return bridge.getPlaylists();
});

export const getPlaylistSongs = query('unchecked', async (name: string) => {
	return bridge.getPlaylistSongs(name);
});

export const addSongToPlaylist = command(
	'unchecked',
	async ({ uri, playlist }: { uri: string; playlist: string }) => {
		await bridge.playlistAdd(playlist, uri);
	}
);

export const addPlaylistToQueue = command('unchecked', async (name: string) => {
	await bridge.playlistLoad(name);
});

export const playPlaylist = command('unchecked', async (name: string) => {
	await bridge.playlistLoad(name, { replace: true, play: true });
});

export const removeFromPlaylist = command(
	'unchecked',
	async ({ playlist, pos }: { playlist: string; pos: number }) => {
		await bridge.playlistDelete(playlist, pos);
	}
);

export const moveInPlaylist = command(
	'unchecked',
	async ({ playlist, from, to }: { playlist: string; from: number; to: number }) => {
		await bridge.playlistMove(playlist, from, to);
	}
);

// --- Search ---

export const searchSongs = query('unchecked', async (q: string) => {
	return searchIndex(q);
});

export const getSearchStatus = query(async () => {
	return getSearchState();
});
