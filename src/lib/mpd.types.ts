export interface SnapVolume {
	percent: number;
	muted: boolean;
}

export interface SnapClient {
	id: string;
	connected: boolean;
	name: string;
	host: string;
	ip: string;
	volume: SnapVolume;
}

export interface LibraryDirectory {
	directory: string; // full path from root, e.g. "Blur/The great escape"
	name: string; // last segment only, e.g. "The great escape"
}

export interface LibraryFile {
	file: string;
	title?: string;
	artist?: string;
	album?: string;
	track?: string;
	date?: string;
	duration?: number;
}

export interface LibraryListing {
	directories: LibraryDirectory[];
	files: LibraryFile[];
}

export interface MpdStatus {
	state: 'play' | 'pause' | 'stop';
	volume: number;
	elapsed?: number;
	duration?: number;
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
