<script lang="ts">
	import { browser } from '$app/environment';
	import { mpdStore } from '$lib/mpd.svelte';
	import QueueSong from '$lib/components/QueueSong.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// Seed the store with server-fetched queue on every navigation to this page.
	// SSE will keep it up to date afterwards.
	mpdStore.queue = data.queue;

	// Cover art toggle — persisted in localStorage
	let showCover = $state(browser ? localStorage.getItem('showCover') !== 'false' : true);

	function toggleCover() {
		showCover = !showCover;
		localStorage.setItem('showCover', String(showCover));
	}

	// Fetch cover URL from iTunes Search API when current song changes
	let coverUrl = $state<string | null>(null);

	$effect(() => {
		if (!showCover) return;
		const song = mpdStore.currentSong;
		if (!song) {
			coverUrl = null;
			return;
		}
		const q = [song.artist, song.album].filter(Boolean).join(' ');
		if (!q) {
			coverUrl = null;
			return;
		}
		fetch(
			`https://itunes.apple.com/search?term=${encodeURIComponent(q)}&media=music&entity=album&limit=1`
		)
			.then((r) => r.json())
			.then((d) => {
				const art = d.results?.[0]?.artworkUrl100;
				coverUrl = art ? art.replace('100x100bb', '600x600bb') : null;
			})
			.catch(() => {
				coverUrl = null;
			});
	});
</script>

<!-- Cover art + toggle -->
<div class="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2">
	<button
		onclick={toggleCover}
		class="text-[10px] tracking-widest uppercase transition-colors {showCover
			? 'text-[var(--color-fg)]'
			: 'text-[var(--color-muted)]'}"
		aria-label="toggle cover art"
	>
		cover
	</button>
</div>

{#if showCover && coverUrl && mpdStore.currentSong}
	<div class="flex justify-center border-b border-[var(--color-border)]">
		<img src={coverUrl} alt={mpdStore.currentSong.album ?? 'album art'} class="w-full max-w-sm" />
	</div>
{/if}

<ul class="divide-y-0">
	{#if mpdStore.queue.length === 0}
		<li class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— queue is empty —</li>
	{:else}
		{#each mpdStore.queue as song (song.id)}
			<QueueSong
				{song}
				active={mpdStore.currentSong?.file === song.file}
				playing={mpdStore.playing}
			/>
		{/each}
	{/if}
</ul>
