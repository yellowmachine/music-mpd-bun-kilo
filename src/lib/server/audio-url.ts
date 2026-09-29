import { createHmac, timingSafeEqual } from 'node:crypto';
import { env } from '$env/dynamic/private';

// Article audio is played by MPD on the Raspberry Pi, which fetches it from
// this app over the internet and has no session cookie. Each URL therefore
// carries an HMAC of the segment id, which hooks.server.ts accepts in place of
// the login cookie for that one file. The signature doesn't expire, so URLs
// saved in stored playlists keep working.

function secret(): string {
	// A dedicated secret lets you rotate ADMIN_PASSWORD without breaking audio
	// URLs already saved in playlists.
	if (env.AUDIO_URL_SECRET) return env.AUDIO_URL_SECRET;
	return env.ADMIN_PASSWORD ? `audio-url:${env.ADMIN_PASSWORD}` : '';
}

function signature(id: number): string {
	return createHmac('sha256', secret()).update(`audio:${id}`).digest('hex').slice(0, 32);
}

/** Absolute URL MPD can fetch the segment from, e.g. https://music.example.com/audio/12?sig=… */
export function audioUrl(id: number): string {
	const origin = (env.ORIGIN ?? '').replace(/\/+$/, '');
	return `${origin}/audio/${id}?sig=${signature(id)}`;
}

/** Whether `sig` is the valid signature for audio segment `id`. */
export function audioSignatureValid(id: number, sig: string | null): boolean {
	if (!sig || !secret()) return false;
	const a = Buffer.from(sig);
	const b = Buffer.from(signature(id));
	return a.length === b.length && timingSafeEqual(a, b);
}
