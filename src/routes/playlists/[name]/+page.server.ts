import type { PageServerLoad } from './$types';
import { getPlaylistSongs } from '$lib/server/bridge';

export const load: PageServerLoad = async ({ params }) => {
	const songs = await getPlaylistSongs(params.name);
	return { songs };
};
