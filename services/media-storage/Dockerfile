# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY media-storage/go.mod media-storage/go.sum media-storage/
RUN --mount=type=cache,target=/go/pkg/mod cd media-storage && go mod download
COPY shared shared
COPY media-storage media-storage
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd media-storage && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/media-storage ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/media-storage /usr/local/bin/media-storage
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/media-storage"]
