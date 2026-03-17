import { startIdle, buildIndex } from '$lib/server/mpd';
import { connectWebSocket } from '$lib/server/snap';

export async function init() {
	startIdle().then(() => buildIndex());
	connectWebSocket();
}
