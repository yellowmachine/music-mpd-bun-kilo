<script lang="ts">
	import { MagnifyingGlassIcon, XIcon, PlayIcon, PlusIcon, BookmarkSimpleIcon } from 'phosphor-svelte';
	import { searchRadios } from '$lib/radio.remote';
	import { addToQueue, playNow } from '$lib/mpd.remote';
	import AddToPlaylistPopup from '$lib/components/AddToPlaylistPopup.svelte';
	import type { RadioStation } from '$lib/radio.remote';

	let { data } = $props();

	let query = $state('');
	const results = $derived(query.trim().length >= 2 ? searchRadios(query) : null);

	// Per-station action state keyed by stationuuid
	let playing = $state<string | null>(null);
	let adding = $state<string | null>(null);
	let playlistPopup = $state<string | null>(null); // stationuuid of open popup

	async function handlePlay(station: RadioStation) {
		if (playing) return;
		playing = station.stationuuid;
		try {
			await playNow(station.url_resolved);
		} finally {
			playing = null;
		}
	}

	async function handleAdd(station: RadioStation) {
		if (adding) return;
		adding = station.stationuuid;
		try {
			await addToQueue(station.url_resolved);
			setTimeout(() => {
				if (adding === station.stationuuid) adding = null;
			}, 2000);
		} catch {
			adding = null;
		}
	}
</script>

<!-- Search input -->
<div class="sticky top-0 z-10 border-b border-[var(--color-border)] bg-[var(--color-bg)] px-4 py-2">
	<div class="flex items-center gap-2">
		<MagnifyingGlassIcon size={14} weight="bold" class="shrink-0 text-[var(--color-muted)]" />
		<input
			type="text"
			placeholder="search stations..."
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
</div>

<!-- Results or top stations -->
{#if results === null}
	<!-- Top stations -->
	{#if data.stations.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— could not load stations —</p>
	{:else}
		<p class="px-4 py-2 text-[10px] text-[var(--color-muted)]">— top stations —</p>
		<ul>
			{#each data.stations as station (station.stationuuid)}
				<li class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5">
					<!-- Favicon -->
					<div class="flex h-6 w-6 shrink-0 items-center justify-center">
						{#if station.favicon}
							<img
								src={station.favicon}
								alt=""
								class="h-5 w-5 object-contain"
								onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
							/>
						{/if}
					</div>

					<!-- Info -->
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs font-bold leading-tight">{station.name}</p>
						<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
							{station.countrycode}
							{#if station.bitrate}· {station.bitrate}kbps{/if}
							{#if station.codec}· {station.codec}{/if}
							{#if station.tags}· <span class="opacity-70">{station.tags.split(',').slice(0, 3).join(', ')}</span>{/if}
						</p>
					</div>

					<!-- Actions -->
					<div class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
						<button
							onclick={() => handleAdd(station)}
							disabled={adding !== null}
							class="border border-[var(--color-border)] p-1 transition-colors
								{adding === station.stationuuid
								? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
								: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
								disabled:opacity-40"
							aria-label="add to queue"
							title="Add to queue"
						>
							{#if adding === station.stationuuid}
								<span class="px-0.5 text-[10px]">✓</span>
							{:else}
								<PlusIcon size={11} weight="bold" />
							{/if}
						</button>

						<button
							onclick={() => handlePlay(station)}
							disabled={playing !== null}
							class="border border-[var(--color-border)] p-1 transition-colors
								hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
								disabled:opacity-40"
							aria-label="play now"
							title="Play now"
						>
							<PlayIcon size={11} weight="fill" />
						</button>

						<button
							onclick={() => (playlistPopup = station.stationuuid)}
							class="border border-[var(--color-border)] p-1 transition-colors
								hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
							aria-label="add to playlist"
							title="Add to playlist"
						>
							<BookmarkSimpleIcon size={11} weight="bold" />
						</button>
					</div>

					{#if playlistPopup === station.stationuuid}
						<AddToPlaylistPopup
							songUri={station.url_resolved}
							onclose={() => (playlistPopup = null)}
						/>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{:else}
	<!-- Search results -->
	{#await results}
		<p class="px-4 py-3 text-xs text-[var(--color-muted)]">searching...</p>
	{:then stations}
		{#if stations.length === 0}
			<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">— no stations found —</p>
		{:else}
			<ul>
				{#each stations as station (station.stationuuid)}
					<li class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5">
						<div class="flex h-6 w-6 shrink-0 items-center justify-center">
							{#if station.favicon}
								<img
									src={station.favicon}
									alt=""
									class="h-5 w-5 object-contain"
									onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
								/>
							{/if}
						</div>

						<div class="min-w-0 flex-1">
							<p class="truncate text-xs font-bold leading-tight">{station.name}</p>
							<p class="truncate text-[10px] leading-tight text-[var(--color-muted)]">
								{station.countrycode}
								{#if station.bitrate}· {station.bitrate}kbps{/if}
								{#if station.codec}· {station.codec}{/if}
								{#if station.tags}· <span class="opacity-70">{station.tags.split(',').slice(0, 3).join(', ')}</span>{/if}
							</p>
						</div>

						<div class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
							<button
								onclick={() => handleAdd(station)}
								disabled={adding !== null}
								class="border border-[var(--color-border)] p-1 transition-colors
									{adding === station.stationuuid
									? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
									: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
									disabled:opacity-40"
								aria-label="add to queue"
								title="Add to queue"
							>
								{#if adding === station.stationuuid}
									<span class="px-0.5 text-[10px]">✓</span>
								{:else}
									<PlusIcon size={11} weight="bold" />
								{/if}
							</button>

							<button
								onclick={() => handlePlay(station)}
								disabled={playing !== null}
								class="border border-[var(--color-border)] p-1 transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
									disabled:opacity-40"
								aria-label="play now"
								title="Play now"
							>
								<PlayIcon size={11} weight="fill" />
							</button>

							<button
								onclick={() => (playlistPopup = station.stationuuid)}
								class="border border-[var(--color-border)] p-1 transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
								aria-label="add to playlist"
								title="Add to playlist"
							>
								<BookmarkSimpleIcon size={11} weight="bold" />
							</button>
						</div>

						{#if playlistPopup === station.stationuuid}
							<AddToPlaylistPopup
								songUri={station.url_resolved}
								onclose={() => (playlistPopup = null)}
							/>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{:catch}
		<p class="px-4 py-3 text-xs text-[var(--color-muted)]">search failed</p>
	{/await}
{/if}
