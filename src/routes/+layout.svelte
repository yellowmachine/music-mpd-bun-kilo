<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { mpdStore } from '$lib/mpd.svelte';
	import Player from '$lib/components/Player.svelte';
	import type { MpdStatus, MpdSong, MpdQueueItem, SnapClient } from '$lib/mpd.types';
	import {
		QueueIcon,
		MagnifyingGlassIcon,
		MusicNotesIcon,
		GearIcon,
		SpeakerHighIcon
	} from 'phosphor-svelte';

	let { children } = $props();

	$effect(() => {
		if (!browser) return;

		const eventSource = new EventSource('/sse');

		// Full state snapshot sent once on connect
		eventSource.addEventListener('snapshot', (e: MessageEvent) => {
			try {
				const data = JSON.parse(e.data) as {
					status: MpdStatus | null;
					song: MpdSong | null;
					queue: MpdQueueItem[];
				};
				applyStatus(data.status);
				applyCurrentSong(data.song);
				mpdStore.queue = data.queue;
				mpdStore.connected = true;
			} catch {}
		});

		// Incremental broadcasts from idle connection
		eventSource.addEventListener('player', (e: MessageEvent) => {
			try {
				const data = JSON.parse(e.data) as { status: MpdStatus; song: MpdSong };
				applyStatus(data.status);
				applyCurrentSong(data.song);
			} catch {}
		});

		eventSource.addEventListener('mixer', (e: MessageEvent) => {
			try {
				const { volume } = JSON.parse(e.data);
				mpdStore.volume = volume ?? mpdStore.volume;
			} catch {}
		});

		eventSource.addEventListener('playlist', (e: MessageEvent) => {
			try {
				const { queue } = JSON.parse(e.data) as { queue: MpdQueueItem[] };
				mpdStore.queue = queue;
			} catch {}
		});

		eventSource.addEventListener('options', (e: MessageEvent) => {
			try {
				const data = JSON.parse(e.data);
				mpdStore.random = data.random ?? mpdStore.random;
				mpdStore.repeat = data.repeat ?? mpdStore.repeat;
				mpdStore.single = data.single ?? mpdStore.single;
				mpdStore.consume = data.consume ?? mpdStore.consume;
			} catch {}
		});

		eventSource.addEventListener('snap_clients', (e: MessageEvent) => {
			try {
				const { clients } = JSON.parse(e.data) as { clients: SnapClient[] };
				mpdStore.snapClients = clients;
			} catch {}
		});

		eventSource.onerror = () => {
			mpdStore.connected = false;
		};

		return () => {
			eventSource.close();
		};
	});

	function applyStatus(status: MpdStatus | null) {
		if (!status) return;
		mpdStore.playing = status.state === 'play';
		mpdStore.paused = status.state === 'pause';
		mpdStore.stopped = status.state === 'stop';
		mpdStore.volume = status.volume;
		mpdStore.elapsed = status.elapsed ?? 0;
		mpdStore.duration = status.duration ?? 0;
		mpdStore.random = status.random;
		mpdStore.repeat = status.repeat;
		mpdStore.single = status.single;
		mpdStore.consume = status.consume;
	}

	function applyCurrentSong(song: MpdSong | null) {
		mpdStore.currentSong = song
			? {
					file: song.file,
					title: song.title,
					artist: song.artist,
					album: song.album
				}
			: null;
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>MPD</title>
</svelte:head>

<div class="flex h-screen flex-col overflow-hidden">
	<!-- Navbar -->
	<header
		class="flex items-center justify-between border-b-2 border-[var(--color-border)] px-4 py-0"
	>
		<!-- Brand -->
		<span class="text-xs font-bold tracking-[0.25em] uppercase">svelte-mpd</span>

		<!-- Nav links -->
		<nav class="flex h-full items-stretch">
			<a
				href="/"
				aria-label="queue"
				class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-4 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/'
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			>
				<QueueIcon size={14} weight="bold" />
				queue
			</a>
			<a
				href="/library"
				aria-label="library"
				class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-4 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname.startsWith('/library')
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			>
				<MusicNotesIcon size={14} weight="bold" />
				library
			</a>
			<a
				href="/search"
				aria-label="search"
				class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-4 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/search'
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			>
				<MagnifyingGlassIcon size={14} weight="bold" />
				search
			</a>
			<a
				href="/snap"
				aria-label="snapcast"
				class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-4 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/snap'
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			>
				<SpeakerHighIcon size={14} weight="bold" />
				snap
			</a>
			<a
				href="/admin"
				aria-label="admin"
				class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-4 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/admin'
					? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
					: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
			>
				<GearIcon size={14} weight="bold" />
				admin
			</a>
		</nav>

		<!-- Connection status -->
		<span class="text-[10px] text-[var(--color-muted)]">
			{mpdStore.connected ? '●' : '○'}
		</span>
	</header>

	<!-- Main content -->
	<main class="min-h-0 flex-1 overflow-auto">
		{@render children()}
	</main>

	<!-- Player bar -->
	<Player />
</div>
