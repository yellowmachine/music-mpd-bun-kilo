<script lang="ts">
	import { page } from '$app/stores';
	import {
		CaretLeftIcon,
		PlayIcon,
		PlusIcon,
		BookmarkSimpleIcon,
		SpeakerHighIcon
	} from 'phosphor-svelte';
	import { getFeedItems, generateArticle, type FeedItem } from '$lib/articles.remote';
	import { addToQueue, playNow } from '$lib/mpd.remote';
	import AddToPlaylistPopup from '$lib/components/AddToPlaylistPopup.svelte';

	const id = $derived(Number($page.params.id));
	const data = $derived(getFeedItems(id));

	// Per-item action state keyed by guid
	let generating = $state<string | null>(null);
	let playing = $state<string | null>(null);
	let adding = $state<string | null>(null);
	let playlistPopup = $state<string | null>(null);

	function audioUrlFor(articleId: number): string {
		return `${$page.url.origin}/audio/${articleId}`;
	}

	async function handleListen(feedUrl: string, item: FeedItem) {
		if (generating) return;
		generating = item.guid;
		try {
			await generateArticle({ feedUrl, guid: item.guid, title: item.title, link: item.link });
		} catch {
			// swallowed — item falls back to 'error' status below, user can retry
		} finally {
			generating = null;
			await data.refresh();
		}
	}

	async function handlePlay(articleId: number) {
		if (playing) return;
		playing = String(articleId);
		try {
			await playNow(audioUrlFor(articleId));
		} finally {
			playing = null;
		}
	}

	async function handleAdd(articleId: number) {
		if (adding) return;
		adding = String(articleId);
		try {
			await addToQueue(audioUrlFor(articleId));
			setTimeout(() => {
				if (adding === String(articleId)) adding = null;
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
				<li
					class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
				>
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs leading-tight font-bold">{item.title}</p>
						<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
							{formatDate(item.pubDate)}
							{#if item.status === 'error'}· could not generate audio{/if}
						</p>
					</div>

					<div
						class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
					>
						{#if item.status === 'ready' && item.articleId}
							<button
								onclick={() => handleAdd(item.articleId!)}
								disabled={adding !== null}
								class="border border-[var(--color-border)] p-1 transition-colors
									{adding === String(item.articleId)
									? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
									: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
									disabled:opacity-40"
								aria-label="add to queue"
								title="Add to queue"
							>
								{#if adding === String(item.articleId)}
									<span class="px-0.5 text-[10px]">✓</span>
								{:else}
									<PlusIcon size={11} weight="bold" />
								{/if}
							</button>

							<button
								onclick={() => handlePlay(item.articleId!)}
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
									: item.status === 'error'
										? 'retry'
										: 'listen'}
							</button>
						{/if}
					</div>

					{#if playlistPopup === item.guid && item.articleId}
						<AddToPlaylistPopup
							songUri={audioUrlFor(item.articleId)}
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
