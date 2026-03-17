<script lang="ts">
	import { page } from '$app/stores';
	import { CaretRightIcon, HouseIcon, PlusIcon, PlayIcon } from 'phosphor-svelte';
	import { lsinfo, addToQueue, playNow } from '$lib/mpd.remote';
	import LibraryDir from '$lib/components/LibraryDir.svelte';
	import LibrarySong from '$lib/components/LibrarySong.svelte';

	const mpdPath = $derived($page.params.path ?? '');
	const segments = $derived(mpdPath ? mpdPath.split('/') : []);
	const listing = $derived(lsinfo(mpdPath));

	let addingAll = $state(false);
	let addedAll = $state(false);
	let playingAll = $state(false);

	async function handleAddAll() {
		if (addingAll) return;
		addingAll = true;
		try {
			await addToQueue(mpdPath || '/');
			addedAll = true;
			setTimeout(() => (addedAll = false), 2000);
		} finally {
			addingAll = false;
		}
	}

	async function handlePlayAll() {
		if (playingAll) return;
		playingAll = true;
		try {
			await playNow(mpdPath);
		} finally {
			playingAll = false;
		}
	}
</script>

<!-- Breadcrumb + actions -->
<div
	class="sticky top-0 z-10 flex items-center gap-2 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2"
>
	<!-- Breadcrumb -->
	<nav class="flex min-w-0 flex-1 items-center gap-1 text-[10px]" aria-label="breadcrumb">
		<a
			href="/library"
			class="shrink-0 text-[var(--color-muted)] hover:text-[var(--color-fg)]"
			aria-label="library root"
		>
			<HouseIcon size={12} weight="bold" />
		</a>

		{#each segments as segment, i}
			<CaretRightIcon size={10} class="shrink-0 text-[var(--color-muted)]" />
			{#if i === segments.length - 1}
				<span class="truncate font-bold">{segment}</span>
			{:else}
				<a
					href="/library/{segments.slice(0, i + 1).join('/')}"
					class="truncate text-[var(--color-muted)] hover:text-[var(--color-fg)]"
				>
					{segment}
				</a>
			{/if}
		{/each}
	</nav>

	<!-- Actions (only when inside a directory) -->
	{#if mpdPath}
		<div class="flex shrink-0 items-center gap-1">
			<!-- Play all -->
			<button
				onclick={handlePlayAll}
				disabled={playingAll}
				class="flex items-center gap-1 border border-[var(--color-border)] px-2 py-0.5
					text-[10px] tracking-wider uppercase transition-colors
					hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
					disabled:opacity-40"
				title="Clear queue and play this directory"
			>
				<PlayIcon size={10} weight="fill" />
				play
			</button>

			<!-- Add all -->
			<button
				onclick={handleAddAll}
				disabled={addingAll}
				class="flex items-center gap-1 border border-[var(--color-border)] px-2 py-0.5
					text-[10px] tracking-wider uppercase transition-colors
					{addedAll
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
					disabled:opacity-40"
			>
				{#if addedAll}
					✓ added
				{:else}
					<PlusIcon size={10} weight="bold" /> all
				{/if}
			</button>
		</div>
	{/if}
</div>

<!-- Listing -->
{#await listing}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">loading...</p>
{:then data}
	{#if data.directories.length === 0 && data.files.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— empty —</p>
	{:else}
		<ul>
			{#each data.directories as dir (dir.directory)}
				<LibraryDir {dir} href="/library/{dir.directory}" />
			{/each}
			{#each data.files as song (song.file)}
				<LibrarySong {song} />
			{/each}
		</ul>
	{/if}
{:catch err}
	<p class="px-4 py-3 text-xs text-[var(--color-muted)]">failed to load: {err.message}</p>
{/await}
