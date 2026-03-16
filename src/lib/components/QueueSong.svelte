<script lang="ts">
	import { MusicNoteIcon, XIcon } from 'phosphor-svelte';
	import { removeFromQueue, playId } from '$lib/mpd.remote';
	import type { MpdQueueItem } from '$lib/mpd.types';

	interface Props {
		song: MpdQueueItem;
		active: boolean;
		playing: boolean;
	}

	let { song, active, playing }: Props = $props();
</script>

<li
	class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 transition-colors
		{active ? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]' : 'hover:bg-[var(--color-fg)]/5'}"
>
	<!-- Position / now playing indicator -->
	<button
		onclick={() => playId(song.id)}
		class="flex w-5 shrink-0 items-center justify-center text-[10px] tabular-nums
			{active ? 'text-[var(--color-accent-fg)]' : 'text-[var(--color-muted)]'}"
		aria-label="play {song.title ?? song.file}"
	>
		{#if active && playing}
			<MusicNoteIcon size={11} weight="fill" />
		{:else}
			{song.pos + 1}
		{/if}
	</button>

	<!-- Song info -->
	<div class="min-w-0 flex-1">
		<p class="truncate text-xs leading-tight font-bold">
			{song.title ?? song.file}
		</p>
		{#if song.artist || song.album}
			<p
				class="truncate text-[10px] leading-tight {active
					? 'opacity-60'
					: 'text-[var(--color-muted)]'}"
			>
				{song.artist ?? ''}
				{#if song.artist && song.album}—{/if}
				{song.album ?? ''}
			</p>
		{/if}
	</div>

	<!-- Duration -->
	{#if song.duration}
		<span
			class="shrink-0 text-[10px] tabular-nums {active
				? 'opacity-60'
				: 'text-[var(--color-muted)]'}"
		>
			{Math.floor(song.duration / 60)}:{String(Math.floor(song.duration % 60)).padStart(2, '0')}
		</span>
	{/if}

	<!-- Remove -->
	<button
		onclick={() => removeFromQueue(song.id)}
		class="shrink-0 opacity-0 transition-opacity group-hover:opacity-100
			{active
			? 'text-[var(--color-accent-fg)]'
			: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
		aria-label="remove from queue"
	>
		<XIcon size={12} weight="bold" />
	</button>
</li>
