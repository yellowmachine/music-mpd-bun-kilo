import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { error } from '@sveltejs/kit';
import { XMLParser } from 'fast-xml-parser';

const dataDir = './data';
const dataFile = `${dataDir}/article-feeds.json`;

export interface ArticleFeedRow {
	id: number;
	feed_url: string;
	title: string;
	added_at: number;
}

export interface ParsedFeedItem {
	guid: string;
	title: string;
	link: string;
	pubDate?: string;
}

function readFeeds(): ArticleFeedRow[] {
	try {
		return JSON.parse(readFileSync(dataFile, 'utf-8'));
	} catch {
		return [];
	}
}

function writeFeeds(feeds: ArticleFeedRow[]): void {
	mkdirSync(dataDir, { recursive: true });
	writeFileSync(dataFile, JSON.stringify(feeds, null, 2));
}

export function listFeeds(): ArticleFeedRow[] {
	return readFeeds().sort((a, b) => b.added_at - a.added_at);
}

export function getFeedById(id: number): ArticleFeedRow | undefined {
	return readFeeds().find((f) => f.id === id);
}

export function insertFeed(feedUrl: string, title: string): ArticleFeedRow {
	const feeds = readFeeds();
	if (feeds.some((f) => f.feed_url === feedUrl)) {
		error(409, 'already subscribed to this feed');
	}

	const row: ArticleFeedRow = {
		id: feeds.reduce((max, f) => Math.max(max, f.id), 0) + 1,
		feed_url: feedUrl,
		title,
		added_at: Date.now()
	};

	feeds.push(row);
	writeFeeds(feeds);
	return row;
}

export function deleteFeed(id: number): void {
	writeFeeds(readFeeds().filter((f) => f.id !== id));
}

// --- RSS fetch + parse (items only — titles/links, no article body) ---

const parser = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: '' });

function textOf(value: unknown): string | undefined {
	if (value == null) return undefined;
	if (typeof value === 'object')
		return (value as Record<string, unknown>)['#text'] as string | undefined;
	return String(value);
}

export async function fetchFeedTitleAndItems(
	feedUrl: string
): Promise<{ title: string; items: ParsedFeedItem[] }> {
	const res = await fetch(feedUrl, {
		headers: { 'User-Agent': 'svelte-mpd/1.0' },
		signal: AbortSignal.timeout(8000)
	});
	if (!res.ok) error(502, 'could not fetch feed');

	const xml = await res.text();
	const parsed = parser.parse(xml);
	const channel = parsed?.rss?.channel;
	if (!channel) error(422, 'not a valid RSS feed');

	const rawItems = channel.item
		? Array.isArray(channel.item)
			? channel.item
			: [channel.item]
		: [];

	const items: ParsedFeedItem[] = rawItems
		.map((item: Record<string, unknown>): ParsedFeedItem | null => {
			const link = textOf(item.link);
			if (!link) return null;

			return {
				guid: textOf(item.guid) ?? link,
				title: textOf(item.title) ?? 'untitled article',
				link,
				pubDate: textOf(item.pubDate)
			};
		})
		.filter((i: ParsedFeedItem | null): i is ParsedFeedItem => i !== null);

	return { title: textOf(channel.title) ?? feedUrl, items };
}
