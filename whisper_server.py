import os
import tempfile

from fastapi import FastAPI, Request
from faster_whisper import WhisperModel

MODEL_NAME = os.environ.get("WHISPER_MODEL", "small")
LANGUAGE = os.environ.get("WHISPER_LANGUAGE", "es")

model = WhisperModel(MODEL_NAME, device="cpu", compute_type="int8")
app = FastAPI()


@app.get("/health")
async def health():
    return {"ok": True, "model": MODEL_NAME}


@app.post("/transcribe")
async def transcribe(request: Request):
    audio_bytes = await request.body()
    if not audio_bytes:
        return {"text": ""}

    with tempfile.NamedTemporaryFile(suffix=".wav") as tmp:
        tmp.write(audio_bytes)
        tmp.flush()
        segments, _ = model.transcribe(tmp.name, language=LANGUAGE)
        text = "".join(segment.text for segment in segments).strip()

    return {"text": text}
