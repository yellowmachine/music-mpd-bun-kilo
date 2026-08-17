import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { runAssistant } from '$lib/server/assistant';
import { synthesize } from '$lib/server/piper';

const whisperHost = env.WHISPER_HOST || 'localhost';
const whisperPort = env.WHISPER_PORT || '5001';
const language = env.WHISPER_LANGUAGE || 'es';

const EXT_BY_CONTENT_TYPE: Record<string, string> = {
	'audio/webm': 'webm',
	'audio/ogg': 'ogg',
	'audio/wav': 'wav',
	'audio/x-wav': 'wav',
	'audio/mpeg': 'mp3',
	'audio/mp4': 'mp4',
	'audio/m4a': 'm4a',
	'audio/flac': 'flac'
};

function extFromContentType(contentType: string | null): string {
	const type = (contentType ?? '').split(';')[0].trim().toLowerCase();
	return EXT_BY_CONTENT_TYPE[type] ?? 'wav';
}

export interface VoiceResult {
	transcript: string;
	reply: string;
	wav: Buffer;
}

async function transcribeWithOpenAI(audio: Buffer, contentType: string | null): Promise<string> {
	const type = contentType || 'audio/wav';
	const ext = extFromContentType(contentType);

	const form = new FormData();
	form.append('file', new Blob([Uint8Array.from(audio)], { type }), `audio.${ext}`);
	form.append('model', 'gpt-4o-mini-transcribe');
	form.append('language', language);

	const res = await fetch('https://api.openai.com/v1/audio/transcriptions', {
		method: 'POST',
		headers: { Authorization: `Bearer ${env.OPENAI_API_KEY}` },
		body: form,
		signal: AbortSignal.timeout(60_000)
	});
	if (!res.ok) {
		console.error('OpenAI transcription failed:', res.status, await res.text());
		error(502, 'stt failed');
	}

	const { text } = (await res.json()) as { text: string };
	return text;
}

async function transcribeWithLocalWhisper(
	audio: Buffer,
	contentType: string | null
): Promise<string> {
	const res = await fetch(`http://${whisperHost}:${whisperPort}/transcribe`, {
		method: 'POST',
		headers: { 'Content-Type': contentType || 'audio/wav' },
		body: Uint8Array.from(audio),
		signal: AbortSignal.timeout(60_000)
	});
	if (!res.ok) {
		console.error('Local whisper transcription failed:', res.status, await res.text());
		error(502, 'stt failed');
	}

	const { text } = (await res.json()) as { text: string };
	return text;
}

export async function handleVoiceMessage(
	audio: Buffer,
	contentType: string | null = null
): Promise<VoiceResult> {
	if (!audio.length) error(400, 'empty audio');

	const transcript = await (env.OPENAI_API_KEY
		? transcribeWithOpenAI(audio, contentType)
		: transcribeWithLocalWhisper(audio, contentType));
	if (!transcript?.trim()) error(400, 'empty transcript');

	const { text: reply } = await runAssistant(transcript);
	const wav = await synthesize(reply);

	return { transcript, reply, wav };
}
