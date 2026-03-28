# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -o /bin/server ./cmd/server/
RUN CGO_ENABLED=1 go build -o /bin/query-service ./cmd/query-service/
RUN CGO_ENABLED=1 go build -o /bin/portal-service ./cmd/portal-service/
RUN CGO_ENABLED=1 go build -o /bin/worker-service ./cmd/worker-service/
RUN go build -o /bin/gateway ./cmd/gateway/

# ── Runtime images ────────────────────────────────────────────────────────────

FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates

FROM base AS gateway
COPY --from=builder /bin/gateway /usr/local/bin/gateway
ENTRYPOINT ["gateway"]

FROM base AS query-service
COPY --from=builder /bin/query-service /usr/local/bin/query-service
ENTRYPOINT ["query-service", "--env=staging"]

FROM base AS portal-service
COPY --from=builder /bin/portal-service /usr/local/bin/portal-service
ENTRYPOINT ["portal-service", "--env=staging"]

FROM base AS worker-service
COPY --from=builder /bin/worker-service /usr/local/bin/worker-service
ENTRYPOINT ["worker-service", "--env=staging"]

FROM base AS server
COPY --from=builder /bin/server /usr/local/bin/server
ENTRYPOINT ["server", "--env=staging"]
