import { startIdle, buildIndex } from '$lib/server/mpd';
import { connectWebSocket } from '$lib/server/snap';
import type { Handle } from '@sveltejs/kit';

export async function init() {
	startIdle().then(() => buildIndex());
	connectWebSocket();
}

export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);

	const { pathname } = event.url;

	if (pathname.startsWith('/_app/immutable/')) {
		// Hashed assets — safe to cache forever
		response.headers.set('cache-control', 'public, max-age=31536000, immutable');
	} else if (!pathname.includes('.')) {
		// HTML routes — never cache, always fetch fresh
		response.headers.set('cache-control', 'no-store');
	}

	return response;
};
