# ── Build stage ──────────────────────────────────────────────────────────────
FROM oven/bun:1 AS builder

WORKDIR /app

# Install dependencies (frozen lockfile for reproducibility)
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source and build
COPY . .
RUN bun run build

# Prune to production-only dependencies
RUN bun install --frozen-lockfile --production

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM oven/bun:1-slim AS runner

WORKDIR /app

# Copy only what's needed to run
COPY --from=builder /app/build ./build
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

ENV NODE_ENV=production

EXPOSE 3000

CMD ["bun", "./build/index.js"]
