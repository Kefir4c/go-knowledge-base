FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl \
    && adduser -D -u 1000 appuser
COPY --chown=appuser:appuser --from=builder /out/server /app/server
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1
# EXEC-ФОРМА. PID 1 = /app/server. SIGTERM доходит напрямую.
ENTRYPOINT ["/app/server"]