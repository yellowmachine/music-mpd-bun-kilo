import * as bridge from './bridge';
import type { SnapClient, SnapVolume } from '$lib/mpd.types';
export type { SnapClient, SnapVolume };

// Snapserver is reached through mpd-bridge. The client list is kept current
// by the bridge's snap_clients events (see mpd.ts), so reads are synchronous.

let clients: SnapClient[] = [];

export function getClients(): SnapClient[] {
	return clients;
}

export function setClients(list: SnapClient[]) {
	clients = list;
}

export async function setClientVolume(id: string, percent: number, muted?: boolean): Promise<void> {
	await bridge.setSnapVolume(id, { percent: Math.round(percent), muted });
}

export async function setClientMute(id: string, muted: boolean): Promise<void> {
	await bridge.setSnapVolume(id, { muted });
}
