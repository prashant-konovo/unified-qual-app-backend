# ---- Build Stage ----
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /unified-qual-api ./cmd/server

# ---- Runtime Stage ----
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser
COPY --from=builder /unified-qual-api /usr/local/bin/unified-qual-api
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["unified-qual-api"]
