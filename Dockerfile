# ── Build stage ──────────────────────────────────────────────────────────────
FROM oven/bun:1.3 AS builder

WORKDIR /app

# Install dependencies (frozen lockfile for reproducibility)
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source
COPY . .

# Build args for $env/static/private — only needed at build time by svelte-kit sync
# Real values are injected at runtime via docker-compose environment
ARG MPD_HOST=localhost
ARG MPD_PORT=6600
ARG SNAP_HOST=localhost
ARG SNAP_PORT=1780
ARG ORIGIN=http://localhost:3000
ARG PORT=3000

ENV MPD_HOST=$MPD_HOST \
    MPD_PORT=$MPD_PORT \
    SNAP_HOST=$SNAP_HOST \
    SNAP_PORT=$SNAP_PORT \
    ORIGIN=$ORIGIN \
    PORT=$PORT

RUN bun run build

# Prune to production-only dependencies
RUN bun install --frozen-lockfile --production

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM oven/bun:1.3-slim AS runner

WORKDIR /app

# Copy only what's needed to run
COPY --from=builder /app/build ./build
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

ENV NODE_ENV=production

EXPOSE 3000

CMD ["bun", "./build/index.js"]
