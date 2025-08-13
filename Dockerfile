FROM golang:1.24.6 AS builder

# 安装 libvips 依赖
RUN apt-get update && apt-get install -y \
    libvips-dev \
    pkg-config \
    build-essential \
    pngquant

WORKDIR /app
COPY . .
ENV CGO_ENABLED=0
# 编译 Go 项目
RUN go build -o tinyimage-server .

# 生产镜像
FROM alpine

RUN apk --update --no-cache add --no-cache vips-dev pngquant && rm -rf /var/cache/apk/*

COPY --from=builder /app/tinyimage-server /usr/local/bin/tinyimage-server

EXPOSE 8080
ENTRYPOINT ["tinyimage-server"]
