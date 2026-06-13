package handler

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"mime/multipart"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"image-processor/config"
	"image-processor/internal/model"
	"image-processor/internal/service"
	"image-processor/internal/utils"
)

const Version = "v0.0.3"

func RegisterHTTPRoutes(app *fiber.App) {
	app.Get("/", handleRoot)
	app.Post("/upload", handleUpload)
	app.Get("/queue/:md5", handleQueueStatus)
	app.Get("/status/:md5", handleStatus)
	app.Get("/download/:md5", handleDownload)
}

func handleRoot(c *fiber.Ctx) error {
	response := fiber.Map{
		"version":               Version,
		"download_url":          config.AppConfig.Download.DownloadUrl,
		"max_upload_size_bytes": config.MaxUploadSizeBytes,
		"max_concurrent_tasks":  config.AppConfig.Upload.MaxConcurrentTasks,
		"max_age":               config.AppConfig.Upload.MaxAge,
	}
	return c.JSON(response)
}

func handleUpload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("picture")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Missing file")
	}
	if fileHeader.Size > config.MaxUploadSizeBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).SendString("File too large")
	}

	format := utils.NormalizeFormat(c.FormValue("format", "webp"))
	if !utils.IsSupportedFormat(format) {
		return c.Status(fiber.StatusBadRequest).SendString("Unsupported format")
	}

	var quality int
	if format == "jpg" {
		quality, _ = strconv.Atoi(c.FormValue("quality", "80"))
		if quality <= 0 || quality > 100 {
			quality = 80
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open file")
	}
	defer closeFile(file)

	inputBuffer, err := io.ReadAll(file)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file")
	}

	hash := md5.Sum(inputBuffer)
	md5Str := hex.EncodeToString(hash[:])
	key := model.ProcessKey{MD5: md5Str, Format: format, Quality: quality}

	if service.IsProcessed(key) {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Image already processed for this format/quality",
			"md5":     md5Str,
			"format":  format,
			"quality": quality,
		})
	}

	service.EnqueueTask(key, fileHeader.Filename)

	c.Set("Content-Type", "application/json")
	_ = c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Your image is queued for processing",
		"md5":     md5Str,
		"format":  format,
		"quality": quality,
	})

	go service.ProcessTask(key, inputBuffer)

	return nil
}

func handleQueueStatus(c *fiber.Ctx) error {
	md5Str := c.Params("md5")
	items := service.GetQueueByMD5(md5Str)
	if len(items) == 0 {
		return c.Status(fiber.StatusNotFound).SendString("MD5 not found in queue")
	}
	return c.JSON(items)
}

func handleStatus(c *fiber.Ctx) error {
	md5Str := c.Params("md5")
	items := service.GetStatusByMD5(md5Str)
	if len(items) == 0 {
		return c.Status(fiber.StatusNotFound).SendString("MD5 not found")
	}
	return c.JSON(items)
}

func handleDownload(c *fiber.Ctx) error {
	md5Str := c.Params("md5")
	outputPath := utils.FindOutputFile(md5Str)

	if outputPath == "" {
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}

	return c.SendFile(outputPath)
}

func closeFile(file multipart.File) {
	if err := file.Close(); err != nil {
		log.Printf("Warning: failed to close file: %v", err)
	}
}