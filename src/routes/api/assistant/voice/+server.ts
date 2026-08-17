import { error } from '@sveltejs/kit';
import { ASSISTANT_TOKEN } from '$env/static/private';
import { handleVoiceMessage } from '$lib/server/assistant-voice';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request }) => {
	if (!ASSISTANT_TOKEN || request.headers.get('x-assistant-token') !== ASSISTANT_TOKEN) {
		error(401, 'unauthorized');
	}

	const audio = Buffer.from(await request.arrayBuffer());
	const { transcript, reply, wav } = await handleVoiceMessage(audio);

	return new Response(Uint8Array.from(wav), {
		headers: {
			'Content-Type': 'audio/wav',
			'Content-Length': String(wav.length),
			'X-Transcript': encodeURIComponent(transcript),
			'X-Reply': encodeURIComponent(reply)
		}
	});
};
