# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY identity-access/go.mod identity-access/go.sum identity-access/
RUN --mount=type=cache,target=/go/pkg/mod cd identity-access && go mod download
COPY shared shared
COPY identity-access identity-access
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd identity-access && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/identity-access ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/identity-access /usr/local/bin/identity-access
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/identity-access"]
