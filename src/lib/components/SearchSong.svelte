<script lang="ts">
	import { PlusIcon } from 'phosphor-svelte';
	import { addToQueue } from '$lib/mpd.remote';
	import type { Song } from '$lib/server/mpd';

	interface Props {
		song: Song;
	}

	let { song }: Props = $props();

	let adding = $state(false);
	let added = $state(false);

	async function handleAdd() {
		if (adding) return;
		adding = true;
		try {
			await addToQueue(song.file);
			added = true;
			setTimeout(() => (added = false), 2000);
		} finally {
			adding = false;
		}
	}
</script>

<li
	class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
>
	<!-- Song info -->
	<div class="min-w-0 flex-1">
		<p class="truncate text-xs leading-tight font-bold">
			{song.title ?? song.file}
		</p>
		{#if song.artist || song.album}
			<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
				{song.artist ?? ''}
				{#if song.artist && song.album}—{/if}
				{song.album ?? ''}
				{#if song.date}
					<span class="opacity-60">({song.date})</span>
				{/if}
			</p>
		{/if}
	</div>

	<!-- Duration -->
	{#if song.duration}
		<span class="shrink-0 text-[10px] text-[var(--color-muted)] tabular-nums">
			{Math.floor(song.duration / 60)}:{String(Math.floor(song.duration % 60)).padStart(2, '0')}
		</span>
	{/if}

	<!-- Add to queue -->
	<button
		onclick={handleAdd}
		disabled={adding}
		class="shrink-0 border border-[var(--color-border)] px-2 py-0.5 text-[10px] tracking-wider uppercase
			opacity-0 transition-opacity group-hover:opacity-100
			{added
			? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
			: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}"
		aria-label="add to queue"
	>
		{#if added}
			✓
		{:else}
			<PlusIcon size={10} weight="bold" />
		{/if}
	</button>
</li>
