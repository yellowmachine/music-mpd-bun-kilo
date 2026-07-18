import { createReadStream, statSync } from 'node:fs';
import { Readable } from 'node:stream';
import { error } from '@sveltejs/kit';
import { getArticleById } from '$lib/server/articles';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, request }) => {
	const id = Number(params.id);
	const row = getArticleById(id);
	if (!row || row.status !== 'ready' || !row.audio_path) error(404, 'audio not found');

	const stat = statSync(row.audio_path);
	const range = request.headers.get('range');
	const match = range ? /bytes=(\d+)-(\d*)/.exec(range) : null;

	if (match) {
		const start = Number(match[1]);
		const end = match[2] ? Number(match[2]) : stat.size - 1;

		const stream = createReadStream(row.audio_path, { start, end });
		return new Response(Readable.toWeb(stream) as ReadableStream, {
			status: 206,
			headers: {
				'Content-Type': 'audio/wav',
				'Accept-Ranges': 'bytes',
				'Content-Range': `bytes ${start}-${end}/${stat.size}`,
				'Content-Length': String(end - start + 1)
			}
		});
	}

	const stream = createReadStream(row.audio_path);
	return new Response(Readable.toWeb(stream) as ReadableStream, {
		status: 200,
		headers: {
			'Content-Type': 'audio/wav',
			'Accept-Ranges': 'bytes',
			'Content-Length': String(stat.size)
		}
	});
};
