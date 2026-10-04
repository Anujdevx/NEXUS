# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY graph-engine/go.mod graph-engine/go.sum graph-engine/
RUN --mount=type=cache,target=/go/pkg/mod cd graph-engine && go mod download
COPY shared shared
COPY graph-engine graph-engine
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd graph-engine && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/graph-engine ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/graph-engine /usr/local/bin/graph-engine
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/graph-engine"]
