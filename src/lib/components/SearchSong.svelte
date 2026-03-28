<script lang="ts">
	import { PlayIcon, PlusIcon, FolderOpenIcon, BookmarkSimpleIcon } from 'phosphor-svelte';
	import { addToQueue, playNow } from '$lib/mpd.remote';
	import AddToPlaylistPopup from '$lib/components/AddToPlaylistPopup.svelte';
	import type { Song } from '$lib/server/mpd';

	interface Props {
		song: Song;
	}

	let { song }: Props = $props();

	// Parent directory path for library navigation
	// e.g. "Blur/The great escape/track.flac" → "/library/Blur/The great escape"
	const libraryHref = $derived.by(() => {
		const parts = song.file.split('/');
		parts.pop();
		return parts.length ? `/library/${parts.join('/')}` : '/library';
	});

	let adding = $state(false);
	let added = $state(false);
	let playing = $state(false);
	let showPlaylistPopup = $state(false);

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

	async function handlePlay() {
		if (playing) return;
		playing = true;
		try {
			await playNow(song.file);
		} finally {
			playing = false;
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

	<!-- Actions (visible on hover) -->
	<div
		class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100"
	>
		<!-- Play now -->
		<button
			onclick={handlePlay}
			disabled={playing}
			class="border border-[var(--color-border)] p-1 transition-colors
				hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
				disabled:opacity-40"
			aria-label="play now"
			title="Play now (clears queue)"
		>
			<PlayIcon size={11} weight="fill" />
		</button>

		<!-- Add to queue -->
		<button
			onclick={handleAdd}
			disabled={adding}
			class="border border-[var(--color-border)] p-1 transition-colors
				{added
				? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
				: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
				disabled:opacity-40"
			aria-label="add to queue"
			title="Add to queue"
		>
			{#if added}
				<span class="px-0.5 text-[10px]">✓</span>
			{:else}
				<PlusIcon size={11} weight="bold" />
			{/if}
		</button>

		<!-- Add to playlist -->
		<button
			onclick={() => (showPlaylistPopup = true)}
			class="border border-[var(--color-border)] p-1 transition-colors
				hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
			aria-label="add to playlist"
			title="Add to playlist"
		>
			<BookmarkSimpleIcon size={11} weight="bold" />
		</button>

		<!-- Navigate to library folder -->
		<a
			href={libraryHref}
			class="border border-[var(--color-border)] p-1 transition-colors
				hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
			aria-label="open in library"
			title="Open folder in library"
		>
			<FolderOpenIcon size={11} weight="bold" />
		</a>
	</div>

	{#if showPlaylistPopup}
		<AddToPlaylistPopup songUri={song.file} onclose={() => (showPlaylistPopup = false)} />
	{/if}
</li>
