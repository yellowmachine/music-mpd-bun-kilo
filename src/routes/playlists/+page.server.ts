import type { PageServerLoad } from './$types';
import { getClient } from '$lib/server/mpd';

export const load: PageServerLoad = async () => {
	const mpd = await getClient();
	const playlists = await mpd.api.playlists.get<{ playlist: string; last_modified?: string }>();
	return { playlists };
};
