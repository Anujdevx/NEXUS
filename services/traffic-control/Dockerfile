# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY traffic-control/go.mod traffic-control/go.sum traffic-control/
RUN --mount=type=cache,target=/go/pkg/mod cd traffic-control && go mod download
COPY shared shared
COPY traffic-control traffic-control
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd traffic-control && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/traffic-control ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/traffic-control /usr/local/bin/traffic-control
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/traffic-control"]
