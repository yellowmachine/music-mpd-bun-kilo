import type { PageServerLoad } from './$types';
import { getPlaylists } from '$lib/server/bridge';

export const load: PageServerLoad = async () => {
	const playlists = await getPlaylists();
	return { playlists };
};
