import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { handleVoiceMessage } from '$lib/server/assistant-voice';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request }) => {
	if (!env.ASSISTANT_TOKEN || request.headers.get('x-assistant-token') !== env.ASSISTANT_TOKEN) {
		error(401, 'unauthorized');
	}

	const audio = Buffer.from(await request.arrayBuffer());
	const { transcript, reply, wav } = await handleVoiceMessage(
		audio,
		request.headers.get('content-type')
	);

	return new Response(Uint8Array.from(wav), {
		headers: {
			'Content-Type': 'audio/wav',
			'Content-Length': String(wav.length),
			'X-Transcript': encodeURIComponent(transcript),
			'X-Reply': encodeURIComponent(reply)
		}
	});
};
