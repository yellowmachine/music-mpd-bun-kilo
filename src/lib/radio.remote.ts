import { query } from '$app/server';

export interface RadioStation {
	stationuuid: string;
	name: string;
	url_resolved: string;
	country: string;
	countrycode: string;
	tags: string;
	bitrate: number;
	favicon: string;
	codec: string;
}

export const searchRadios = query('unchecked', async (q: string): Promise<RadioStation[]> => {
	const params = new URLSearchParams({
		name: q,
		limit: '30',
		hidebroken: 'true',
		order: 'votes',
		reverse: 'true'
	});
	const res = await fetch(`https://de1.api.radio-browser.info/json/stations/search?${params}`, {
		headers: { 'User-Agent': 'svelte-mpd/1.0' }
	});
	if (!res.ok) return [];
	return res.json();
});
