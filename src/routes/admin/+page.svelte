<script lang="ts">
	import {
		ArrowsClockwiseIcon,
		DatabaseIcon,
		DownloadSimpleIcon,
		PowerIcon,
		WarningIcon
	} from 'phosphor-svelte';
	import { mpdUpdate, systemReboot, systemShutdown, getSearchStatus } from '$lib/mpd.remote';
	import { getUpdateInfo, triggerUpdate } from '$lib/update.remote';

	// MPD update state
	let updating = $state(false);
	let updateDone = $state(false);
	let updateError = $state<string | null>(null);

	// Search index status (reactive query)
	const indexStatus = getSearchStatus();

	// App version / update status
	const updateInfo = getUpdateInfo();
	let confirmApply = $state(false);
	let applying = $state(false);
	let applyError = $state<string | null>(null);

	async function handleApplyUpdate() {
		if (applying) return;
		applying = true;
		applyError = null;
		try {
			await triggerUpdate();
			// on success the app container is about to restart — stay in "applying" state
		} catch (e) {
			applyError = e instanceof Error ? e.message : 'unknown error';
			applying = false;
		}
	}

	async function handleMpdUpdate() {
		if (updating) return;
		updating = true;
		updateDone = false;
		updateError = null;
		try {
			await mpdUpdate();
			updateDone = true;
			setTimeout(() => (updateDone = false), 4000);
		} catch (e) {
			updateError = e instanceof Error ? e.message : 'unknown error';
		} finally {
			updating = false;
		}
	}

	// Danger zone: two-step confirmation
	let confirmAction = $state<'reboot' | 'shutdown' | null>(null);
	let running = $state(false);

	async function handleConfirm() {
		if (!confirmAction || running) return;
		running = true;
		try {
			if (confirmAction === 'reboot') await systemReboot();
			if (confirmAction === 'shutdown') await systemShutdown();
		} finally {
			running = false;
			confirmAction = null;
		}
	}
</script>

<div class="mx-auto max-w-lg space-y-6 p-6">
	<!-- MPD Database -->
	<section class="border border-[var(--color-border)]">
		<div class="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-2">
			<DatabaseIcon size={13} weight="bold" />
			<span class="text-[10px] font-bold tracking-widest uppercase">MPD Database</span>
		</div>

		<div class="space-y-3 px-4 py-4">
			<!-- Index status -->
			{#await indexStatus}
				<p class="text-[10px] text-[var(--color-muted)]">loading index status...</p>
			{:then status}
				<div class="flex items-center justify-between text-[10px]">
					<span class="text-[var(--color-muted)]">search index</span>
					<span class="tabular-nums">
						{#if status.indexing}
							<span class="text-[var(--color-muted)]">indexing...</span>
						{:else if status.ready}
							{status.total.toLocaleString()} songs
						{:else}
							<span class="text-[var(--color-muted)]">not ready</span>
						{/if}
					</span>
				</div>
			{:catch}
				<p class="text-[10px] text-[var(--color-muted)]">could not fetch index status</p>
			{/await}

			<!-- Update button -->
			<div class="flex items-center justify-between">
				<div class="space-y-0.5">
					<p class="text-xs font-bold">Update database</p>
					<p class="text-[10px] text-[var(--color-muted)]">
						Rescans the music directory and rebuilds the search index
					</p>
				</div>
				<button
					onclick={handleMpdUpdate}
					disabled={updating}
					class="flex shrink-0 items-center gap-1.5 border border-[var(--color-border)] px-3 py-1.5
						text-[10px] tracking-wider uppercase transition-colors
						{updateDone
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}
						disabled:opacity-40"
				>
					<ArrowsClockwiseIcon size={11} weight="bold" class={updating ? 'animate-spin' : ''} />
					{#if updateDone}
						done
					{:else if updating}
						updating...
					{:else}
						update
					{/if}
				</button>
			</div>

			{#if updateError}
				<p class="text-[10px] text-[var(--color-muted)]">error: {updateError}</p>
			{/if}
		</div>
	</section>

	<!-- App version -->
	<section class="border border-[var(--color-border)]">
		<div class="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-2">
			<DownloadSimpleIcon size={13} weight="bold" />
			<span class="text-[10px] font-bold tracking-widest uppercase">App version</span>
		</div>

		<div class="space-y-3 px-4 py-4">
			{#await updateInfo}
				<p class="text-[10px] text-[var(--color-muted)]">checking version...</p>
			{:then info}
				<div class="flex items-center justify-between text-[10px]">
					<span class="text-[var(--color-muted)]">running</span>
					<span class="tabular-nums">{info.current ? info.current.slice(0, 7) : 'unknown'}</span>
				</div>

				{#if info.error}
					<p class="text-[10px] text-[var(--color-muted)]">
						could not check for updates: {info.error}
					</p>
				{:else if !info.current}
					<p class="text-[10px] text-[var(--color-muted)]">
						image was not built with GIT_SHA — update checks disabled
					</p>
				{:else if info.latest}
					<div class="flex items-center justify-between text-[10px]">
						<span class="text-[var(--color-muted)]">latest (main)</span>
						<span class="tabular-nums">{info.latest.slice(0, 7)}</span>
					</div>
				{/if}

				{#if info.updateAvailable}
					<div class="flex items-center justify-between pt-1">
						<div class="space-y-0.5">
							<p class="text-xs font-bold">Update available</p>
							<p class="text-[10px] text-[var(--color-muted)]">
								Pulls the new image and restarts the app (playback is unaffected)
							</p>
						</div>
						{#if confirmApply}
							<div class="flex shrink-0 items-center gap-2">
								<span class="text-[10px] text-[var(--color-muted)]">sure?</span>
								<button
									onclick={handleApplyUpdate}
									disabled={applying}
									class="border border-[var(--color-border)] bg-[var(--color-fg)] px-3 py-1.5
										text-[10px] tracking-wider text-[var(--color-accent-fg)] uppercase
										disabled:opacity-40"
								>
									{applying ? 'updating...' : 'yes, update'}
								</button>
								{#if !applying}
									<button
										onclick={() => (confirmApply = false)}
										class="text-[10px] text-[var(--color-muted)] hover:text-[var(--color-fg)]"
									>
										cancel
									</button>
								{/if}
							</div>
						{:else}
							<button
								onclick={() => (confirmApply = true)}
								class="flex shrink-0 items-center gap-1.5 border border-[var(--color-border)] px-3 py-1.5
									text-[10px] tracking-wider uppercase transition-colors
									hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
							>
								<DownloadSimpleIcon size={11} weight="bold" />
								update
							</button>
						{/if}
					</div>
				{:else if info.current && !info.error}
					<p class="text-[10px] text-[var(--color-muted)]">up to date</p>
				{/if}

				{#if applyError}
					<p class="text-[10px] text-[var(--color-muted)]">error: {applyError}</p>
				{/if}
			{:catch}
				<p class="text-[10px] text-[var(--color-muted)]">could not fetch version info</p>
			{/await}
		</div>
	</section>

	<!-- Danger zone -->
	<section class="border border-[var(--color-border)]">
		<div class="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-2">
			<WarningIcon size={13} weight="bold" />
			<span class="text-[10px] font-bold tracking-widest uppercase">Danger zone</span>
		</div>

		<div class="divide-y divide-[var(--color-border)]/30">
			<!-- Reboot -->
			<div class="flex items-center justify-between px-4 py-3">
				<div class="space-y-0.5">
					<p class="text-xs font-bold">Reboot system</p>
					<p class="text-[10px] text-[var(--color-muted)]">Restarts the Raspberry Pi</p>
				</div>
				{#if confirmAction === 'reboot'}
					<div class="flex items-center gap-2">
						<span class="text-[10px] text-[var(--color-muted)]">sure?</span>
						<button
							onclick={handleConfirm}
							disabled={running}
							class="border border-[var(--color-border)] bg-[var(--color-fg)] px-3 py-1.5
								text-[10px] tracking-wider text-[var(--color-accent-fg)] uppercase
								disabled:opacity-40"
						>
							{running ? 'rebooting...' : 'yes, reboot'}
						</button>
						<button
							onclick={() => (confirmAction = null)}
							class="text-[10px] text-[var(--color-muted)] hover:text-[var(--color-fg)]"
						>
							cancel
						</button>
					</div>
				{:else}
					<button
						onclick={() => (confirmAction = 'reboot')}
						class="flex items-center gap-1.5 border border-[var(--color-border)] px-3 py-1.5
							text-[10px] tracking-wider uppercase transition-colors
							hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
					>
						<ArrowsClockwiseIcon size={11} weight="bold" />
						reboot
					</button>
				{/if}
			</div>

			<!-- Shutdown -->
			<div class="flex items-center justify-between px-4 py-3">
				<div class="space-y-0.5">
					<p class="text-xs font-bold">Shutdown system</p>
					<p class="text-[10px] text-[var(--color-muted)]">Powers off the Raspberry Pi</p>
				</div>
				{#if confirmAction === 'shutdown'}
					<div class="flex items-center gap-2">
						<span class="text-[10px] text-[var(--color-muted)]">sure?</span>
						<button
							onclick={handleConfirm}
							disabled={running}
							class="border border-[var(--color-border)] bg-[var(--color-fg)] px-3 py-1.5
								text-[10px] tracking-wider text-[var(--color-accent-fg)] uppercase
								disabled:opacity-40"
						>
							{running ? 'shutting down...' : 'yes, shutdown'}
						</button>
						<button
							onclick={() => (confirmAction = null)}
							class="text-[10px] text-[var(--color-muted)] hover:text-[var(--color-fg)]"
						>
							cancel
						</button>
					</div>
				{:else}
					<button
						onclick={() => (confirmAction = 'shutdown')}
						class="flex items-center gap-1.5 border border-[var(--color-border)] px-3 py-1.5
							text-[10px] tracking-wider uppercase transition-colors
							hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
					>
						<PowerIcon size={11} weight="bold" />
						shutdown
					</button>
				{/if}
			</div>
		</div>
	</section>
</div>
