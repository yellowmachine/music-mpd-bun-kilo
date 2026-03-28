<script lang="ts">
	import { ListPlusIcon, PlayIcon, PlusIcon } from 'phosphor-svelte';
	import { addPlaylistToQueue, playPlaylist } from '$lib/mpd.remote';

	let { data } = $props();

	let playing = $state<string | null>(null);
	let adding = $state<string | null>(null);

	async function handlePlay(name: string) {
		if (playing) return;
		playing = name;
		try {
			await playPlaylist(name);
		} finally {
			playing = null;
		}
	}

	async function handleAdd(name: string) {
		if (adding) return;
		adding = name;
		try {
			await addPlaylistToQueue(name);
			setTimeout(() => { if (adding === name) adding = null; }, 2000);
		} catch {
			adding = null;
		}
	}
</script>

{#if data.playlists.length === 0}
	<p class="py-8 text-center text-xs text-[var(--color-muted)]">— no saved playlists —</p>
{:else}
	<ul>
		{#each data.playlists as item}
			<li class="group flex items-center gap-3 border-b border-[var(--color-border)]/30 px-4 py-2 hover:bg-[var(--color-fg)]/5">
				<ListPlusIcon size={13} weight="bold" class="shrink-0 text-[var(--color-muted)]" />

				<a
					href="/playlists/{encodeURIComponent(item.playlist)}"
					class="flex-1 text-xs font-bold hover:underline"
				>
					{item.playlist}
				</a>

				<!-- Actions -->
				<div class="flex shrink-0 items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
					<!-- Add to queue -->
					<button
						onclick={() => handleAdd(item.playlist)}
						disabled={adding !== null}
						class="border border-[var(--color-border)] p-1 transition-colors
							{adding === item.playlist
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
							disabled:opacity-40"
						aria-label="add to queue"
						title="Add to queue"
					>
						{#if adding === item.playlist}
							<span class="px-0.5 text-[10px]">✓</span>
						{:else}
							<PlusIcon size={11} weight="bold" />
						{/if}
					</button>

					<!-- Play (clear queue + load + play) -->
					<button
						onclick={() => handlePlay(item.playlist)}
						disabled={playing !== null}
						class="border border-[var(--color-border)] p-1 transition-colors
							hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
							disabled:opacity-40"
						aria-label="play playlist"
						title="Play now (clears queue)"
					>
						<PlayIcon size={11} weight="fill" />
					</button>
				</div>
			</li>
		{/each}
	</ul>
{/if}
