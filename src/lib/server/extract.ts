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
