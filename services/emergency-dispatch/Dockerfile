# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY emergency-dispatch/go.mod emergency-dispatch/go.sum emergency-dispatch/
RUN --mount=type=cache,target=/go/pkg/mod cd emergency-dispatch && go mod download
COPY shared shared
COPY emergency-dispatch emergency-dispatch
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd emergency-dispatch && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/emergency-dispatch ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/emergency-dispatch /usr/local/bin/emergency-dispatch
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/emergency-dispatch"]
