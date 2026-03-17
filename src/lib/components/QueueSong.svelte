<script lang="ts">
	import { MusicNoteIcon, XIcon, PlayIcon } from 'phosphor-svelte';
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
	<span class="relative flex w-5 shrink-0 items-center justify-center">
		<!-- Position number: visible by default, hidden on hover (desktop only) -->
		{#if active && playing}
			<MusicNoteIcon size={11} weight="fill" class="group-hover:hidden" />
		{:else}
			<span
				class="text-[10px] tabular-nums group-hover:hidden
				{active ? 'text-[var(--color-accent-fg)]' : 'text-[var(--color-muted)]'}"
			>
				{song.pos + 1}
			</span>
		{/if}
		<!-- Play button: visible on hover (desktop), always visible on touch via active state -->
		<button
			onclick={() => playId(song.id)}
			class="absolute hidden items-center justify-center group-hover:flex
				{active
				? 'text-[var(--color-accent-fg)]'
				: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			aria-label="play {song.title ?? song.file}"
		>
			<PlayIcon size={13} weight="fill" />
		</button>
	</span>

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

	<!-- Play (mobile: always visible, desktop: hidden — covered by hover on position) -->
	<button
		onclick={() => playId(song.id)}
		class="shrink-0 sm:hidden
			{active ? 'text-[var(--color-accent-fg)]' : 'text-[var(--color-muted)]'}"
		aria-label="play {song.title ?? song.file}"
	>
		<PlayIcon size={13} weight="fill" />
	</button>

	<!-- Remove -->
	<button
		onclick={() => removeFromQueue(song.id)}
		class="shrink-0 opacity-0 transition-opacity group-hover:opacity-100 sm:block
			{active
			? 'text-[var(--color-accent-fg)]'
			: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
		aria-label="remove from queue"
	>
		<XIcon size={12} weight="bold" />
	</button>
</li>
