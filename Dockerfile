FROM golang:1.24.6 AS builder

RUN apt-get update && apt-get install -y \
    libvips-dev \
    pkg-config \
    build-essential \
    pngquant

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1
RUN go build -o tinyimage-server .

FROM alpine:3.20

RUN apk add --no-cache vips pngquant

COPY --from=builder /app/tinyimage-server /usr/local/bin/tinyimage-server
RUN mkdir -p /config
COPY --from=builder /app/config.yaml /config/config.yaml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/tinyimage-server", "--config", "/config/config.yaml"]
