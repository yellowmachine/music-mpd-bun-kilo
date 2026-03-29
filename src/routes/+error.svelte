<script>
	import { page } from '$app/stores';
</script>

<div class="flex min-h-screen flex-col items-center justify-center gap-5 px-4">
	{#if $page.status === 404}
		<!-- Cracked vinyl record -->
		<svg
			width="160"
			height="160"
			viewBox="0 0 180 180"
			xmlns="http://www.w3.org/2000/svg"
			aria-hidden="true"
		>
			<defs>
				<clipPath id="label-clip">
					<circle cx="90" cy="90" r="31" />
				</clipPath>
			</defs>

			<!-- Record body -->
			<circle cx="90" cy="90" r="88" fill="#0a0a0a" />

			<!-- Groove rings -->
			<circle cx="90" cy="90" r="78" fill="none" stroke="#1c1c1c" stroke-width="2" />
			<circle cx="90" cy="90" r="68" fill="none" stroke="#1c1c1c" stroke-width="2" />
			<circle cx="90" cy="90" r="58" fill="none" stroke="#1c1c1c" stroke-width="2" />
			<circle cx="90" cy="90" r="48" fill="none" stroke="#1c1c1c" stroke-width="2" />
			<circle cx="90" cy="90" r="40" fill="none" stroke="#1c1c1c" stroke-width="1.5" />

			<!-- Label -->
			<circle cx="90" cy="90" r="31" fill="#f5f0e8" />

			<!-- Center hole -->
			<circle cx="90" cy="90" r="5" fill="#0a0a0a" />

			<!-- Crack — cream, visible on dark record -->
			<polyline
				points="62,4 83,70 74,92 94,112 78,176"
				stroke="#f5f0e8"
				stroke-width="2.5"
				fill="none"
				stroke-linejoin="miter"
				stroke-linecap="square"
			/>
			<line
				x1="83"
				y1="70"
				x2="108"
				y2="52"
				stroke="#f5f0e8"
				stroke-width="1.5"
				stroke-linecap="square"
			/>

			<!-- Crack — black, clipped to label so it shows over cream -->
			<polyline
				points="62,4 83,70 74,92 94,112 78,176"
				stroke="#0a0a0a"
				stroke-width="2.5"
				fill="none"
				stroke-linejoin="miter"
				stroke-linecap="square"
				clip-path="url(#label-clip)"
			/>
			<line
				x1="83"
				y1="70"
				x2="108"
				y2="52"
				stroke="#0a0a0a"
				stroke-width="1.5"
				stroke-linecap="square"
				clip-path="url(#label-clip)"
			/>
		</svg>

		<p class="text-[3.5rem] font-bold leading-none tracking-[0.5rem]">404</p>
		<p class="text-[0.7rem] uppercase tracking-[0.35rem]">Track Not Found</p>
		<p class="max-w-xs text-center text-[0.65rem] text-[var(--color-muted)]">
			The requested resource could not be located in the library.
		</p>
	{:else}
		<!-- Tangled cassette tape -->
		<svg
			width="200"
			height="175"
			viewBox="0 0 200 175"
			xmlns="http://www.w3.org/2000/svg"
			aria-hidden="true"
		>
			<!-- Cassette body -->
			<rect x="4" y="4" width="192" height="108" fill="#0a0a0a" rx="6" />

			<!-- Window (cream) -->
			<rect x="40" y="24" width="120" height="64" fill="#f5f0e8" rx="3" />

			<!-- Left reel -->
			<circle cx="72" cy="56" r="23" fill="#0a0a0a" />
			<line x1="72" y1="33" x2="72" y2="56" stroke="#f5f0e8" stroke-width="2.5" />
			<line x1="72" y1="56" x2="93" y2="68" stroke="#f5f0e8" stroke-width="2.5" />
			<line x1="72" y1="56" x2="51" y2="68" stroke="#f5f0e8" stroke-width="2.5" />
			<circle cx="72" cy="56" r="7" fill="#f5f0e8" />
			<circle cx="72" cy="56" r="3" fill="#0a0a0a" />

			<!-- Right reel (partially unwound — smaller radius tape) -->
			<circle cx="128" cy="56" r="16" fill="#0a0a0a" />
			<line x1="128" y1="40" x2="128" y2="56" stroke="#f5f0e8" stroke-width="2.5" />
			<line x1="128" y1="56" x2="142" y2="64" stroke="#f5f0e8" stroke-width="2.5" />
			<line x1="128" y1="56" x2="114" y2="64" stroke="#f5f0e8" stroke-width="2.5" />
			<circle cx="128" cy="56" r="7" fill="#f5f0e8" />
			<circle cx="128" cy="56" r="3" fill="#0a0a0a" />

			<!-- Corner screws -->
			<circle cx="17" cy="17" r="5" fill="#f5f0e8" />
			<circle cx="183" cy="17" r="5" fill="#f5f0e8" />
			<circle cx="17" cy="99" r="5" fill="#f5f0e8" />
			<circle cx="183" cy="99" r="5" fill="#f5f0e8" />

			<!-- Tape guide pins -->
			<circle cx="87" cy="85" r="5" fill="#0a0a0a" />
			<circle cx="113" cy="85" r="5" fill="#0a0a0a" />

			<!-- Unraveled tape spilling out -->
			<path
				d="M 87 112 Q 78 122 88 130 Q 98 138 84 144 Q 70 150 74 138 Q 78 126 62 128 Q 46 130 50 146 Q 54 162 38 164"
				stroke="#0a0a0a"
				stroke-width="3"
				fill="none"
				stroke-linecap="round"
				stroke-linejoin="round"
			/>
			<path
				d="M 113 112 Q 122 122 114 132 Q 106 142 118 150 Q 130 158 136 146 Q 142 134 156 140"
				stroke="#0a0a0a"
				stroke-width="3"
				fill="none"
				stroke-linecap="round"
				stroke-linejoin="round"
			/>
		</svg>

		<p class="text-[3.5rem] font-bold leading-none tracking-[0.5rem]">{$page.status}</p>
		<p class="text-[0.7rem] uppercase tracking-[0.35rem]">Playback Error</p>
		<p class="max-w-xs text-center text-[0.65rem] text-[var(--color-muted)]">
			{$page.error?.message ?? 'Something went wrong on the server.'}
		</p>
	{/if}

	<a
		href="/"
		class="mt-2 border border-[var(--color-border)] px-4 py-2 text-[0.65rem] uppercase tracking-[0.3rem] transition-colors hover:bg-[var(--color-fg)] hover:text-[var(--color-accent-fg)]"
	>
		← Return to Library
	</a>
</div>
