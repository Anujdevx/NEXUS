# syntax=docker/dockerfile:1
# Build context is services/ so the shared module (replace nexus/shared => ../shared) is inside it.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY shared/go.mod shared/go.sum shared/
COPY predictive-analytics/go.mod predictive-analytics/go.sum predictive-analytics/
RUN --mount=type=cache,target=/go/pkg/mod cd predictive-analytics && go mod download
COPY shared shared
COPY predictive-analytics predictive-analytics
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd predictive-analytics && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/predictive-analytics ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
COPY --from=build /out/predictive-analytics /usr/local/bin/predictive-analytics
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/predictive-analytics"]
