# STAGE 1: builder
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates
WORKDIR /src

#Слой 1: манифесты workspace.
COPY go.work go.work.sum* ./

#Слой 2: манифесты нужных модулей.
# Только order + его shared-зависимость.
COPY services/order/go.mod services/order/go.sum* ./services/order/
COPY shared/logging/go.mod shared/logging/go.sum* ./shared/logging/

#Слой 3: sync + download.
RUN --mount=type=cache,target=/go/pkg/mod \
    go work sync && go mod download

#Слой 4: код только нужных модулей.
COPY services/order ./services/order
COPY shared/logging ./shared/logging

ARG VERSION=dev
ARG COMMIT=none

#Слой 5: сборка.
RUN --mount=type=cache,target=/go/pkg/mode \
    --mount=type=cache,target=/root/./cache/go-build \
    CGO_ENABLED=0 \
    go build \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -trimpath \
        -o /out/server \
        ./services/order/cmd/order

# STAGE 2: prod
FROM gcr.io/distroless/static-debian12:nonroot AS prod

ARG VERSION=dev
LABEL org.opencontainers.image.title="order-service"
LABEL org.opencontainers.image.version="${VERSION}"

COPY --from=builder /out/server /server

USER nonroot:nonroot
EXPOSE 8081
ENV APP_ENV=production
STOPSIGNAL SIGTERM

ENTRYPOINT["/server"]