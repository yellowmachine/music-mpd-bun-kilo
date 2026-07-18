import { command, query } from '$app/server';
import { error } from '@sveltejs/kit';
import {
	listFeeds,
	getFeedById,
	insertFeed,
	deleteFeed,
	fetchAndParseFeed,
	type PodcastFeedRow
} from '$lib/server/podcasts';

// --- Types ---

export interface PodcastFeed {
	id: number;
	feedUrl: string;
	title: string;
	imageUrl?: string;
	description?: string;
}

export interface PodcastEpisode {
	guid: string;
	title: string;
	description?: string;
	audioUrl: string;
	pubDate?: string;
	duration?: string;
}

function toFeed(row: PodcastFeedRow): PodcastFeed {
	return {
		id: row.id,
		feedUrl: row.feed_url,
		title: row.title,
		imageUrl: row.image_url ?? undefined,
		description: row.description ?? undefined
	};
}

// --- Queries ---

export const getSubscribedFeeds = query(async (): Promise<PodcastFeed[]> => {
	return listFeeds().map(toFeed);
});

export const getFeedEpisodes = query(
	'unchecked',
	async (id: number): Promise<{ feed: PodcastFeed; episodes: PodcastEpisode[] }> => {
		const row = getFeedById(id);
		if (!row) error(404, 'feed not found');

		const parsed = await fetchAndParseFeed(row.feed_url);
		return { feed: toFeed(row), episodes: parsed.episodes };
	}
);

// --- Commands ---

export const subscribeFeed = command('unchecked', async (feedUrl: string): Promise<PodcastFeed> => {
	const parsed = await fetchAndParseFeed(feedUrl);
	const row = insertFeed(feedUrl, parsed.title, parsed.imageUrl, parsed.description);
	return toFeed(row);
});

export const unsubscribeFeed = command('unchecked', async (id: number) => {
	deleteFeed(id);
});
