# STAGE 1: builder — общий для всех образов
FROM golang:1.25-alpine AS builder

RUN apk add --no-cahce git ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cahce,terger=/go/pkg/mod \
go mod donwload

COPY . .

ARG VERSION=dev
ARG COMMIT=none

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -trimpath \
      -o /out/server .

# STAGE 2: ubuntu — полный Linux, 80 МБ базы
FROM ubuntu:22.04 AS ubuntu

RUN apt-get update && apt-get install -y \
    ca-certificates tzdata curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -u 1000 -m appuser

COPY --from=builder --chown=1000:1000 /out/server /server

USER appuser
EXPOSE 8080
ENV APP_ENV=production

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/server"]      

# STAGE 3: alpine — минимализм с shell, 5 МБ базы
FROM alpine:3.19 AS alpine

RUN apk add --no-cache ca-certificates tzdata curl \
    &&adduser -D -u 1000 appuser

COPY --from=builder --chown=1000:1000 /out/server /server

USER appuser
EXPOSE 8080
ENV APP_ENV=production

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["/server"]

# STAGE 4: distroless — золотая середина, 2 МБ базы
FROM gcr.io/distroless/static-debian12:nonroot AS distroless

COPY --from=builder /out/server /server

USER nonroot:nonroot
EXPOSE 8080
ENV APP_ENV=production
STOPSIGNAL SIGTERM

ENTRYPOINT [ "/server" ]

# STAGE 5: scratch — абсолютный минимум, 0 МБ базы
FROM scratch AS scratch

# Копируем ca-certs из builder'а — иначе HTTPS не работает.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt \
                    /etc/ssl/certs/ca-certificates.crt

# Копируем tzdata — иначе time.LoadLocation падает.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копируем /etc/passwd и /etc/group для nonroot-пользователя.
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Копируем бинарник.
COPY --from=builder /out/server /server

# В scratch USER только по UID, если нет /etc/passwd.
# У нас passwd скопирован — можно по имени.
USER appuser

EXPOSE 8080
ENV APP_ENV=production

ENTRYPOINT ["/server"]