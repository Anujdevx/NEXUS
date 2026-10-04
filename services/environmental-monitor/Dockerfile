# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY environmental-monitor/go.mod environmental-monitor/go.sum environmental-monitor/
RUN --mount=type=cache,target=/go/pkg/mod cd environmental-monitor && go mod download
COPY shared shared
COPY environmental-monitor environmental-monitor
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd environmental-monitor && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/environmental-monitor ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/environmental-monitor /usr/local/bin/environmental-monitor
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/environmental-monitor"]
