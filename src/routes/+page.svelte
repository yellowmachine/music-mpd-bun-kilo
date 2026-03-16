<script lang="ts">
	import { mpdStore } from '$lib/mpd.svelte';
	import QueueSong from '$lib/components/QueueSong.svelte';
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
