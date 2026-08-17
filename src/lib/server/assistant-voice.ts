import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { runAssistant } from '$lib/server/assistant';
import { synthesize } from '$lib/server/piper';

const host = env.WHISPER_HOST || 'localhost';
const port = env.WHISPER_PORT || '5001';

export interface VoiceResult {
	transcript: string;
	reply: string;
	wav: Buffer;
}

export async function handleVoiceMessage(audio: Buffer): Promise<VoiceResult> {
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

	return { transcript, reply, wav };
}
