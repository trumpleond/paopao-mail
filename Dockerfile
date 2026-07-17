# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/paopao-api \
    ./cmd/server

# ── runtime ──────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/paopao-api /app/paopao-api
COPY web /app/web

RUN mkdir -p /data && chown -R appuser:appuser /app /data

USER appuser

ENV ADDR=:8080 \
    DB_PATH=/data/paopao.db \
    UPSTREAM_BASE=https://query.paopaodw.com \
    UPSTREAM_TIMEOUT_SEC=30

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=8s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1

ENTRYPOINT ["/app/paopao-api"]
