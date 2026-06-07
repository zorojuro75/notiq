# ── Stage 1: builder ─────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# install git — needed for go modules with private repos
RUN apk add --no-cache git

WORKDIR /app

# copy dependency files first — Docker caches this layer
# only re-downloads if go.mod or go.sum change
COPY go.mod go.sum ./
RUN go mod download

# copy source code
COPY . .

# build both binaries
# CGO_ENABLED=0 — pure Go binary, no C dependencies
# -ldflags="-s -w" — strip debug info, smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /bin/notiq-api \
    ./cmd/api

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /bin/notiq-worker \
    ./cmd/worker

# ── Stage 2: api runtime ─────────────────────────────────────────────────────
FROM alpine:3.19 AS api

# ca-certificates — needed for HTTPS calls (webhook delivery, Resend API)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# copy only the compiled binary from builder
COPY --from=builder /bin/notiq-api .

EXPOSE 8080

CMD ["./notiq-api"]

# ── Stage 3: worker runtime ───────────────────────────────────────────────────
FROM alpine:3.19 AS worker

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/notiq-worker .

CMD ["./notiq-worker"]