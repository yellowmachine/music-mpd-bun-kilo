import { SNAP_HOST, SNAP_PORT } from '$env/static/private';
import type { SnapClient, SnapVolume } from '$lib/mpd.types';
export type { SnapClient, SnapVolume };

const host = SNAP_HOST ?? 'localhost';
const port = parseInt(SNAP_PORT ?? '1780', 10);
const url = `ws://${host}:${port}/jsonrpc`;

// --- JSON-RPC helpers ---

let rpcId = 1;

async function rpc<T>(method: string, params?: unknown): Promise<T> {
	const body = JSON.stringify({ id: rpcId++, jsonrpc: '2.0', method, params });
	const res = await fetch(`http://${host}:${port}/jsonrpc`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
		body
	});
	const json = await res.json();
	if (json.error) throw new Error(`Snapserver RPC error: ${json.error.message}`);
	return json.result as T;
}

// --- State ---

let clients: SnapClient[] = [];

function parseClients(serverObj: Record<string, unknown>): SnapClient[] {
	const groups = (serverObj.groups as Record<string, unknown>[]) ?? [];
	const result: SnapClient[] = [];
	for (const group of groups) {
		const groupClients = (group.clients as Record<string, unknown>[]) ?? [];
		for (const c of groupClients) {
			const config = c.config as Record<string, unknown>;
			const h = c.host as Record<string, unknown>;
			const vol = (config.volume as SnapVolume) ?? { percent: 100, muted: false };
			result.push({
				id: String(c.id),
				connected: Boolean(c.connected),
				name: String(config.name || h.name || c.id),
				host: String(h.name ?? ''),
				ip: String(h.ip ?? ''),
				volume: { percent: vol.percent, muted: vol.muted }
			});
		}
	}
	return result;
}

export function getClients(): SnapClient[] {
	return clients;
}

// --- Pub/sub (reuses SSE broadcast from mpd.ts) ---

// We emit snap events through the same SSE channel using a 'snap' prefix
// mpd.ts exports subscribe/broadcast — we import broadcast directly
import { broadcast } from './mpd';

function broadcastClients() {
	broadcast('snap_clients', { clients });
}

// --- WebSocket idle connection to Snapserver ---

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

async function loadInitialState() {
	try {
		const result = await rpc<{ server: Record<string, unknown> }>('Server.GetStatus');
		clients = parseClients(result.server);
		broadcastClients();
	} catch (err) {
		console.error('[snap] Failed to load initial state:', err);
	}
}

function connectWebSocket() {
	if (ws) return;

	try {
		ws = new WebSocket(url);
	} catch (err) {
		console.error('[snap] WebSocket creation failed:', err);
		scheduleReconnect();
		return;
	}

	ws.addEventListener('open', async () => {
		console.log('[snap] WebSocket connected');
		await loadInitialState();
	});

	ws.addEventListener('message', (event) => {
		try {
			const msg = JSON.parse(event.data as string) as {
				method?: string;
				params?: Record<string, unknown>;
			};
			if (!msg.method) return; // it's a response, not a notification

			switch (msg.method) {
				case 'Client.OnVolumeChanged': {
					const { id, volume } = msg.params as { id: string; volume: SnapVolume };
					const client = clients.find((c) => c.id === id);
					if (client) {
						client.volume = volume;
						broadcastClients();
					}
					break;
				}
				case 'Client.OnConnect':
				case 'Client.OnDisconnect':
				case 'Client.OnNameChanged':
				case 'Server.OnUpdate':
					// Full reload on structural changes
					loadInitialState();
					break;
			}
		} catch {
			// ignore malformed messages
		}
	});

	ws.addEventListener('close', () => {
		console.warn('[snap] WebSocket closed, reconnecting...');
		ws = null;
		clients = [];
		scheduleReconnect();
	});

	ws.addEventListener('error', () => {
		ws?.close();
		ws = null;
		scheduleReconnect();
	});
}

function scheduleReconnect() {
	if (reconnectTimer) return;
	reconnectTimer = setTimeout(() => {
		reconnectTimer = null;
		connectWebSocket();
	}, 5000);
}

// Start eagerly at module load (skip during build)
if (!process.env.BUILDING) connectWebSocket();

// --- Public API ---

export async function setClientVolume(id: string, percent: number, muted: boolean): Promise<void> {
	await rpc('Client.SetVolume', { id, volume: { percent, muted } });
}

export async function setClientMute(id: string, muted: boolean): Promise<void> {
	const client = clients.find((c) => c.id === id);
	const percent = client?.volume.percent ?? 100;
	await rpc('Client.SetVolume', { id, volume: { percent, muted } });
}
