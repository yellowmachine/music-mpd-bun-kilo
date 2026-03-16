<script lang="ts">
	import {
		PlayIcon,
		PauseIcon,
		StopIcon,
		SkipBackIcon,
		SkipForwardIcon,
		SpeakerHighIcon,
		SpeakerLowIcon,
		SpeakerNoneIcon,
		ShuffleIcon,
		RepeatIcon,
		RepeatOnceIcon
	} from 'phosphor-svelte';
	import { mpdStore } from '$lib/mpd.svelte';
	import {
		play,
		pause,
		stop,
		next,
		prev,
		setVolume,
		seek,
		setRandom,
		setRepeat
	} from '$lib/mpd.remote';

	function formatTime(seconds: number): string {
		const m = Math.floor(seconds / 60);
		const s = Math.floor(seconds % 60);
		return `${m}:${s.toString().padStart(2, '0')}`;
	}

	const progress = $derived(
		mpdStore.duration > 0 ? (mpdStore.elapsed / mpdStore.duration) * 100 : 0
	);

	async function handleSeek(e: MouseEvent) {
		const bar = e.currentTarget as HTMLElement;
		const rect = bar.getBoundingClientRect();
		const ratio = (e.clientX - rect.left) / rect.width;
		await seek(Math.floor(ratio * mpdStore.duration));
	}

	async function handleVolume(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		await setVolume(Number(input.value));
	}
</script>

<div class="border-t-2 border-[var(--color-border)] bg-[var(--color-bg)]">
	<!-- Song info -->
	<div class="border-b border-[var(--color-border)] px-4 py-2">
		{#if mpdStore.currentSong}
			<p class="truncate text-sm font-bold">
				{mpdStore.currentSong.title ?? mpdStore.currentSong.file}
			</p>
			{#if mpdStore.currentSong.artist}
				<p class="truncate text-xs text-[var(--color-muted)]">
					{mpdStore.currentSong.artist}
					{#if mpdStore.currentSong.album}
						— {mpdStore.currentSong.album}
					{/if}
				</p>
			{/if}
		{:else}
			<p class="text-xs text-[var(--color-muted)]">no track loaded</p>
		{/if}
	</div>

	<!-- Progress bar -->
	<div
		class="group relative h-1.5 cursor-pointer bg-[var(--color-border)]/20"
		role="slider"
		aria-label="seek"
		aria-valuenow={mpdStore.elapsed}
		aria-valuemin={0}
		aria-valuemax={mpdStore.duration}
		tabindex="0"
		onclick={handleSeek}
		onkeydown={(e) => {
			if (e.key === 'ArrowRight') seek(mpdStore.elapsed + 5);
			if (e.key === 'ArrowLeft') seek(Math.max(0, mpdStore.elapsed - 5));
		}}
	>
		<div
			class="h-full bg-[var(--color-fg)] transition-[width] duration-1000"
			style="width: {progress}%"
		></div>
	</div>

	<!-- Time -->
	<div class="flex justify-between px-4 pt-1 text-[10px] text-[var(--color-muted)]">
		<span>{formatTime(mpdStore.elapsed)}</span>
		<span>{formatTime(mpdStore.duration)}</span>
	</div>

	<!-- Controls -->
	<div class="flex items-center justify-between px-4 py-2">
		<!-- Playback -->
		<div class="flex items-center gap-1">
			<button
				onclick={() => prev()}
				class="rounded p-1.5 hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
				aria-label="previous"
			>
				<SkipBackIcon size={18} weight="bold" />
			</button>

			{#if mpdStore.playing}
				<button
					onclick={() => pause()}
					class="rounded border-2 border-[var(--color-border)] p-1.5 hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
					aria-label="pause"
				>
					<PauseIcon size={18} weight="fill" />
				</button>
			{:else}
				<button
					onclick={() => play()}
					class="rounded border-2 border-[var(--color-border)] p-1.5 hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
					aria-label="play"
				>
					<PlayIcon size={18} weight="fill" />
				</button>
			{/if}

			<button
				onclick={() => stop()}
				class="rounded p-1.5 hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
				aria-label="stop"
			>
				<StopIcon size={18} weight="fill" />
			</button>

			<button
				onclick={() => next()}
				class="rounded p-1.5 hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
				aria-label="next"
			>
				<SkipForwardIcon size={18} weight="bold" />
			</button>
		</div>

		<!-- Flags -->
		<div class="flex items-center gap-1">
			<button
				onclick={() => setRandom(!mpdStore.random)}
				class="rounded p-1.5 {mpdStore.random
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
				aria-label="shuffle"
			>
				<ShuffleIcon size={15} weight="bold" />
			</button>

			<button
				onclick={() => setRepeat(!mpdStore.repeat)}
				class="rounded p-1.5 {mpdStore.repeat
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
				aria-label="repeat"
			>
				{#if mpdStore.single}
					<RepeatOnceIcon size={15} weight="bold" />
				{:else}
					<RepeatIcon size={15} weight="bold" />
				{/if}
			</button>
		</div>

		<!-- Volume -->
		<div class="flex items-center gap-2">
			{#if mpdStore.volume === 0}
				<SpeakerNoneIcon size={15} />
			{:else if mpdStore.volume < 50}
				<SpeakerLowIcon size={15} />
			{:else}
				<SpeakerHighIcon size={15} />
			{/if}
			<input
				type="range"
				min="0"
				max="100"
				value={mpdStore.volume}
				onchange={handleVolume}
				class="h-0.5 w-20 cursor-pointer appearance-none bg-[var(--color-fg)] accent-[var(--color-fg)]"
				aria-label="volume"
			/>
			<span class="w-6 text-right text-[10px] text-[var(--color-muted)]">{mpdStore.volume}</span>
		</div>
	</div>
</div>
