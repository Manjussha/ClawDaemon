# ── Stage 1: Build ──────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.* ./
RUN go mod download

COPY . .

# No CGO needed — modernc.org/sqlite is pure Go
# This means we can use alpine without gcc/musl
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=$(cat VERSION 2>/dev/null || echo v0.1.0)" \
    -o clawdaemon .

# ── Stage 2: Runtime ─────────────────────────────────────────────
FROM alpine:3.19

# Node.js for Playwright browser scripts
RUN apk add --no-cache \
    ca-certificates \
    nodejs \
    npm \
    chromium \
    && npm install -g playwright \
    && npx playwright install chromium \
    && rm -rf /var/cache/apk/*

WORKDIR /app
COPY --from=builder /app/clawdaemon .

# Data directory
RUN mkdir -p /data/screenshots /data/character/skills
VOLUME ["/data"]

EXPOSE 8080

ENV WORK_DIR=/data
ENV PORT=8080

CMD ["./clawdaemon"]
