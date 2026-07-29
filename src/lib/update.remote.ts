import { command, query } from '$app/server';
import { env } from '$env/dynamic/private';
import { getUpdateStatus } from '$lib/server/update-check';

export const getUpdateInfo = query(async () => {
	return getUpdateStatus();
});

export const triggerUpdate = command(async () => {
	if (!env.WATCHTOWER_URL || !env.WATCHTOWER_TOKEN) {
		throw new Error('Watchtower is not configured (WATCHTOWER_URL / WATCHTOWER_TOKEN missing)');
	}

	const res = await fetch(`${env.WATCHTOWER_URL}/v1/update`, {
		headers: { Authorization: `Bearer ${env.WATCHTOWER_TOKEN}` }
	});

	if (!res.ok) {
		throw new Error(`Watchtower responded ${res.status}`);
	}
});
