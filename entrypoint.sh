#!/bin/sh
set -e

FIFO_PATH="/shared/snapfifo"

# Crea el directorio si no existe
mkdir -p "$(dirname "$FIFO_PATH")"

# Crea la FIFO si no existe
if [ ! -p "$FIFO_PATH" ]; then
    mkfifo "$FIFO_PATH"
    chmod 660 "$FIFO_PATH"
fi

# Lanza MPD con la configuración estándar
exec mpd --no-daemon /etc/mpd.conf
