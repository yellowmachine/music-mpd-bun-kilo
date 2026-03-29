import type { PageServerLoad } from './$types';
import type { RadioStation } from '$lib/radio.remote';

export const load: PageServerLoad = async ({ fetch }) => {
	try {
		const res = await fetch(
			'https://de1.api.radio-browser.info/json/stations/topvote?limit=40&hidebroken=true',
			{ headers: { 'User-Agent': 'svelte-mpd/1.0' } }
		);
		const stations: RadioStation[] = await res.json();
		return { stations };
	} catch {
		return { stations: [] };
	}
};
