# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY notification-gateway/go.mod notification-gateway/go.sum notification-gateway/
RUN --mount=type=cache,target=/go/pkg/mod cd notification-gateway && go mod download
COPY shared shared
COPY notification-gateway notification-gateway
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd notification-gateway && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notification-gateway ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/notification-gateway /usr/local/bin/notification-gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/notification-gateway"]
