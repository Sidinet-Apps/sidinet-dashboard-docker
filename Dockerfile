FROM golang:1.23-alpine AS build
RUN apk add --no-cache build-base sqlite-dev
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=0.2.0-dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w -X github.com/sidinet/sidinet-dashboard-docker/internal/version.Version=${VERSION} -X github.com/sidinet/sidinet-dashboard-docker/internal/version.Commit=${COMMIT} -X github.com/sidinet/sidinet-dashboard-docker/internal/version.BuildDate=${BUILD_DATE}" -o /out/sidinet-dashboard ./cmd/dashboard
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sidinet-docker-proxy ./cmd/docker-proxy

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata sqlite-libs wget && addgroup -S sidinet && adduser -S -G sidinet -h /app sidinet
WORKDIR /app
COPY --from=build /out/sidinet-dashboard /app/sidinet-dashboard
COPY --from=build /out/sidinet-docker-proxy /app/sidinet-docker-proxy
RUN mkdir -p /data /tmp && chown -R sidinet:sidinet /data /tmp /app
USER sidinet
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/health || exit 1
ENTRYPOINT ["/app/sidinet-dashboard"]
