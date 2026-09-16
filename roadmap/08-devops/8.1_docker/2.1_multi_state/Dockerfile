# STAGE 1: base — общие настройки для всех stage
FROM golang:1.25-alpine AS base

RUN apk --no-cache git ca-certificates

WORKDIR /src

# STAGE 2: deps — скачиваем зависимости отдельным слоем
# Этот слой кэшируется, пока не изменится go.mod/go.sum.
FROM base AS deps

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# STAGE 3: builder — компилируем бинарник
# Копируем код ПОСЛЕ deps, чтобы изменение кода
# не инвалидировало слой с зависимостями.
FROM deps AS builder

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
    -o /out/server

RUN /out/server --version 2>/dev/null || true    

# STAGE 4: prod — финальный образ для продакшена
# Только бинарник. Ничего лишнего.
FROM gcr.io/distroless/static-debian12:nonroot AS prod

ARG VERSION=dev
LABEL org.opencontainers.image.title="multistage-demo"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.description="Production multi-stage image"

COPY --from=builder /out/sever /server

USER nonroot:nonroot
EXPOSE 8080
ENV APP_ENV=Production
STOPSIGNAL SIGTERM

ENTRYPOINT [ "/server" ]

# STAGE 5: debug — для отладки в проде через docker exec
# Тот же бинарник, но с shell и инструментами.
FROM alpine:3.19 AS debug

RUN apk add --no-cache \
    curl \
    strace \
    tcpdump \
    jq \
    ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder --chown=1000:1000 /out/server /app/server

USER appuser
EXPOSE 8080
ENV APP_ENV=debug
ENV LOG_LEVEL=debug

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]