import { command, query } from '$app/server';
import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { getUpdateStatus } from '$lib/server/update-check';

export const getUpdateInfo = query(async () => {
	return await getUpdateStatus();
});

export const triggerUpdate = command(async () => {
	if (!env.WATCHTOWER_URL || !env.WATCHTOWER_TOKEN) {
		error(500, 'Watchtower is not configured (WATCHTOWER_URL / WATCHTOWER_TOKEN missing)');
	}

	let res: Response;
	try {
		res = await fetch(`${env.WATCHTOWER_URL}/v1/update`, {
			method: 'POST',
			headers: { Authorization: `Bearer ${env.WATCHTOWER_TOKEN}` }
		});
	} catch (e) {
		error(502, `Could not reach Watchtower: ${e instanceof Error ? e.message : String(e)}`);
	}

	if (!res.ok) {
		error(502, `Watchtower responded ${res.status}`);
	}
});
