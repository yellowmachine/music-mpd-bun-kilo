<script lang="ts">
	import { PlusIcon, TrashIcon } from 'phosphor-svelte';
	import {
		getSubscribedArticleFeeds,
		subscribeArticleFeed,
		unsubscribeArticleFeed
	} from '$lib/articles.remote';

	let feeds = $state(getSubscribedArticleFeeds());

	let feedUrl = $state('');
	let subscribing = $state(false);
	let error = $state<string | null>(null);

	async function handleSubscribe() {
		const url = feedUrl.trim();
		if (!url || subscribing) return;
		subscribing = true;
		error = null;
		try {
			await subscribeArticleFeed(url).updates(getSubscribedArticleFeeds());
			feedUrl = '';
		} catch (err) {
			error =
				(err as { body?: { message?: string } })?.body?.message || 'could not subscribe to feed';
		} finally {
			subscribing = false;
		}
	}

	async function handleUnsubscribe(id: number) {
		await unsubscribeArticleFeed(id).updates(getSubscribedArticleFeeds());
	}

	function hostnameOf(url: string): string {
		try {
			return new URL(url).hostname;
		} catch {
			return url;
		}
	}
</script>

<!-- Subscribe form -->
<div class="sticky top-0 z-10 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2">
	<div class="flex items-center gap-2">
		<input
			type="text"
			placeholder="paste news RSS feed URL..."
			bind:value={feedUrl}
			onkeydown={(e) => e.key === 'Enter' && handleSubscribe()}
			class="w-full bg-transparent text-xs outline-none placeholder:text-[var(--color-muted)]"
		/>
		<button
			onclick={handleSubscribe}
			disabled={!feedUrl.trim() || subscribing}
			class="flex shrink-0 items-center gap-1 border border-[var(--color-border)] px-2 py-1
				text-[10px] tracking-wider uppercase transition-colors
				hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
				disabled:opacity-40"
		>
			<PlusIcon size={11} weight="bold" />
			subscribe
		</button>
	</div>
	{#if error}
		<p class="pt-1 text-[10px] text-[var(--color-muted)]">{error}</p>
	{/if}
</div>

<!-- Subscribed feeds -->
{#await feeds}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">loading...</p>
{:then items}
	{#if items.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">
			— no news feeds subscribed —
		</p>
	{:else}
		<ul>
			{#each items as feed (feed.id)}
				<li
					class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
				>
					<a href="/articles/{feed.id}" class="min-w-0 flex-1">
						<p class="truncate text-xs leading-tight font-bold">{feed.title}</p>
						<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
							{hostnameOf(feed.feedUrl)}
						</p>
					</a>

					<button
						onclick={() => handleUnsubscribe(feed.id)}
						class="shrink-0 border border-[var(--color-border)] p-1 opacity-100 transition-colors
							hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)] sm:opacity-0 sm:group-hover:opacity-100"
						aria-label="unsubscribe"
						title="Unsubscribe"
					>
						<TrashIcon size={11} weight="bold" />
					</button>
				</li>
			{/each}
		</ul>
	{/if}
{:catch}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">failed to load feeds</p>
{/await}
