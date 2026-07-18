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
		SpeakerHighIcon,
		DotsThreeIcon,
		ListPlusIcon,
		RadioIcon,
		RssIcon,
		ArticleIcon
	} from 'phosphor-svelte';

	let { children } = $props();

	let overflowOpen = $state(false);

	// Close overflow menu on navigation
	$effect(() => {
		$page.url.pathname;
		overflowOpen = false;
	});

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
	<div
		class="mx-auto flex w-full max-w-3xl flex-1 flex-col overflow-hidden border-x border-[var(--color-border)]"
	>
		<!-- Navbar -->
		<header
			class="relative flex items-center justify-between border-b-2 border-[var(--color-border)] px-4 py-0"
		>
			<!-- Brand -->
			<span class="text-xs font-bold tracking-[0.25em] uppercase">svelte-mpd</span>

			<!-- Nav links -->
			<nav class="flex h-full items-stretch">
				<!-- Always visible -->
				<a
					href="/"
					aria-label="queue"
					class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-3 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/'
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
				>
					<QueueIcon size={14} weight="bold" />
					<span class="hidden sm:inline">queue</span>
				</a>
				<a
					href="/library"
					aria-label="library"
					class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-3 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname.startsWith('/library')
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
				>
					<MusicNotesIcon size={14} weight="bold" />
					<span class="hidden sm:inline">library</span>
				</a>
				<a
					href="/search"
					aria-label="search"
					class="flex items-center gap-1.5 border-l border-[var(--color-border)] px-3 py-2.5 text-[10px] tracking-widest uppercase transition-colors
					{$page.url.pathname === '/search'
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
				>
					<MagnifyingGlassIcon size={14} weight="bold" />
					<span class="hidden sm:inline">search</span>
				</a>
				<!-- Overflow menu: playlists, radio, podcasts, articles, snap, admin.
				     Always collapsed here (not just on mobile) — the app shell is capped
				     at max-w-3xl regardless of screen size, so there's never enough room
				     to show every section as its own tab, laptop included. -->
				<button
					onclick={() => (overflowOpen = !overflowOpen)}
					class="flex items-center border-l border-[var(--color-border)] px-3 py-2.5 transition-colors
					{overflowOpen ||
					$page.url.pathname.startsWith('/playlists') ||
					$page.url.pathname.startsWith('/radio') ||
					$page.url.pathname.startsWith('/podcasts') ||
					$page.url.pathname.startsWith('/articles') ||
					$page.url.pathname === '/snap' ||
					$page.url.pathname === '/admin'
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: 'text-[var(--color-muted)]'}"
					aria-label="more"
				>
					<DotsThreeIcon size={16} weight="bold" />
				</button>
			</nav>

			<!-- Connection status -->
			<span class="text-[10px] text-[var(--color-muted)]">
				{mpdStore.connected ? '●' : '○'}
			</span>

			<!-- Overflow dropdown -->
			{#if overflowOpen}
				<div
					class="absolute top-full right-0 z-50 border border-[var(--color-border)] bg-[var(--color-bg)]"
					style="min-width: 140px"
				>
					<a
						href="/playlists"
						class="flex items-center gap-2 border-b border-[var(--color-border)]/30 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname.startsWith('/playlists')
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<ListPlusIcon size={13} weight="bold" />
						playlists
					</a>
					<a
						href="/radio"
						class="flex items-center gap-2 border-b border-[var(--color-border)]/30 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname.startsWith('/radio')
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<RadioIcon size={13} weight="bold" />
						radio
					</a>
					<a
						href="/podcasts"
						class="flex items-center gap-2 border-b border-[var(--color-border)]/30 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname.startsWith('/podcasts')
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<RssIcon size={13} weight="bold" />
						podcasts
					</a>
					<a
						href="/articles"
						class="flex items-center gap-2 border-b border-[var(--color-border)]/30 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname.startsWith('/articles')
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<ArticleIcon size={13} weight="bold" />
						articles
					</a>
					<a
						href="/snap"
						class="flex items-center gap-2 border-b border-[var(--color-border)]/30 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname === '/snap'
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<SpeakerHighIcon size={13} weight="bold" />
						snap
					</a>
					<a
						href="/admin"
						class="flex items-center gap-2 px-4 py-3 text-[10px] tracking-widest uppercase transition-colors
						{$page.url.pathname === '/admin'
							? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
							: 'text-[var(--color-muted)] hover:text-[var(--color-fg)]'}"
					>
						<GearIcon size={13} weight="bold" />
						admin
					</a>
				</div>
			{/if}
		</header>

		<!-- Main content -->
		<main class="min-h-0 flex-1 overflow-auto">
			{@render children()}
		</main>

		<!-- Player bar -->
		<Player />

		<footer
			class="border-t border-[var(--color-border)]/30 px-4 py-2 text-center text-[10px] tracking-widest text-[var(--color-muted)] uppercase"
		>
			built with the help of claude
		</footer>
	</div>
</div>
