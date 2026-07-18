<script lang="ts">
	import { XIcon } from 'phosphor-svelte';
	import SnapClientList from '$lib/components/SnapClientList.svelte';

	interface Props {
		onclose: () => void;
	}

	let { onclose }: Props = $props();
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
		class="relative z-10 max-h-[80vh] w-80 overflow-y-auto border-2 border-[var(--color-border)] bg-[var(--color-bg)] shadow-lg"
	>
		<div
			class="sticky top-0 flex items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-bg)] p-4"
		>
			<span class="text-[10px] font-bold tracking-widest uppercase">snap volumes</span>
			<button
				onclick={onclose}
				class="text-[var(--color-muted)] transition-colors hover:text-[var(--color-fg)]"
				aria-label="close"
			>
				<XIcon size={12} weight="bold" />
			</button>
		</div>

		<SnapClientList />
	</div>
</div>
