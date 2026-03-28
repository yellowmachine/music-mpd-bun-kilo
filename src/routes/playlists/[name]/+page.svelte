<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { page } from '$app/stores';
	import { PlusIcon, TrashIcon } from 'phosphor-svelte';
	import { addPlaylistToQueue, removeFromPlaylist } from '$lib/mpd.remote';

	let { data } = $props();

	const name = $derived($page.params.name);

	let addingToQueue = $state(false);
	let addedToQueue = $state(false);
	let removingPos = $state<number | null>(null);

	async function handleAddToQueue() {
		if (addingToQueue) return;
		addingToQueue = true;
		try {
			await addPlaylistToQueue(name);
			addedToQueue = true;
			setTimeout(() => (addedToQueue = false), 2000);
		} finally {
			addingToQueue = false;
		}
	}

	async function handleRemove(pos: number) {
		if (removingPos !== null) return;
		removingPos = pos;
		try {
			await removeFromPlaylist({ playlist: name, pos });
			await invalidateAll();
		} finally {
			removingPos = null;
		}
	}
</script>

<!-- Header -->
<div
	class="sticky top-0 z-10 flex items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2"
>
	<div class="flex items-center gap-2">
		<a
			href="/playlists"
			class="text-[10px] text-[var(--color-muted)] hover:text-[var(--color-fg)] transition-colors"
		>
			← playlists
		</a>
		<span class="text-[var(--color-muted)]/40 text-[10px]">/</span>
		<span class="text-xs font-bold">{name}</span>
	</div>

	<button
		onclick={handleAddToQueue}
		disabled={addingToQueue}
		class="flex items-center gap-1.5 border border-[var(--color-border)] px-2.5 py-1 text-[10px] tracking-wider uppercase transition-colors
			{addedToQueue
			? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
			: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
			disabled:opacity-40"
	>
		{#if addedToQueue}
			<span>✓</span>
			added
		{:else}
			<PlusIcon size={11} weight="bold" />
			add to queue
		{/if}
	</button>
</div>

<!-- Songs -->
{#if data.songs.length === 0}
	<p class="py-8 text-center text-xs text-[var(--color-muted)]">— playlist is empty —</p>
{:else}
	<ul>
		{#each data.songs as song, i}
			<li
				class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5"
			>
				<span class="w-5 shrink-0 text-right text-[10px] text-[var(--color-muted)] tabular-nums">
					{i + 1}
				</span>

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

				{#if song.duration}
					<span class="shrink-0 text-[10px] text-[var(--color-muted)] tabular-nums">
						{Math.floor(song.duration / 60)}:{String(Math.floor(song.duration % 60)).padStart(2, '0')}
					</span>
				{/if}

				<button
					onclick={() => handleRemove(i)}
					disabled={removingPos !== null}
					class="shrink-0 border border-[var(--color-border)] p-1 opacity-100 transition-colors
						hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
						disabled:opacity-40 sm:opacity-0 sm:group-hover:opacity-100"
					aria-label="remove from playlist"
					title="Remove from playlist"
				>
					<TrashIcon size={11} weight="bold" />
				</button>
			</li>
		{/each}
	</ul>
{/if}
