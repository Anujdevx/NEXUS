# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY citizen-reporting/go.mod citizen-reporting/go.sum citizen-reporting/
RUN --mount=type=cache,target=/go/pkg/mod cd citizen-reporting && go mod download
COPY shared shared
COPY citizen-reporting citizen-reporting
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd citizen-reporting && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/citizen-reporting ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/citizen-reporting /usr/local/bin/citizen-reporting
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/citizen-reporting"]
