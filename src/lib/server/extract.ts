import { extract } from '@extractus/article-extractor';
import { error } from '@sveltejs/kit';

const BLOCK_TAGS_CLOSE = /<\/(p|div|h[1-6]|li|blockquote|section|article)>/gi;
const BREAK_TAG = /<br\s*\/?>/gi;
const ANY_TAG = /<[^>]+>/g;

const ENTITIES: Record<string, string> = {
	'&amp;': '&',
	'&lt;': '<',
	'&gt;': '>',
	'&quot;': '"',
	'&#39;': "'",
	'&nbsp;': ' '
};

function htmlToText(html: string): string {
	const withBreaks = html
		.replace(BLOCK_TAGS_CLOSE, '\n\n')
		.replace(BREAK_TAG, '\n')
		.replace(ANY_TAG, '');

	const decoded = withBreaks.replace(
		/&amp;|&lt;|&gt;|&quot;|&#39;|&nbsp;/g,
		(entity) => ENTITIES[entity]
	);

	return decoded
		.replace(/[ \t]+/g, ' ')
		.replace(/\n{3,}/g, '\n\n')
		.trim();
}

export async function extractArticleText(url: string): Promise<{ title: string; text: string }> {
	const article = await extract(url).catch(() => null);
	if (!article?.content) error(422, 'could not extract article text');

	const text = htmlToText(article.content);
	if (!text) error(422, 'could not extract article text');

	return { title: article.title ?? url, text };
}

// Splits already-extracted plain text into TTS-sized segments, grouping
// consecutive paragraphs greedily up to maxChars. Never splits a single
// paragraph across two segments, even if it alone exceeds maxChars.
export function splitIntoSegments(text: string, maxChars = 1500): string[] {
	const paragraphs = text.split(/\n\n+/).filter((p) => p.trim().length > 0);
	const segments: string[] = [];
	let current = '';

	for (const paragraph of paragraphs) {
		if (current && current.length + 2 + paragraph.length > maxChars) {
			segments.push(current);
			current = paragraph;
		} else {
			current = current ? `${current}\n\n${paragraph}` : paragraph;
		}
	}
	if (current) segments.push(current);

	return segments;
}
