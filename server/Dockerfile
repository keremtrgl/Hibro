# Build: docker build -t hibro-server ./server
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hibro-server ./cmd/hibro-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 hibro && mkdir -p /data /collect && chown hibro /data /collect
USER hibro
COPY --from=build /out/hibro-server /usr/local/bin/hibro-server
ENV HIBRO_ADDR=:8080 HIBRO_DATA_DIR=/data TZ=Europe/Istanbul
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=120s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["hibro-server"]
