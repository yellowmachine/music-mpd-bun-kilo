<script lang="ts">
	import { enhance } from '$app/forms';
	import { LockIcon } from 'phosphor-svelte';
	import type { ActionData } from './$types';

	let { form }: { form: ActionData } = $props();

	let submitting = $state(false);
</script>

<svelte:head>
	<title>Login — MPD</title>
</svelte:head>

<div class="flex min-h-screen items-center justify-center p-6">
	<div class="w-full max-w-xs border border-[var(--color-border)]">
		<div class="flex items-center gap-2 border-b border-[var(--color-border)] px-4 py-2">
			<LockIcon size={13} weight="bold" />
			<span class="text-[10px] font-bold tracking-widest uppercase">Login</span>
		</div>

		<form
			method="POST"
			class="space-y-3 px-4 py-4"
			use:enhance={() => {
				submitting = true;
				return async ({ update }) => {
					await update();
					submitting = false;
				};
			}}
		>
			<div class="space-y-1.5">
				<label
					for="password"
					class="text-[10px] tracking-widest text-[var(--color-muted)] uppercase"
				>
					Password
				</label>
				<input
					id="password"
					name="password"
					type="password"
					autocomplete="current-password"
					autofocus
					required
					class="w-full border border-[var(--color-border)] bg-transparent px-3 py-2 text-sm
						focus:outline-none"
				/>
			</div>

			{#if form?.error}
				<p class="text-[10px] text-[var(--color-muted)]">{form.error}</p>
			{/if}

			<button
				type="submit"
				disabled={submitting}
				class="w-full border border-[var(--color-border)] px-3 py-2 text-[10px] tracking-widest
					uppercase transition-colors hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]
					disabled:opacity-40"
			>
				{submitting ? 'checking...' : 'log in'}
			</button>
		</form>
	</div>
</div>
