FROM golang:1.24.6-alpine AS builder

RUN apk add --no-cache build-base musl-dev pkgconfig vips-dev pngquant

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1
ENV CC=musl-gcc
RUN go build -o tinyimage-server .

FROM alpine:3.20

RUN apk add --no-cache vips pngquant

COPY --from=builder /app/tinyimage-server /usr/local/bin/tinyimage-server
RUN mkdir -p /config
COPY --from=builder /app/config.yaml /config/config.yaml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/tinyimage-server", "--config", "/config/config.yaml"]
