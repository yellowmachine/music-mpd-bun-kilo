import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { runAssistant } from '$lib/server/assistant';
import { synthesize } from '$lib/server/piper';

const whisperHost = env.WHISPER_HOST || 'localhost';
const whisperPort = env.WHISPER_PORT || '5001';
const language = env.WHISPER_LANGUAGE || 'es';

export interface VoiceResult {
	transcript: string;
	reply: string;
	wav: Buffer;
}

async function transcribeWithOpenAI(audio: Buffer): Promise<string> {
	const form = new FormData();
	form.append('file', new Blob([Uint8Array.from(audio)], { type: 'audio/wav' }), 'audio.wav');
	form.append('model', 'gpt-4o-mini-transcribe');
	form.append('language', language);

	const res = await fetch('https://api.openai.com/v1/audio/transcriptions', {
		method: 'POST',
		headers: { Authorization: `Bearer ${env.OPENAI_API_KEY}` },
		body: form,
		signal: AbortSignal.timeout(60_000)
	});
	if (!res.ok) error(502, 'stt failed');

	const { text } = (await res.json()) as { text: string };
	return text;
}

async function transcribeWithLocalWhisper(audio: Buffer): Promise<string> {
	const res = await fetch(`http://${whisperHost}:${whisperPort}/transcribe`, {
		method: 'POST',
		headers: { 'Content-Type': 'audio/wav' },
		body: Uint8Array.from(audio),
		signal: AbortSignal.timeout(60_000)
	});
	if (!res.ok) error(502, 'stt failed');

	const { text } = (await res.json()) as { text: string };
	return text;
}

export async function handleVoiceMessage(audio: Buffer): Promise<VoiceResult> {
	if (!audio.length) error(400, 'empty audio');

	const transcript = await (env.OPENAI_API_KEY
		? transcribeWithOpenAI(audio)
		: transcribeWithLocalWhisper(audio));
	if (!transcript?.trim()) error(400, 'empty transcript');

	const { text: reply } = await runAssistant(transcript);
	const wav = await synthesize(reply);

	return { transcript, reply, wav };
}
