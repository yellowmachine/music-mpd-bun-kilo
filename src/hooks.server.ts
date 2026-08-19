import { redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { startIdle, buildIndex } from '$lib/server/mpd';
import { connectWebSocket } from '$lib/server/snap';
import { startPeriodicCheck } from '$lib/server/update-check';
import { AUTH_COOKIE, sessionToken } from '$lib/server/auth';
import type { Handle } from '@sveltejs/kit';

export async function init() {
	startIdle().then(() => buildIndex());
	connectWebSocket();
	startPeriodicCheck();
}

// Paths reachable without a session cookie: the login page itself, and the
// voice endpoint used by an external device that authenticates with its own
// X-Assistant-Token header instead.
const PUBLIC_PATHS = new Set(['/login', '/api/assistant/voice']);

export const handle: Handle = async ({ event, resolve }) => {
	const { pathname } = event.url;

	if (env.ADMIN_PASSWORD && !PUBLIC_PATHS.has(pathname)) {
		const cookie = event.cookies.get(AUTH_COOKIE);
		const authed = !!cookie && cookie === sessionToken(env.ADMIN_PASSWORD);

		if (!authed) {
			// SvelteKit rewrites `event.url` to the calling page's pathname for
			// remote function requests (query/command/form calls under
			// `/_app/remote/...`), and automatically reformats a thrown redirect
			// into the JSON envelope its client runtime expects — so this one
			// `redirect()` correctly handles both full page navigations and
			// remote function calls.
			redirect(303, '/login');
		}
	}

	const response = await resolve(event);

	if (pathname.startsWith('/_app/immutable/')) {
		// Hashed assets — safe to cache forever
		response.headers.set('cache-control', 'public, max-age=31536000, immutable');
	} else if (!pathname.includes('.')) {
		// HTML routes — never cache, always fetch fresh
		response.headers.set('cache-control', 'no-store');
	}

	return response;
};
