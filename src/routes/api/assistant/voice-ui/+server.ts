import { handleVoiceMessage } from '$lib/server/assistant-voice';
import type { RequestHandler } from './$types';

// Same pipeline as /api/assistant/voice (used by the Raspberry Pi voice
// client) but without the ASSISTANT_TOKEN check, since this is only reachable
// from the app's own frontend — same trust boundary as the other remote
// functions, not an unauthenticated device on the LAN.
export const POST: RequestHandler = async ({ request }) => {
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
