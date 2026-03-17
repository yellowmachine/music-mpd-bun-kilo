<script lang="ts">
	import { MagnifyingGlassIcon, XIcon } from 'phosphor-svelte';
	import { searchSongs, getSearchStatus } from '$lib/mpd.remote';
	import SearchSong from '$lib/components/SearchSong.svelte';

	let query = $state('');
	const results = $derived(query.trim() ? searchSongs(query) : null);
	const indexStatus = getSearchStatus();
</script>

<!-- Search input -->
<div class="sticky top-0 z-10 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2">
	<div class="flex items-center gap-2">
		<MagnifyingGlassIcon size={14} weight="bold" class="shrink-0 text-[var(--color-muted)]" />
		<input
			type="text"
			placeholder="artist, title, album..."
			bind:value={query}
			class="w-full bg-transparent text-xs outline-none placeholder:text-[var(--color-muted)]"
		/>
		{#if query}
			<button
				onclick={() => (query = '')}
				class="shrink-0 text-[var(--color-muted)] hover:text-[var(--color-fg)]"
			>
				<XIcon size={12} weight="bold" />
			</button>
		{/if}
	</div>

	<!-- Index status bar -->
	{#await indexStatus then status}
		{#if status.indexing}
			<p class="mt-1 text-[10px] text-[var(--color-muted)]">● indexing library...</p>
		{:else if !status.ready}
			<p class="mt-1 text-[10px] text-[var(--color-muted)]">○ search index not ready</p>
		{/if}
	{/await}
</div>

<!-- Results -->
{#if results === null}
	{#await indexStatus then status}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">
			{#if status.ready}
				— type to search · {status.total.toLocaleString()} songs —
			{:else if status.indexing}
				— building index, search available shortly —
			{:else}
				— type to search —
			{/if}
		</p>
	{/await}
{:else}
	{#await results}
		<p class="px-4 py-3 text-xs text-[var(--color-muted)]">searching...</p>
	{:then songs}
		{#if songs.length === 0}
			<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— no results —</p>
		{:else}
			<ul>
				{#each songs as song (song.file)}
					<SearchSong {song} />
				{/each}
			</ul>
		{/if}
	{:catch}
		<p class="px-4 py-3 text-xs text-[var(--color-muted)]">search failed</p>
	{/await}
{/if}
