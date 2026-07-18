<script lang="ts">
	import { SpeakerHighIcon, SpeakerNoneIcon, SpeakerSlashIcon } from 'phosphor-svelte';
	import { mpdStore } from '$lib/mpd.svelte';
	import { snapSetVolume, snapSetMute } from '$lib/mpd.remote';

	// Local optimistic volume per client id
	// (sliders update immediately, command fires in background)
	let localVolumes = $state<Record<string, number>>({});

	function getVolume(id: string, percent: number): number {
		return localVolumes[id] ?? percent;
	}

	async function handleVolume(id: string, e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const percent = Number(input.value);
		localVolumes[id] = percent;
		await snapSetVolume({ id, percent });
	}

	async function stepSnapVolume(id: string, current: number, delta: number) {
		const percent = Math.max(0, Math.min(100, current + delta));
		localVolumes[id] = percent;
		await snapSetVolume({ id, percent });
	}

	async function handleMute(id: string, muted: boolean) {
		await snapSetMute({ id, muted });
	}
</script>

<div class="divide-y divide-[var(--color-border)]/30">
	{#if mpdStore.snapClients.length === 0}
		<p class="px-4 py-8 text-center text-xs text-[var(--color-muted)]">
			— no snapclients connected —
		</p>
	{:else}
		{#each mpdStore.snapClients as client (client.id)}
			<div
				class="flex items-center gap-4 px-4 py-3
					{client.connected ? '' : 'opacity-40'}"
			>
				<!-- Name + status -->
				<div class="min-w-0 flex-1">
					<p class="truncate text-xs font-bold">
						{client.name}
					</p>
					<p class="text-[10px] text-[var(--color-muted)]">
						{client.ip}
						{#if !client.connected}
							<span class="ml-1">· offline</span>
						{/if}
					</p>
				</div>

				<!-- Volume slider -->
				<div class="flex items-center gap-2">
					<!-- Mute toggle -->
					<button
						onclick={() => handleMute(client.id, !client.volume.muted)}
						class="shrink-0 text-[var(--color-muted)] transition-colors hover:text-[var(--color-fg)]
							{client.volume.muted ? 'text-[var(--color-fg)]' : ''}"
						aria-label={client.volume.muted ? 'unmute' : 'mute'}
					>
						{#if client.volume.muted}
							<SpeakerSlashIcon size={14} weight="bold" />
						{:else if getVolume(client.id, client.volume.percent) === 0}
							<SpeakerNoneIcon size={14} weight="bold" />
						{:else}
							<SpeakerHighIcon size={14} weight="bold" />
						{/if}
					</button>

					<button
						onclick={() =>
							stepSnapVolume(client.id, getVolume(client.id, client.volume.percent), -5)}
						disabled={client.volume.muted || !client.connected}
						class="leading-none text-[var(--color-muted)] hover:text-[var(--color-fg)] disabled:opacity-30"
						aria-label="decrease volume">−</button
					>
					<input
						type="range"
						min="0"
						max="100"
						value={getVolume(client.id, client.volume.percent)}
						oninput={(e) => {
							localVolumes[client.id] = Number((e.currentTarget as HTMLInputElement).value);
						}}
						onchange={(e) => handleVolume(client.id, e)}
						disabled={client.volume.muted || !client.connected}
						class="w-28 text-[var(--color-fg)] disabled:opacity-30"
						aria-label="volume for {client.name}"
					/>
					<button
						onclick={() =>
							stepSnapVolume(client.id, getVolume(client.id, client.volume.percent), 5)}
						disabled={client.volume.muted || !client.connected}
						class="leading-none text-[var(--color-muted)] hover:text-[var(--color-fg)] disabled:opacity-30"
						aria-label="increase volume">+</button
					>

					<span class="w-7 text-right text-[10px] text-[var(--color-muted)] tabular-nums">
						{getVolume(client.id, client.volume.percent)}
					</span>
				</div>
			</div>
		{/each}
	{/if}
</div>
