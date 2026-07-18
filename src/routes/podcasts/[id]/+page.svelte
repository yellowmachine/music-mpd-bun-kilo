<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		CaretLeftIcon,
		PlayIcon,
		PlusIcon,
		BookmarkSimpleIcon,
		TrashIcon
	} from 'phosphor-svelte';
	import {
		getFeedEpisodes,
		getSubscribedFeeds,
		unsubscribeFeed,
		type PodcastEpisode
	} from '$lib/podcasts.remote';
	import { addToQueue, playNow } from '$lib/mpd.remote';
	import AddToPlaylistPopup from '$lib/components/AddToPlaylistPopup.svelte';

	const id = $derived(Number($page.params.id));
	const data = $derived(getFeedEpisodes(id));

	// Per-episode action state keyed by guid
	let playing = $state<string | null>(null);
	let adding = $state<string | null>(null);
	let playlistPopup = $state<string | null>(null);
	let unsubscribing = $state(false);

	async function handlePlay(episode: PodcastEpisode) {
		if (playing) return;
		playing = episode.guid;
		try {
			await playNow(episode.audioUrl);
		} finally {
			playing = null;
		}
	}

	async function handleAdd(episode: PodcastEpisode) {
		if (adding) return;
		adding = episode.guid;
		try {
			await addToQueue(episode.audioUrl);
			setTimeout(() => {
				if (adding === episode.guid) adding = null;
			}, 2000);
		} catch {
			adding = null;
		}
	}

	async function handleUnsubscribe() {
		if (unsubscribing) return;
		unsubscribing = true;
		try {
			await unsubscribeFeed(id).updates(getSubscribedFeeds());
			goto('/podcasts');
		} catch {
			unsubscribing = false;
		}
	}

	function formatDate(pubDate?: string): string {
		if (!pubDate) return '';
		const d = new Date(pubDate);
		return isNaN(d.getTime()) ? '' : d.toLocaleDateString();
	}

	// itunes:duration is either "HH:MM:SS"/"MM:SS" or a plain seconds count
	function formatDuration(duration?: string): string {
		if (!duration) return '';
		if (duration.includes(':')) return duration;

		const totalSeconds = Number(duration);
		if (!Number.isFinite(totalSeconds)) return duration;

		const hours = Math.floor(totalSeconds / 3600);
		const minutes = Math.floor((totalSeconds % 3600) / 60);
		const seconds = Math.floor(totalSeconds % 60);
		const mm = hours > 0 ? String(minutes).padStart(2, '0') : String(minutes);
		const ss = String(seconds).padStart(2, '0');
		return hours > 0 ? `${hours}:${mm}:${ss}` : `${mm}:${ss}`;
	}
</script>

<!-- Feed header -->
<div class="sticky top-0 z-10 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2">
	<div class="flex items-center gap-2">
		<a
			href="/podcasts"
			class="shrink-0 text-[var(--color-muted)] hover:text-[var(--color-fg)]"
			aria-label="back to podcasts"
		>
			<CaretLeftIcon size={14} weight="bold" />
		</a>

		{#await data then { feed }}
			<div class="min-w-0 flex-1">
				<p class="truncate text-xs leading-tight font-bold">{feed.title}</p>
			</div>
			<button
				onclick={handleUnsubscribe}
				disabled={unsubscribing}
				class="shrink-0 border border-[var(--color-border)] p-1 transition-colors
					hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)] disabled:opacity-40"
				aria-label="unsubscribe"
				title="Unsubscribe"
			>
				<TrashIcon size={11} weight="bold" />
			</button>
		{/await}
	</div>
</div>

<!-- Episodes -->
{#await data}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">loading...</p>
{:then { episodes }}
	{#if episodes.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— no episodes found —</p>
	{:else}
		<ul>
			{#each episodes as episode (episode.guid)}
				<li
					class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
				>
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs leading-tight font-bold">{episode.title}</p>
						<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
							{formatDate(episode.pubDate)}
							{#if episode.duration}· {formatDuration(episode.duration)}{/if}
						</p>
					</div>

					<div
						class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
					>
						<button
							onclick={() => handleAdd(episode)}
							disabled={adding !== null}
							class="border border-[var(--color-border)] p-1 transition-colors
								{adding === episode.guid
								? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
								: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
								disabled:opacity-40"
							aria-label="add to queue"
							title="Add to queue"
						>
							{#if adding === episode.guid}
								<span class="px-0.5 text-[10px]">✓</span>
							{:else}
								<PlusIcon size={11} weight="bold" />
							{/if}
						</button>

						<button
							onclick={() => handlePlay(episode)}
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
							onclick={() => (playlistPopup = episode.guid)}
							class="border border-[var(--color-border)] p-1 transition-colors
								hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
							aria-label="add to playlist"
							title="Add to playlist"
						>
							<BookmarkSimpleIcon size={11} weight="bold" />
						</button>
					</div>

					{#if playlistPopup === episode.guid}
						<AddToPlaylistPopup songUri={episode.audioUrl} onclose={() => (playlistPopup = null)} />
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{:catch err}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">failed to load episodes: {err.message}</p>
{/await}
