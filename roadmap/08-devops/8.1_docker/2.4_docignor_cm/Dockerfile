# STAGE 1: builder
FROM golang:1.25-alpine AS builder

RUN apk add --no-cahce git ca-certificates

WORKDIR /src

#Слой 1: манифесты. Меняются редко.
COPY go.mod go.sum* ./

#Слой 2: скачивание модулей с cache mount.
# Cache mount на /go/pkg/mod — модули сохраняются
# между билдами, не перекачиваются.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

#Слой 3: код. Меняется часто.
COPY . .

ARG VERSION=dev
ARG COMMIT=none

#Слой 4: сборка с двумя cache mounts.
# /go/pkg/mod           — модули (только чтение).
# /root/.cache/go-build — объектники компиляции.
# Оба сохраняются между билдами.
RUN --mount=type=cache,target=go/pkg/mod \
    --mount=type=cache,target=root/.cache/go-build \
    CGO_ENABLED=0 \
    go build \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -trimpath \
        -o /out/server .

# STAGE 2: prod \
FROM gcr.io/distroless/static-debian12:nonroot AS prod

ARG VERSION=dev
LABEL org.opencontainers.image.title="dockerignore-cache-demo"
LABEL org.opencontainers.image.version="${VERSION}"

COPY --from=builder /out/server /server

USER nonroot:nonroot
EXPSOE 8080
ENV APP_ENV=production
STOPSIGNAL SIGTERM

ENTRYPOINT["/server"]