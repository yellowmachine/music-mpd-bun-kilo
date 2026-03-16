import type { RequestHandler } from './$types';
import { subscribe, getSnapshot } from '$lib/server/mpd';

export const GET: RequestHandler = async () => {
	const encoder = new TextEncoder();

	let unsubscribe: (() => void) | null = null;
	let interval: ReturnType<typeof setInterval> | null = null;

	const stream = new ReadableStream({
		async start(controller) {
			const send = (event: string, data: object) => {
				try {
					controller.enqueue(encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`));
				} catch {
					// client disconnected
				}
			};

			// Subscribe to broadcasts from the idle connection
			unsubscribe = subscribe(({ event, data }) => send(event, data));

			// Send current state snapshot to this client only
			try {
				const { status, song, queue } = await getSnapshot();
				send('snapshot', { status, song, queue });
			} catch {
				send('snapshot', { status: null, song: null, queue: [] });
			}

			interval = setInterval(() => send('ping', { timestamp: Date.now() }), 30000);
		},
		cancel() {
			unsubscribe?.();
			if (interval) clearInterval(interval);
		}
	});

	return new Response(stream, {
		headers: {
			'Content-Type': 'text/event-stream',
			'Cache-Control': 'no-cache',
			Connection: 'keep-alive',
			'X-Accel-Buffering': 'no'
		}
	});
};
