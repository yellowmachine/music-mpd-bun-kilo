import type { PageServerLoad } from './$types';
import { getSnapshot } from '$lib/server/mpd';

export const load: PageServerLoad = async () => {
	const { queue } = await getSnapshot();
	return { queue };
};
