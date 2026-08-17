#!/usr/bin/env python3
"""Cliente de voz para el ReSpeaker USB Mic Array v2.0.

Corre en una Raspberry Pi dedicada (separada de la que aloja el backend).
Escucha en bucle, detecta la palabra clave con openWakeWord, graba la frase
que sigue, la envía a /api/assistant/voice del backend, y reproduce la
respuesta hablada por la salida de audio de esta Pi.

No hace STT/LLM/TTS aquí -- todo eso vive en el backend. Este script solo
hace wake-word + VAD + de/codificación de audio.
"""

from __future__ import annotations

import io
import os
import sys
import time
import wave
from urllib.parse import unquote

import numpy as np
import requests
import sounddevice as sd
import webrtcvad
from openwakeword.model import Model

SAMPLE_RATE = 16000
FRAME_SAMPLES = 1280  # 80ms @ 16kHz -- tamaño de chunk que espera openWakeWord
VAD_FRAME_MS = 30
SILENCE_MS_TO_STOP = 1000
MAX_UTTERANCE_S = 8

# El ReSpeaker USB Mic Array v2.0 transmite 6 canales: el 0 es la salida ya
# procesada por su DSP (AEC + beamforming), lista para ASR; el resto son
# los 4 micros en crudo más el canal de referencia de reproducción.
RESPEAKER_CHANNELS = 6
RESPEAKER_PROCESSED_CHANNEL = 0


def env(name: str, default: str | None = None) -> str | None:
	return os.environ.get(name, default)


def find_device(name_substring: str, kind: str) -> int | None:
	for idx, dev in enumerate(sd.query_devices()):
		channels = dev["max_input_channels"] if kind == "input" else dev["max_output_channels"]
		if name_substring.lower() in dev["name"].lower() and channels > 0:
			return idx
	return None


def record_utterance(stream: sd.InputStream, vad: webrtcvad.Vad) -> bytes:
	"""Graba desde `stream` hasta detectar silencio con VAD o MAX_UTTERANCE_S."""
	vad_frame_samples = int(SAMPLE_RATE * VAD_FRAME_MS / 1000)
	frames: list[np.ndarray] = []
	silence_ms = 0
	started = time.monotonic()

	while time.monotonic() - started < MAX_UTTERANCE_S:
		block, _ = stream.read(vad_frame_samples)
		mono = block[:, RESPEAKER_PROCESSED_CHANNEL]
		frames.append(mono.copy())

		is_speech = vad.is_speech(mono.tobytes(), SAMPLE_RATE)
		silence_ms = 0 if is_speech else silence_ms + VAD_FRAME_MS
		if silence_ms >= SILENCE_MS_TO_STOP and len(frames) > 3:
			break

	pcm = np.concatenate(frames)
	buf = io.BytesIO()
	with wave.open(buf, "wb") as wav:
		wav.setnchannels(1)
		wav.setsampwidth(2)  # int16
		wav.setframerate(SAMPLE_RATE)
		wav.writeframes(pcm.tobytes())
	return buf.getvalue()


def send_to_backend(wav_bytes: bytes, backend_url: str, token: str) -> bytes:
	res = requests.post(
		backend_url,
		data=wav_bytes,
		headers={"Content-Type": "audio/wav", "X-Assistant-Token": token},
		timeout=60,
	)
	res.raise_for_status()

	transcript = unquote(res.headers.get("X-Transcript", ""))
	reply = unquote(res.headers.get("X-Reply", ""))
	if transcript:
		print(f"[voz] escuchado: {transcript}")
	if reply:
		print(f"[voz] respuesta: {reply}")
	return res.content


def play_reply(wav_bytes: bytes, output_device: int | None) -> None:
	buf = io.BytesIO(wav_bytes)
	with wave.open(buf, "rb") as wav:
		data = np.frombuffer(wav.readframes(wav.getnframes()), dtype=np.int16)
		sd.play(data, samplerate=wav.getframerate(), device=output_device)
		sd.wait()


def main() -> None:
	backend_url = env("BACKEND_URL", "http://192.168.0.169:3000/api/assistant/voice")
	token = env("ASSISTANT_TOKEN")
	wakeword_model = env("WAKEWORD_MODEL", "hey_jarvis")
	wakeword_threshold = float(env("WAKEWORD_THRESHOLD", "0.5"))
	input_device_match = env("INPUT_DEVICE_MATCH", "ReSpeaker")
	output_device_match = env("OUTPUT_DEVICE_MATCH", "")

	if not token:
		sys.exit("Falta ASSISTANT_TOKEN en el entorno (debe coincidir con el del backend).")

	input_idx = find_device(input_device_match, "input")
	if input_idx is None:
		sys.exit(f"No se encontró un dispositivo de entrada que contenga '{input_device_match}'.")
	output_idx = find_device(output_device_match, "output") if output_device_match else None

	print(f"[voz] entrada: {sd.query_devices(input_idx)['name']} (#{input_idx})")
	print(f"[voz] salida: {'por defecto del sistema' if output_idx is None else sd.query_devices(output_idx)['name']}")
	print(f"[voz] backend: {backend_url}")

	oww = Model(wakeword_models=[wakeword_model])
	vad = webrtcvad.Vad(2)

	with sd.InputStream(
		device=input_idx,
		channels=RESPEAKER_CHANNELS,
		samplerate=SAMPLE_RATE,
		dtype="int16",
		blocksize=FRAME_SAMPLES,
	) as stream:
		print("[voz] escuchando... di la palabra clave")
		while True:
			block, _ = stream.read(FRAME_SAMPLES)
			mono = block[:, RESPEAKER_PROCESSED_CHANNEL]

			prediction = oww.predict(mono)
			score = prediction.get(wakeword_model, 0.0)
			if score < wakeword_threshold:
				continue

			print(f"[voz] wake word detectada (score={score:.2f}), grabando...")
			try:
				utterance = record_utterance(stream, vad)
				reply_wav = send_to_backend(utterance, backend_url, token)
				play_reply(reply_wav, output_idx)
			except requests.RequestException as err:
				print(f"[voz] error hablando con el backend: {err}")
			print("[voz] escuchando... di la palabra clave")


if __name__ == "__main__":
	main()
