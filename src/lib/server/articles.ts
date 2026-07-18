import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';

const dataDir = './data';
const dataFile = `${dataDir}/articles.json`;
const audioDir = `${dataDir}/audio`;

export interface ArticleRow {
	id: number;
	source: 'rss';
	source_ref: string;
	feed_url: string;
	url: string;
	title: string;
	audio_path: string | null;
	status: 'pending' | 'ready' | 'error';
	created_at: number;
}

function readArticles(): ArticleRow[] {
	try {
		return JSON.parse(readFileSync(dataFile, 'utf-8'));
	} catch {
		return [];
	}
}

function writeArticles(articles: ArticleRow[]): void {
	mkdirSync(dataDir, { recursive: true });
	writeFileSync(dataFile, JSON.stringify(articles, null, 2));
}

export function refFor(guidOrLink: string): string {
	return createHash('sha256').update(guidOrLink).digest('hex').slice(0, 16);
}

export function listArticles(): ArticleRow[] {
	return readArticles().sort((a, b) => b.created_at - a.created_at);
}

export function getArticleById(id: number): ArticleRow | undefined {
	return readArticles().find((a) => a.id === id);
}

export function findByRef(source: ArticleRow['source'], ref: string): ArticleRow | undefined {
	return readArticles().find((a) => a.source === source && a.source_ref === ref);
}

export function insertArticle(row: {
	source: ArticleRow['source'];
	source_ref: string;
	feed_url: string;
	url: string;
	title: string;
}): ArticleRow {
	const articles = readArticles();
	const newRow: ArticleRow = {
		id: articles.reduce((max, a) => Math.max(max, a.id), 0) + 1,
		...row,
		audio_path: null,
		status: 'pending',
		created_at: Date.now()
	};

	articles.push(newRow);
	writeArticles(articles);
	return newRow;
}

export function saveAudio(id: number, wav: Buffer): string {
	mkdirSync(audioDir, { recursive: true });
	const audioPath = `${audioDir}/${id}.wav`;
	writeFileSync(audioPath, wav);
	return audioPath;
}

export function markReady(id: number, audioPath: string): ArticleRow {
	const articles = readArticles();
	const row = articles.find((a) => a.id === id);
	if (!row) throw new Error('article not found');

	row.audio_path = audioPath;
	row.status = 'ready';
	writeArticles(articles);
	return row;
}

export function markError(id: number): void {
	const articles = readArticles();
	const row = articles.find((a) => a.id === id);
	if (!row) return;

	row.status = 'error';
	writeArticles(articles);
}
