import Anthropic from '@anthropic-ai/sdk';
import { env } from '$env/dynamic/private';
import { getClient, search as searchLibrary } from '$lib/server/mpd';
import { getClients, setClientVolume, setClientMute } from '$lib/server/snap';

const anthropic = new Anthropic({ apiKey: env.ANTHROPIC_API_KEY });
const MODEL = 'claude-haiku-4-5-20251001';

const tools: Anthropic.Tool[] = [
	{
		name: 'search_library',
		description:
			'Busca en la biblioteca por título/artista/álbum. Úsalo para desambiguar antes de actuar.',
		input_schema: { type: 'object', properties: { query: { type: 'string' } }, required: ['query'] }
	},
	{
		name: 'play_song',
		description: 'Vacía la cola y reproduce la canción que mejor coincide con la búsqueda.',
		input_schema: { type: 'object', properties: { query: { type: 'string' } }, required: ['query'] }
	},
	{
		name: 'play_album',
		description: 'Vacía la cola y reproduce un álbum completo en orden de pista.',
		input_schema: {
			type: 'object',
			properties: { album: { type: 'string' }, artist: { type: 'string' } },
			required: ['album']
		}
	},
	{
		name: 'queue_song',
		description: 'Añade una canción al final de la cola sin interrumpir lo que suena.',
		input_schema: { type: 'object', properties: { query: { type: 'string' } }, required: ['query'] }
	},
	{
		name: 'play_playlist',
		description: 'Reproduce una playlist guardada por nombre exacto.',
		input_schema: { type: 'object', properties: { name: { type: 'string' } }, required: ['name'] }
	},
	{
		name: 'playback_control',
		description: 'Controla la reproducción actual.',
		input_schema: {
			type: 'object',
			properties: {
				action: { type: 'string', enum: ['play', 'pause', 'resume', 'stop', 'next', 'prev'] }
			},
			required: ['action']
		}
	},
	{
		name: 'set_volume',
		description: 'Ajusta el volumen general (0-100).',
		input_schema: {
			type: 'object',
			properties: { percent: { type: 'number' } },
			required: ['percent']
		}
	},
	{
		name: 'set_random',
		description: 'Activa o desactiva la reproducción aleatoria.',
		input_schema: {
			type: 'object',
			properties: { enabled: { type: 'boolean' } },
			required: ['enabled']
		}
	},
	{
		name: 'get_now_playing',
		description: 'Devuelve la canción actual y el estado de reproducción.',
		input_schema: { type: 'object', properties: {} }
	},
	{
		name: 'list_rooms',
		description: 'Lista las salas/altavoces Snapcast conectados y su volumen.',
		input_schema: { type: 'object', properties: {} }
	},
	{
		name: 'set_room_volume',
		description: 'Ajusta el volumen de una sala/altavoz concreto por nombre.',
		input_schema: {
			type: 'object',
			properties: { room: { type: 'string' }, percent: { type: 'number' } },
			required: ['room', 'percent']
		}
	},
	{
		name: 'mute_room',
		description: 'Silencia o reactiva una sala/altavoz.',
		input_schema: {
			type: 'object',
			properties: { room: { type: 'string' }, muted: { type: 'boolean' } },
			required: ['room', 'muted']
		}
	}
];

const PLAYBACK_ACTIONS = ['play', 'pause', 'resume', 'stop', 'next', 'prev'] as const;
type PlaybackAction = (typeof PLAYBACK_ACTIONS)[number];

function findRoom(name: string) {
	const needle = name.toLowerCase();
	return getClients().find((c) => c.name.toLowerCase().includes(needle));
}

async function runTool(name: string, input: Record<string, unknown>): Promise<unknown> {
	const mpd = await getClient();

	switch (name) {
		case 'search_library':
			return searchLibrary(String(input.query)).slice(0, 8);

		case 'play_song': {
			const hit = searchLibrary(String(input.query))[0];
			if (!hit) return { error: 'no encontrado' };
			await mpd.api.queue.clear();
			await mpd.api.queue.add(hit.file);
			await mpd.api.playback.play();
			return { playing: hit };
		}

		case 'play_album': {
			const album = String(input.album);
			const artist = input.artist ? String(input.artist) : undefined;
			const tracks = searchLibrary(`${album} ${artist ?? ''}`.trim(), 200)
				.filter(
					(s) =>
						s.album?.toLowerCase() === album.toLowerCase() &&
						(!artist || s.artist?.toLowerCase().includes(artist.toLowerCase()))
				)
				.sort((a, b) => Number(a.track ?? 0) - Number(b.track ?? 0));
			if (!tracks.length) return { error: 'álbum no encontrado' };
			await mpd.api.queue.clear();
			for (const t of tracks) await mpd.api.queue.add(t.file);
			await mpd.api.playback.play();
			return { queued: tracks.length, album };
		}

		case 'queue_song': {
			const hit = searchLibrary(String(input.query))[0];
			if (!hit) return { error: 'no encontrado' };
			await mpd.api.queue.add(hit.file);
			return { queued: hit };
		}

		case 'play_playlist': {
			const playlistName = String(input.name);
			await mpd.api.queue.clear();
			await mpd.api.playlists.load(playlistName);
			await mpd.api.playback.play();
			return { playing_playlist: playlistName };
		}

		case 'playback_control': {
			const action = input.action as PlaybackAction;
			if (!PLAYBACK_ACTIONS.includes(action))
				return { error: `acción desconocida: ${String(input.action)}` };
			await mpd.api.playback[action]();
			return { ok: true, action };
		}

		case 'set_volume': {
			const percent = Number(input.percent);
			await mpd.api.playback.setvol(String(percent));
			return { volume: percent };
		}

		case 'set_random': {
			const enabled = Boolean(input.enabled);
			await mpd.api.playback.random(String(enabled));
			return { random: enabled };
		}

		case 'get_now_playing':
			return { status: await mpd.api.status.get(), song: await mpd.api.status.currentsong() };

		case 'list_rooms':
			return getClients().map((c) => ({ id: c.id, name: c.name, volume: c.volume }));

		case 'set_room_volume': {
			const room = String(input.room);
			const percent = Number(input.percent);
			const client = findRoom(room);
			if (!client) return { error: 'sala no encontrada' };
			await setClientVolume(client.id, percent, client.volume.muted);
			return { room: client.name, volume: percent };
		}

		case 'mute_room': {
			const room = String(input.room);
			const muted = Boolean(input.muted);
			const client = findRoom(room);
			if (!client) return { error: 'sala no encontrada' };
			await setClientMute(client.id, muted);
			return { room: client.name, muted };
		}

		default:
			return { error: `tool desconocida: ${name}` };
	}
}

const SYSTEM_PROMPT = `Eres el asistente de voz de un equipo de música doméstico (MPD + Snapcast multi-sala).
Responde siempre en español, de forma breve (1-2 frases), como si hablaras en voz alta.
Usa las tools para actuar; no inventes canciones, álbumes ni salas que no aparezcan en los resultados de las tools.
Si una búsqueda no tiene una coincidencia clara, usa search_library primero y elige el resultado más razonable.`;

export interface AssistantResult {
	text: string;
	history: Anthropic.MessageParam[];
}

export async function runAssistant(
	userText: string,
	history: Anthropic.MessageParam[] = []
): Promise<AssistantResult> {
	const messages: Anthropic.MessageParam[] = [...history, { role: 'user', content: userText }];

	// Evita loops de tool-calling descontrolados (p.ej. una tool que devuelve
	// siempre error y el modelo insiste en reintentar).
	for (let i = 0; i < 8; i++) {
		const response = await anthropic.messages.create({
			model: MODEL,
			max_tokens: 1024,
			system: SYSTEM_PROMPT,
			tools,
			messages
		});

		if (response.stop_reason !== 'tool_use') {
			const text = response.content.find((b) => b.type === 'text')?.text ?? '';
			messages.push({ role: 'assistant', content: response.content });
			return { text, history: messages };
		}

		messages.push({ role: 'assistant', content: response.content });

		const toolResults: Anthropic.ToolResultBlockParam[] = [];
		for (const block of response.content) {
			if (block.type === 'tool_use') {
				const result = await runTool(block.name, block.input as Record<string, unknown>);
				toolResults.push({
					type: 'tool_result',
					tool_use_id: block.id,
					content: JSON.stringify(result)
				});
			}
		}
		messages.push({ role: 'user', content: toolResults });
	}

	return { text: 'No he podido completar la petición, ¿puedes reformularla?', history: messages };
}
