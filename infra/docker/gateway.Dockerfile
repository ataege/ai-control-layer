# Go gateway. Build context: the repository root. No Node.js in any stage.

FROM golang:1.27-alpine AS build
# Fail instead of downloading another toolchain; produce a static binary.
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src
# Download modules first so this layer is reused until go.mod or go.sum change.
COPY services/gateway/go.mod services/gateway/go.sum ./
RUN go mod download
COPY services/gateway/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM alpine:3.24 AS runtime
RUN addgroup -S -g 10001 gateway && adduser -S -u 10001 -G gateway gateway
COPY --from=build /out/gateway /usr/local/bin/gateway
# Listen on all container interfaces; the host default stays 127.0.0.1.
ENV GATEWAY_HOST=0.0.0.0 GATEWAY_PORT=8080
USER gateway
EXPOSE 8080
# Exec form: the gateway is PID 1 and receives stop signals directly.
ENTRYPOINT ["/usr/local/bin/gateway"]
