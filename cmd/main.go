package main

import (
	"fmt"
	"image-processor/config"
	"image-processor/internal/handler"
	"image-processor/internal/middleware"
	"image-processor/internal/service"
	"log"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	// 加载配置
	config.Load()

	// 初始化服务
	service.InitSemaphore()

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{
		BodyLimit: int(config.MaxUploadSizeBytes),
	})

	// 注册 HTTP 路由
	handler.RegisterHTTPRoutes(app)

	// 注册 WebSocket 路由
	app.Get("/ws", websocket.New(handler.WebSocketHandler))

	// 启动清理中间件
	middleware.StartCleanupRoutine()

	// 启动服务
	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("Starting server on %s with version %s", addr, handler.Version)
	log.Fatal(app.Listen(addr))
}