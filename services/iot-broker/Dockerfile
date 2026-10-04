# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY iot-broker/go.mod iot-broker/go.sum iot-broker/
RUN --mount=type=cache,target=/go/pkg/mod cd iot-broker && go mod download
COPY shared shared
COPY iot-broker iot-broker
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd iot-broker && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/iot-broker ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/iot-broker /usr/local/bin/iot-broker
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/iot-broker"]
