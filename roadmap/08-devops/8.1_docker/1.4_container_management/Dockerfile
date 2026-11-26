# Stage 1: builder — где компилируется бинарник
FROM golang:1.26-alpine AS builder

RUN apk --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum* ./

RUN --mount=type=cahce,target=/go/pkg/mod \
    --mount=type=cahce,target=/root/.cache/go-build \
    go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -trimpath \
      -o /out/server .

# Stage 2: prod — финальный образ для прода \
FROM gcr.io/distroless/static-debian12:nonroot AS prod

ARG VERSION=dev
LABEL org.opencontainers.image.title="go-prod-docker"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/your/repo"

COPY --from=builder /out/server /server

USER nonroot:nonroot

EXPOSE 8080
ENV APP_ENV=production \
    LOG_LEVEL=info \
    PORT=8080

STOPSIGNAL SIGTERM

ENTRYPOINT ["/server"]

# Stage 3: debug — для отладки, с shell и инструментами
FROM alpine:3.19 AS debug

RUN apk add --no-cache \
    curl \
    strace \
    tcpdump \
    jq \
    ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /out/server /app/server
RUN chown -R appuser:appuser /app

USER appuser

ENV APP_ENV=debug \
    LOG_LEVEL=debug \
    PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -fs5 http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]