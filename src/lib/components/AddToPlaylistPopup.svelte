<script lang="ts">
	import { onMount } from 'svelte';
	import { XIcon } from 'phosphor-svelte';
	import { getPlaylists, addSongToPlaylist } from '$lib/mpd.remote';

	interface Props {
		songUri: string;
		onclose: () => void;
	}

	let { songUri, onclose }: Props = $props();

	let playlists = $state<Promise<Awaited<ReturnType<typeof getPlaylists>>>>(new Promise(() => {}));

	onMount(() => {
		playlists = getPlaylists();
	});
	let newName = $state('');
	let saving = $state(false);
	let savedTo = $state<string | null>(null);

	async function addTo(name: string) {
		if (saving) return;
		saving = true;
		try {
			await addSongToPlaylist({ uri: songUri, playlist: name });
			savedTo = name;
			setTimeout(() => onclose(), 800);
		} finally {
			saving = false;
		}
	}

	async function createAndAdd() {
		const name = newName.trim();
		if (!name) return;
		await addTo(name);
	}
</script>

<!-- Fixed overlay -->
<div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true">
	<!-- Backdrop -->
	<button
		class="absolute inset-0 bg-[var(--color-fg)]/20"
		onclick={onclose}
		aria-label="close"
		tabindex="-1"
	></button>

	<!-- Card -->
	<div
		class="relative z-10 w-64 border-2 border-[var(--color-border)] bg-[var(--color-bg)] p-4 shadow-lg"
	>
		<div class="mb-3 flex items-center justify-between">
			<span class="text-[10px] font-bold tracking-widest uppercase">add to playlist</span>
			<button
				onclick={onclose}
				class="text-[var(--color-muted)] hover:text-[var(--color-fg)] transition-colors"
			>
				<XIcon size={12} weight="bold" />
			</button>
		</div>

		{#await playlists}
			<p class="text-[10px] text-[var(--color-muted)]">loading...</p>
		{:then items}
			{#if items.length > 0}
				<ul class="mb-3 max-h-40 overflow-y-auto border border-[var(--color-border)]/30">
					{#each items as item}
						<li>
							<button
								onclick={() => addTo(item.playlist)}
								disabled={saving}
								class="w-full text-left px-2 py-1.5 text-[10px] transition-colors
									{savedTo === item.playlist
									? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
									: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
									disabled:opacity-40"
							>
								{savedTo === item.playlist ? '✓ ' : ''}{item.playlist}
							</button>
						</li>
					{/each}
				</ul>
			{/if}

			<div class="flex gap-1">
				<input
					type="text"
					bind:value={newName}
					placeholder="new playlist name..."
					onkeydown={(e) => e.key === 'Enter' && createAndAdd()}
					class="min-w-0 flex-1 border border-[var(--color-border)] bg-transparent px-2 py-1 text-[10px] outline-none placeholder:text-[var(--color-muted)]"
				/>
				<button
					onclick={createAndAdd}
					disabled={!newName.trim() || saving}
					class="border border-[var(--color-border)] px-2 py-1 text-[10px] font-bold transition-colors
						hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
						disabled:opacity-40"
				>
					+
				</button>
			</div>
		{:catch}
			<p class="text-[10px] text-[var(--color-muted)]">failed to load playlists</p>
		{/await}
	</div>
</div>
