import { error } from '@sveltejs/kit';
import { ASSISTANT_TOKEN, WHISPER_HOST, WHISPER_PORT } from '$env/static/private';
import { runAssistant } from '$lib/server/assistant';
import { synthesize } from '$lib/server/piper';
import type { RequestHandler } from './$types';

const host = WHISPER_HOST || 'localhost';
const port = WHISPER_PORT || '5001';

export const POST: RequestHandler = async ({ request }) => {
	if (!ASSISTANT_TOKEN || request.headers.get('x-assistant-token') !== ASSISTANT_TOKEN) {
		error(401, 'unauthorized');
	}

	const audio = Buffer.from(await request.arrayBuffer());
	if (!audio.length) error(400, 'empty audio');

	const sttRes = await fetch(`http://${host}:${port}/transcribe`, {
		method: 'POST',
		headers: { 'Content-Type': 'audio/wav' },
		body: Uint8Array.from(audio),
		signal: AbortSignal.timeout(60_000)
	});
	if (!sttRes.ok) error(502, 'stt failed');

	const { text: transcript } = (await sttRes.json()) as { text: string };
	if (!transcript?.trim()) error(400, 'empty transcript');

	const { text: reply } = await runAssistant(transcript);
	const wav = await synthesize(reply);

	return new Response(Uint8Array.from(wav), {
		headers: {
			'Content-Type': 'audio/wav',
			'Content-Length': String(wav.length),
			'X-Transcript': encodeURIComponent(transcript),
			'X-Reply': encodeURIComponent(reply)
		}
	});
};
