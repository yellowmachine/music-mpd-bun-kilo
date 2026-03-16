<script lang="ts">
	import { FolderIcon, FolderOpenIcon, PlusIcon } from 'phosphor-svelte';
	import { addToQueue } from '$lib/mpd.remote';
	import type { LibraryDirectory } from '$lib/mpd.types';

	interface Props {
		dir: LibraryDirectory;
		href: string;
	}

	let { dir, href }: Props = $props();

	let adding = $state(false);
	let added = $state(false);

	async function handleAdd(e: MouseEvent) {
		e.preventDefault();
		if (adding) return;
		adding = true;
		try {
			await addToQueue(dir.directory);
			added = true;
			setTimeout(() => (added = false), 2000);
		} finally {
			adding = false;
		}
	}
</script>

<li class="group border-b border-[var(--color-border)]/30">
	<a
		{href}
		class="flex items-center gap-3 px-4 py-2 transition-colors hover:bg-[var(--color-fg)]/5"
	>
		<!-- Folder icon -->
		<span class="shrink-0 text-[var(--color-muted)] group-hover:text-[var(--color-fg)]">
			<FolderIcon size={14} weight="bold" class="group-hover:hidden" />
			<FolderOpenIcon size={14} weight="bold" class="hidden group-hover:block" />
		</span>

		<!-- Name -->
		<span class="min-w-0 flex-1 truncate text-xs font-bold">
			{dir.name}
		</span>

		<!-- Add folder to queue -->
		<button
			onclick={handleAdd}
			disabled={adding}
			class="shrink-0 border border-[var(--color-border)] px-2 py-0.5 text-[10px] tracking-wider uppercase
				opacity-0 transition-opacity group-hover:opacity-100
				{added
				? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
				: 'hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]'}"
			aria-label="add folder to queue"
		>
			{#if added}
				✓
			{:else}
				<PlusIcon size={10} weight="bold" />
			{/if}
		</button>
	</a>
</li>
