<script lang="ts">
	import { onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import {
		CaretLeftIcon,
		PlayIcon,
		PlusIcon,
		BookmarkSimpleIcon,
		SpeakerHighIcon
	} from 'phosphor-svelte';
	import {
		getFeedItems,
		generateArticle,
		getArticleProgress,
		type FeedItem,
		type ArticleSegment
	} from '$lib/articles.remote';
	import { addToQueue, playNow } from '$lib/mpd.remote';
	import AddToPlaylistPopup from '$lib/components/AddToPlaylistPopup.svelte';
	import { SvelteSet } from 'svelte/reactivity';

	const id = $derived(Number($page.params.id));
	const data = $derived(getFeedItems(id));

	// Per-item action state keyed by guid
	let generating = $state<string | null>(null);
	let progress = $state<{ ready: number; total: number } | null>(null);
	let playing = $state<string | null>(null);
	let adding = $state<string | null>(null);
	let playlistPopup = $state<string | null>(null);

	let cancelled = false;
	onDestroy(() => {
		cancelled = true;
	});

	function audioUrlFor(segmentId: number): string {
		return `${$page.url.origin}/audio/${segmentId}`;
	}

	function sortedReady(segments: ArticleSegment[]): ArticleSegment[] {
		return segments.filter((s) => s.status === 'ready').sort((a, b) => a.index - b.index);
	}

	async function handleListen(feedUrl: string, item: FeedItem) {
		if (generating) return;
		generating = item.guid;

		const queued = new SvelteSet<number>();
		let firstQueued = false;

		try {
			let segments = await generateArticle({
				feedUrl,
				guid: item.guid,
				title: item.title,
				link: item.link
			});

			while (!cancelled) {
				for (const seg of sortedReady(segments)) {
					if (queued.has(seg.id)) continue;
					if (!firstQueued) {
						await playNow(audioUrlFor(seg.id));
						firstQueued = true;
					} else {
						await addToQueue(audioUrlFor(seg.id));
					}
					queued.add(seg.id);
				}

				progress = {
					ready: segments.filter((s) => s.status === 'ready').length,
					total: segments.length
				};

				if (segments.every((s) => s.status === 'ready' || s.status === 'error')) break;

				await new Promise((r) => setTimeout(r, 2500));
				if (cancelled) break;
				segments = await getArticleProgress(item.guid);
			}
		} finally {
			generating = null;
			progress = null;
			await data.refresh();
		}
	}

	async function handlePlayAll(guid: string, segments: ArticleSegment[]) {
		if (playing) return;
		const ready = sortedReady(segments);
		if (ready.length === 0) return;
		playing = guid;
		try {
			await playNow(audioUrlFor(ready[0].id));
			for (const seg of ready.slice(1)) await addToQueue(audioUrlFor(seg.id));
		} finally {
			playing = null;
		}
	}

	async function handleAddAll(guid: string, segments: ArticleSegment[]) {
		if (adding) return;
		const ready = sortedReady(segments);
		if (ready.length === 0) return;
		adding = guid;
		try {
			for (const seg of ready) await addToQueue(audioUrlFor(seg.id));
			setTimeout(() => {
				if (adding === guid) adding = null;
			}, 2000);
		} catch {
			adding = null;
		}
	}

	function formatDate(pubDate?: string): string {
		if (!pubDate) return '';
		const d = new Date(pubDate);
		return isNaN(d.getTime()) ? '' : d.toLocaleDateString();
	}
</script>

<!-- Feed header -->
<div class="sticky top-0 z-10 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2">
	<div class="flex items-center gap-2">
		<a
			href="/articles"
			class="shrink-0 text-[var(--color-muted)] hover:text-[var(--color-fg)]"
			aria-label="back to articles"
		>
			<CaretLeftIcon size={14} weight="bold" />
		</a>

		{#await data then { feed }}
			<p class="min-w-0 flex-1 truncate text-xs leading-tight font-bold">{feed.title}</p>
		{/await}
	</div>
</div>

<!-- Items -->
{#await data}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">loading...</p>
{:then { feed, items }}
	{#if items.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— no articles found —</p>
	{:else}
		<ul>
			{#each items as item (item.guid)}
				{@const allReady =
					item.segments.length > 0 && item.segments.every((s) => s.status === 'ready')}
				<li
					class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
				>
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs leading-tight font-bold">{item.title}</p>
						<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
							{formatDate(item.pubDate)}
							{#if generating === item.guid && progress}
								· generating {progress.ready}/{progress.total}...
							{:else if !allReady && item.segments.some((s) => s.status === 'error')}
								· some segments failed
							{/if}
						</p>
					</div>

					<div
						class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
					>
						{#if allReady}
							<button
								onclick={() => handleAddAll(item.guid, item.segments)}
								disabled={adding !== null}
								class="border border-[var(--color-border)] p-1 transition-colors
									{adding === item.guid
									? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
									: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
									disabled:opacity-40"
								aria-label="add to queue"
								title="Add to queue"
							>
								{#if adding === item.guid}
									<span class="px-0.5 text-[10px]">✓</span>
								{:else}
									<PlusIcon size={11} weight="bold" />
								{/if}
							</button>

							<button
								onclick={() => handlePlayAll(item.guid, item.segments)}
								disabled={playing !== null}
								class="border border-[var(--color-border)] p-1 transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
									disabled:opacity-40"
								aria-label="play now"
								title="Play now"
							>
								<PlayIcon size={11} weight="fill" />
							</button>

							<button
								onclick={() => (playlistPopup = item.guid)}
								class="border border-[var(--color-border)] p-1 transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
								aria-label="add to playlist"
								title="Add to playlist"
							>
								<BookmarkSimpleIcon size={11} weight="bold" />
							</button>
						{:else}
							<button
								onclick={() => handleListen(feed.feedUrl, item)}
								disabled={generating !== null}
								class="flex items-center gap-1 border border-[var(--color-border)] px-2 py-1
									text-[10px] tracking-wider uppercase transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
									disabled:opacity-40"
							>
								<SpeakerHighIcon size={11} weight="bold" />
								{generating === item.guid
									? 'generating...'
									: item.segments.length > 0
										? 'resume'
										: 'listen'}
							</button>
						{/if}
					</div>

					{#if playlistPopup === item.guid && allReady}
						<AddToPlaylistPopup
							songUri={audioUrlFor(sortedReady(item.segments)[0].id)}
							onclose={() => (playlistPopup = null)}
						/>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{:catch err}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">failed to load articles: {err.message}</p>
{/await}
