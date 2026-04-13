# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=$(git describe --tags --always)" \
    -o bin/server ./cmd/server

# ── Stage 2: Development ──────────────────────────────────────────────────────
FROM golang:1.22-alpine AS development

WORKDIR /app

RUN go install github.com/air-verse/air@latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

COPY go.mod go.sum ./
RUN go mod download

EXPOSE 8080 2345

CMD ["air", "-c", ".air.toml"]

# ── Stage 3: Production ───────────────────────────────────────────────────────
FROM scratch AS runtime

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/bin/server /server

EXPOSE 8080

USER 65534:65534

ENTRYPOINT ["/server"]