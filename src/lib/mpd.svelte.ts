import type { SnapClient } from '$lib/mpd.types';
export type { SnapClient };

export interface CurrentSong {
	file: string;
	title?: string;
	artist?: string;
	album?: string;
}

export interface QueueItem {
	id: number;
	pos: number;
	file: string;
	title?: string;
	artist?: string;
	album?: string;
	duration?: number;
}

class MpdStore {
	connected = $state(false);
	playing = $state(false);
	paused = $state(false);
	stopped = $state(true);
	currentSong = $state<CurrentSong | null>(null);
	volume = $state(100);
	elapsed = $state(0);
	duration = $state(0);
	random = $state(false);
	repeat = $state(false);
	single = $state(false);
	consume = $state(false);
	queue = $state<QueueItem[]>([]);
	snapClients = $state<SnapClient[]>([]);

	reset() {
		this.connected = false;
		this.playing = false;
		this.paused = false;
		this.stopped = true;
		this.currentSong = null;
		this.volume = 100;
		this.elapsed = 0;
		this.duration = 0;
		this.random = false;
		this.repeat = false;
		this.single = false;
		this.consume = false;
		this.queue = [];
		this.snapClients = [];
	}
}

export const mpdStore = new MpdStore();
