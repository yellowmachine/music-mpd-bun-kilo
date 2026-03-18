<script lang="ts">
	import { mpdStore } from '$lib/mpd.svelte';
	import QueueSong from '$lib/components/QueueSong.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// Seed the store with server-fetched queue on every navigation to this page.
	// SSE will keep it up to date afterwards.
	mpdStore.queue = data.queue;
</script>

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
