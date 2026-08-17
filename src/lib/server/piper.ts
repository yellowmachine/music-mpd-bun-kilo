import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

const host = env.PIPER_HOST || 'localhost';
const port = env.PIPER_PORT || '5000';

export async function synthesize(text: string): Promise<Buffer> {
	const res = await fetch(`http://${host}:${port}/synthesize`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ text }),
		signal: AbortSignal.timeout(300_000)
	});
	if (!res.ok) error(502, 'could not synthesize audio');

	return Buffer.from(await res.arrayBuffer());
}
