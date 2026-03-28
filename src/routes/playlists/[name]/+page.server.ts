import type { PageServerLoad } from './$types';
import { getClient } from '$lib/server/mpd';
import type { PlaylistSong } from '$lib/mpd.remote';

export const load: PageServerLoad = async ({ params }) => {
	const mpd = await getClient();
	const songs = await mpd.api.playlists.listinfo<PlaylistSong>(params.name);
	return { songs };
};
