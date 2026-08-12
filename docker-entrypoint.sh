#!/bin/sh
set -e

YTDLP_URL="https://github.com/yt-dlp/yt-dlp-nightly-builds/releases/latest/download/yt-dlp"
YTDLP_BIN="/usr/local/bin/yt-dlp"

echo "[entrypoint] Updating yt-dlp to the latest nightly build..."
if curl -fsSL "$YTDLP_URL" -o "$YTDLP_BIN" && chmod a+rx "$YTDLP_BIN"; then
    echo "[entrypoint] yt-dlp updated to $(yt-dlp --version)"
else
    echo "[entrypoint] WARNING: failed to update yt-dlp, continuing with existing binary"
fi

exec "$@"
