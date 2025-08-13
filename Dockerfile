FROM golang:1.21 AS builder

# 安装 libvips 依赖
RUN apt-get update && apt-get install -y \
    libvips-dev \
    pkg-config \
    build-essential \
    pngquant

WORKDIR /app
COPY . .

# 编译 Go 项目
RUN go build -o image-compressor .

# 生产镜像
FROM debian:bullseye-slim

RUN apk update && apk add --no-cache vips-dev pngquant && rm -rf /var/cache/apk/*

COPY --from=builder /app/image-compressor /usr/local/bin/image-compressor

EXPOSE 8080
ENTRYPOINT ["image-compressor"]
