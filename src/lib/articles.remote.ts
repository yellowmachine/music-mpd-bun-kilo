import { command, query } from '$app/server';
import { error } from '@sveltejs/kit';
import {
	listFeeds,
	insertFeed,
	deleteFeed,
	getFeedById,
	fetchFeedTitleAndItems,
	type ArticleFeedRow,
	type ParsedFeedItem
} from '$lib/server/article-feeds';
import {
	findSegmentsByRef,
	insertArticle,
	saveAudio,
	markReady,
	markError,
	refFor,
	type ArticleRow
} from '$lib/server/articles';
import { extractArticleText, splitIntoSegments } from '$lib/server/extract';
import { synthesize } from '$lib/server/piper';

// --- Types ---

export interface ArticleFeed {
	id: number;
	feedUrl: string;
	title: string;
}

export interface ArticleSegment {
	id: number;
	index: number;
	status: 'pending' | 'ready' | 'error';
}

export interface FeedItem {
	guid: string;
	title: string;
	link: string;
	pubDate?: string;
	segments: ArticleSegment[];
}

function toFeed(row: ArticleFeedRow): ArticleFeed {
	return { id: row.id, feedUrl: row.feed_url, title: row.title };
}

function toSegment(row: ArticleRow): ArticleSegment {
	return { id: row.id, index: row.segment_index, status: row.status };
}

// --- Queries ---

export const getSubscribedArticleFeeds = query(async (): Promise<ArticleFeed[]> => {
	return listFeeds().map(toFeed);
});

export const getFeedItems = query(
	'unchecked',
	async (id: number): Promise<{ feed: ArticleFeed; items: FeedItem[] }> => {
		const row = getFeedById(id);
		if (!row) error(404, 'feed not found');

		const parsed = await fetchFeedTitleAndItems(row.feed_url);
		const items: FeedItem[] = parsed.items.map((item: ParsedFeedItem) => ({
			...item,
			segments: findSegmentsByRef('rss', refFor(item.guid)).map(toSegment)
		}));

		return { feed: toFeed(row), items };
	}
);

export const getArticleProgress = query(
	'unchecked',
	async (guid: string): Promise<ArticleSegment[]> => {
		return findSegmentsByRef('rss', refFor(guid)).map(toSegment);
	}
);

// --- Commands ---

export const subscribeArticleFeed = command(
	'unchecked',
	async (feedUrl: string): Promise<ArticleFeed> => {
		const parsed = await fetchFeedTitleAndItems(feedUrl);
		const row = insertFeed(feedUrl, parsed.title);
		return toFeed(row);
	}
);

export const unsubscribeArticleFeed = command('unchecked', async (id: number) => {
	deleteFeed(id);
});

async function generateSegment(row: ArticleRow, text: string): Promise<void> {
	try {
		const wav = await synthesize(text);
		const audioPath = saveAudio(row.id, wav);
		markReady(row.id, audioPath);
	} catch {
		markError(row.id);
	}
}

// Refs currently being synthesized in the background, so re-clicking
// "Listen" while generation is in flight doesn't spawn duplicate work.
const inFlight = new Set<string>();

async function continueGenerating(ref: string, chunks: string[]): Promise<void> {
	try {
		for (const [index, chunk] of chunks.entries()) {
			const row = findSegmentsByRef('rss', ref).find((s) => s.segment_index === index);
			if (!row || row.status === 'ready') continue;
			await generateSegment(row, chunk);
		}
	} finally {
		inFlight.delete(ref);
	}
}

export const generateArticle = command(
	'unchecked',
	async (item: {
		feedUrl: string;
		guid: string;
		title: string;
		link: string;
	}): Promise<ArticleSegment[]> => {
		const ref = refFor(item.guid);
		let segments = findSegmentsByRef('rss', ref);
		const allReady = segments.length > 0 && segments.every((s) => s.status === 'ready');

		if (allReady || inFlight.has(ref)) {
			return segments.map(toSegment);
		}

		inFlight.add(ref);

		const { text } = await extractArticleText(item.link);
		const chunks = splitIntoSegments(text);

		if (segments.length === 0) {
			segments = chunks.map((_, index) =>
				insertArticle({
					source: 'rss',
					source_ref: ref,
					segment_index: index,
					segment_count: chunks.length,
					feed_url: item.feedUrl,
					url: item.link,
					title: item.title
				})
			);
		}

		const firstPending = segments.find((s) => s.status !== 'ready');
		if (firstPending) {
			await generateSegment(firstPending, chunks[firstPending.segment_index]);
		}

		continueGenerating(ref, chunks);

		return findSegmentsByRef('rss', ref).map(toSegment);
	}
);
