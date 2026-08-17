<script lang="ts">
	import { browser } from '$app/environment';
	import { sendMessage } from '$lib/assistant.remote';
	import {
		PaperPlaneRightIcon,
		RobotIcon,
		UserIcon,
		MicrophoneIcon,
		StopIcon,
		SpeakerHighIcon,
		SpeakerSlashIcon
	} from 'phosphor-svelte';

	interface ChatMessage {
		role: 'user' | 'assistant';
		text: string;
	}

	let messages = $state<ChatMessage[]>([]);
	let input = $state('');
	let sending = $state(false);
	let recording = $state(false);
	let error = $state<string | null>(null);
	let listEl: HTMLElement;
	let audioEl: HTMLAudioElement;

	// Reply audio playback — off by default, persisted in localStorage
	let playReplyAudio = $state(
		browser ? localStorage.getItem('assistantPlayAudio') === 'true' : false
	);

	// getUserMedia needs a secure context (https, or localhost) — on a plain
	// http://<lan-ip> origin the browser refuses it outright.
	const micSupported = browser && window.isSecureContext && !!navigator.mediaDevices?.getUserMedia;

	function toggleReplyAudio() {
		playReplyAudio = !playReplyAudio;
		localStorage.setItem('assistantPlayAudio', String(playReplyAudio));
	}

	let mediaRecorder: MediaRecorder | null = null;
	let chunks: Blob[] = [];

	function scrollToEnd() {
		queueMicrotask(() => listEl?.scrollTo({ top: listEl.scrollHeight, behavior: 'smooth' }));
	}

	async function submit() {
		const text = input.trim();
		if (!text || sending) return;

		error = null;
		messages.push({ role: 'user', text });
		input = '';
		sending = true;

		try {
			const { reply } = await sendMessage(text);
			messages.push({ role: 'assistant', text: reply });
		} catch (err) {
			error = err instanceof Error ? err.message : 'error al contactar con el asistente';
		} finally {
			sending = false;
			scrollToEnd();
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			submit();
		}
	}

	async function toggleRecording() {
		if (recording) {
			mediaRecorder?.stop();
			return;
		}

		error = null;
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			chunks = [];
			mediaRecorder = new MediaRecorder(stream);
			mediaRecorder.ondataavailable = (e) => {
				if (e.data.size > 0) chunks.push(e.data);
			};
			mediaRecorder.onstop = () => {
				stream.getTracks().forEach((t) => t.stop());
				recording = false;
				sendVoice(new Blob(chunks, { type: mediaRecorder?.mimeType || 'audio/webm' }));
			};
			mediaRecorder.start();
			recording = true;
		} catch {
			error = 'no se pudo acceder al micrófono';
		}
	}

	async function sendVoice(blob: Blob) {
		sending = true;
		messages.push({ role: 'user', text: '🎤 …' });
		const idx = messages.length - 1;
		scrollToEnd();

		try {
			const res = await fetch('/api/assistant/voice-ui', { method: 'POST', body: blob });
			if (!res.ok) throw new Error(await res.text());

			const transcript = decodeURIComponent(res.headers.get('X-Transcript') ?? '');
			const reply = decodeURIComponent(res.headers.get('X-Reply') ?? '');
			messages[idx] = { role: 'user', text: transcript || '(sin transcripción)' };
			messages.push({ role: 'assistant', text: reply });

			audioEl.src = URL.createObjectURL(await res.blob());
			if (playReplyAudio) await audioEl.play();
		} catch (err) {
			messages.splice(idx, 1);
			error = err instanceof Error ? err.message : 'error al procesar el audio';
		} finally {
			sending = false;
			scrollToEnd();
		}
	}
</script>

<div class="flex h-full flex-col">
	<div class="flex items-center justify-end border-b border-[var(--color-border)] px-4 py-2">
		<button
			onclick={toggleReplyAudio}
			class="flex items-center gap-1.5 text-[10px] tracking-widest uppercase transition-colors {playReplyAudio
				? 'text-[var(--color-fg)]'
				: 'text-[var(--color-muted)]'}"
			aria-label="alternar audio de las respuestas"
		>
			{#if playReplyAudio}
				<SpeakerHighIcon size={13} weight="bold" />
			{:else}
				<SpeakerSlashIcon size={13} weight="bold" />
			{/if}
			audio
		</button>
	</div>

	<div bind:this={listEl} class="flex-1 space-y-3 overflow-y-auto px-4 py-4">
		{#if messages.length === 0}
			<p class="text-center text-xs text-[var(--color-muted)]">
				— prueba el asistente por texto{micSupported ? ' o voz' : ''} —
			</p>
			{#if !micSupported}
				<p class="text-center text-xs text-[var(--color-muted)]">
					(el micrófono del navegador requiere https o localhost)
				</p>
			{/if}
		{/if}

		{#each messages as msg}
			<div
				class="flex items-start gap-2 {msg.role === 'user' ? 'flex-row-reverse text-right' : ''}"
			>
				<span class="mt-0.5 shrink-0 text-[var(--color-muted)]">
					{#if msg.role === 'user'}
						<UserIcon size={14} weight="bold" />
					{:else}
						<RobotIcon size={14} weight="bold" />
					{/if}
				</span>
				<p
					class="max-w-[80%] border border-[var(--color-border)] px-3 py-2 text-sm {msg.role ===
					'user'
						? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
						: ''}"
				>
					{msg.text}
				</p>
			</div>
		{/each}

		{#if recording}
			<p class="text-xs text-red-500">● grabando…</p>
		{:else if sending}
			<p class="text-xs text-[var(--color-muted)]">el asistente está pensando…</p>
		{/if}

		{#if error}
			<p class="text-xs text-red-500">{error}</p>
		{/if}
	</div>

	<!-- svelte-ignore a11y_media_has_caption -->
	<audio bind:this={audioEl} class="hidden"></audio>

	<form
		onsubmit={(e) => {
			e.preventDefault();
			submit();
		}}
		class="flex items-end gap-2 border-t border-[var(--color-border)] px-4 py-3"
	>
		<textarea
			bind:value={input}
			onkeydown={onKeydown}
			rows="1"
			placeholder="pon la última de..."
			class="flex-1 resize-none border border-[var(--color-border)] bg-transparent px-3 py-2 text-sm outline-none"
		></textarea>
		<button
			type="button"
			onclick={toggleRecording}
			disabled={!micSupported || (sending && !recording)}
			class="flex h-9 w-9 shrink-0 items-center justify-center border border-[var(--color-border)] transition-colors disabled:opacity-40 {recording
				? 'bg-red-500 text-white'
				: ''}"
			aria-label={recording ? 'detener grabación' : 'grabar mensaje de voz'}
			title={micSupported ? undefined : 'el micrófono del navegador requiere https o localhost'}
		>
			{#if recording}
				<StopIcon size={16} weight="bold" />
			{:else}
				<MicrophoneIcon size={16} weight="bold" />
			{/if}
		</button>
		<button
			type="submit"
			disabled={sending || !input.trim()}
			class="flex h-9 w-9 shrink-0 items-center justify-center border border-[var(--color-border)] transition-colors disabled:opacity-40 {!sending &&
			input.trim()
				? 'bg-[var(--color-fg)] text-[var(--color-accent-fg)]'
				: ''}"
			aria-label="enviar"
		>
			<PaperPlaneRightIcon size={16} weight="bold" />
		</button>
	</form>
</div>
