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
	listArticles,
	findByRef,
	insertArticle,
	saveAudio,
	markReady,
	markError,
	refFor,
	type ArticleRow
} from '$lib/server/articles';
import { extractArticleText } from '$lib/server/extract';
import { synthesize } from '$lib/server/piper';

// --- Types ---

export interface ArticleFeed {
	id: number;
	feedUrl: string;
	title: string;
}

export interface FeedItem {
	guid: string;
	title: string;
	link: string;
	pubDate?: string;
	articleId?: number;
	status?: 'pending' | 'ready' | 'error';
}

export interface GeneratedArticle {
	id: number;
	feedUrl: string;
	url: string;
	title: string;
	status: 'pending' | 'ready' | 'error';
}

function toFeed(row: ArticleFeedRow): ArticleFeed {
	return { id: row.id, feedUrl: row.feed_url, title: row.title };
}

function toGenerated(row: ArticleRow): GeneratedArticle {
	return { id: row.id, feedUrl: row.feed_url, url: row.url, title: row.title, status: row.status };
}

// --- Queries ---

export const getSubscribedArticleFeeds = query(async (): Promise<ArticleFeed[]> => {
	return listFeeds().map(toFeed);
});

export const getGeneratedArticles = query(async (): Promise<GeneratedArticle[]> => {
	return listArticles().map(toGenerated);
});

export const getFeedItems = query(
	'unchecked',
	async (id: number): Promise<{ feed: ArticleFeed; items: FeedItem[] }> => {
		const row = getFeedById(id);
		if (!row) error(404, 'feed not found');

		const parsed = await fetchFeedTitleAndItems(row.feed_url);
		const items: FeedItem[] = parsed.items.map((item: ParsedFeedItem) => {
			const existing = findByRef('rss', refFor(item.guid));
			return { ...item, articleId: existing?.id, status: existing?.status };
		});

		return { feed: toFeed(row), items };
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

export const generateArticle = command(
	'unchecked',
	async (item: {
		feedUrl: string;
		guid: string;
		title: string;
		link: string;
	}): Promise<GeneratedArticle> => {
		const ref = refFor(item.guid);
		const existing = findByRef('rss', ref);
		if (existing?.status === 'ready') return toGenerated(existing);

		const row =
			existing ??
			insertArticle({
				source: 'rss',
				source_ref: ref,
				feed_url: item.feedUrl,
				url: item.link,
				title: item.title
			});

		try {
			const { text } = await extractArticleText(item.link);
			const wav = await synthesize(text);
			const audioPath = saveAudio(row.id, wav);
			return toGenerated(markReady(row.id, audioPath));
		} catch (err) {
			markError(row.id);
			throw err;
		}
	}
);
