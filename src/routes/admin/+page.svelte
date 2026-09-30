<script lang="ts">
	import { ArrowsClockwiseIcon, DatabaseIcon, SignOutIcon } from 'phosphor-svelte';
	import { isHttpError } from '@sveltejs/kit';
	import { mpdUpdate, getSearchStatus } from '$lib/mpd.remote';

	// Remote `command()` failures surface as HttpError (not Error), so a plain
	// `instanceof Error` check never matches — this reads the real message either way.
	function errorMessage(e: unknown): string {
		if (isHttpError(e)) return e.body.message;
		if (e instanceof Error) return e.message;
		return 'unknown error';
	}

	// MPD update state
	let updating = $state(false);
	let updateDone = $state(false);
	let updateError = $state<string | null>(null);

	// Search index status (reactive query)
	const indexStatus = getSearchStatus();

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
			updateError = errorMessage(e);
		} finally {
			updating = false;
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

	<!-- Session -->
	<section class="border border-[var(--color-border)]">
		<div class="flex items-center justify-between px-4 py-3">
			<p class="text-xs font-bold">Session</p>
			<a
				href="/logout"
				class="flex items-center gap-1.5 border border-[var(--color-border)] px-3 py-1.5
					text-[10px] tracking-wider uppercase transition-colors
					hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
			>
				<SignOutIcon size={11} weight="bold" />
				log out
			</a>
		</div>
	</section>
</div>
