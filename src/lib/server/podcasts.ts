import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { error } from '@sveltejs/kit';
import { XMLParser } from 'fast-xml-parser';

const dataDir = './data';
const dataFile = `${dataDir}/podcasts.json`;

// --- Types ---

export interface PodcastFeedRow {
	id: number;
	feed_url: string;
	title: string;
	image_url: string | null;
	description: string | null;
	added_at: number;
}

export interface ParsedEpisode {
	guid: string;
	title: string;
	description?: string;
	audioUrl: string;
	pubDate?: string;
	duration?: string;
}

export interface ParsedFeed {
	title: string;
	imageUrl?: string;
	description?: string;
	episodes: ParsedEpisode[];
}

// --- Storage (JSON file — works identically under `vite dev` and the
// production Bun runtime, unlike `bun:sqlite` which Vite's SSR module
// runner can't load in dev mode) ---

function readFeeds(): PodcastFeedRow[] {
	try {
		return JSON.parse(readFileSync(dataFile, 'utf-8'));
	} catch {
		return [];
	}
}

function writeFeeds(feeds: PodcastFeedRow[]): void {
	mkdirSync(dataDir, { recursive: true });
	writeFileSync(dataFile, JSON.stringify(feeds, null, 2));
}

// --- CRUD ---

export function listFeeds(): PodcastFeedRow[] {
	return readFeeds().sort((a, b) => b.added_at - a.added_at);
}

export function getFeedById(id: number): PodcastFeedRow | undefined {
	return readFeeds().find((f) => f.id === id);
}

export function insertFeed(
	feedUrl: string,
	title: string,
	imageUrl: string | undefined,
	description: string | undefined
): PodcastFeedRow {
	const feeds = readFeeds();
	if (feeds.some((f) => f.feed_url === feedUrl)) {
		error(409, 'already subscribed to this feed');
	}

	const row: PodcastFeedRow = {
		id: feeds.reduce((max, f) => Math.max(max, f.id), 0) + 1,
		feed_url: feedUrl,
		title,
		image_url: imageUrl ?? null,
		description: description ?? null,
		added_at: Date.now()
	};

	feeds.push(row);
	writeFeeds(feeds);
	return row;
}

export function deleteFeed(id: number): void {
	writeFeeds(readFeeds().filter((f) => f.id !== id));
}

// --- RSS fetch + parse ---

const parser = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: '' });

function textOf(value: unknown): string | undefined {
	if (value == null) return undefined;
	if (typeof value === 'object')
		return (value as Record<string, unknown>)['#text'] as string | undefined;
	return String(value);
}

export async function fetchAndParseFeed(feedUrl: string): Promise<ParsedFeed> {
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

	const episodes: ParsedEpisode[] = rawItems
		.map((item: Record<string, unknown>): ParsedEpisode | null => {
			const enclosure = item.enclosure as { url?: string } | undefined;
			const audioUrl = enclosure?.url;
			if (!audioUrl) return null;

			return {
				guid: textOf(item.guid) ?? audioUrl,
				title: textOf(item.title) ?? 'untitled episode',
				description: textOf(item.description),
				audioUrl,
				pubDate: textOf(item.pubDate),
				duration: textOf(item['itunes:duration'])
			};
		})
		.filter((e: ParsedEpisode | null): e is ParsedEpisode => e !== null);

	const itunesImage = channel['itunes:image'] as { href?: string } | undefined;

	return {
		title: textOf(channel.title) ?? feedUrl,
		imageUrl: textOf(channel.image?.url) ?? itunesImage?.href,
		description: textOf(channel.description),
		episodes
	};
}
