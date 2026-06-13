package handler

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/h2non/bimg"

	"github.com/humanutopia/tinyimage-server/config"
	"github.com/humanutopia/tinyimage-server/internal/model"
	"github.com/humanutopia/tinyimage-server/internal/service"
	"github.com/humanutopia/tinyimage-server/internal/utils"
)

func WebSocketHandler(c *websocket.Conn) {
	defer closeWebSocket(c)
	writeLock := &sync.Mutex{}

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		var req model.WSUploadRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "error", Message: "invalid_json"})
			continue
		}

		format := utils.NormalizeFormat(req.Format)
		if !utils.IsSupportedFormat(format) {
			wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "unsupported_format", Message: "only webp/png/jpg supported"})
			continue
		}

		data, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "decode_error", Message: "invalid base64"})
			continue
		}
		if int64(len(data)) > config.MaxUploadSizeBytes {
			wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "size_exceeded", Message: "file too large"})
			continue
		}

		sum := md5.Sum(data)
		md5Str := hex.EncodeToString(sum[:])
		quality := req.Quality
		if format == "jpg" {
			if quality <= 0 || quality > 100 {
				quality = 80
			}
		} else {
			quality = 0
		}
		key := model.ProcessKey{MD5: md5Str, Format: format, Quality: quality}

		if service.IsProcessed(key) {
			path := utils.OutputPathForKey(key)
			if buf, err := os.ReadFile(path); err == nil {
				wsWriteJSON(writeLock, c, model.WSProgressMessage{
					Status: "done", MD5: md5Str, File: base64.StdEncoding.EncodeToString(buf), Format: format, Quality: quality,
				})
				continue
			}
		}

		service.EnqueueTask(key, req.Filename)
		wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "queued", MD5: md5Str, Format: format, Quality: quality})

		go processWSTask(key, data, writeLock, c)
	}
}

func processWSTask(key model.ProcessKey, buffer []byte, writeLock *sync.Mutex, c *websocket.Conn) {
	semaphore := service.GetSemaphore()
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	service.MarkProcessing(key)
	wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "processing", MD5: key.MD5, Format: key.Format, Quality: key.Quality})

	if err := utils.EnsureOutputDir(); err != nil {
		service.MarkFailed(key)
		wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "error", MD5: key.MD5, Message: "mkdir_failed"})
		return
	}

	var outputBuffer []byte
	var err error

	switch key.Format {
	case "png":
		outputBuffer, err = service.ProcessPNGWithPngquant(buffer)
	case "jpg":
		outputBuffer, err = service.ProcessWithBimgQuality(buffer, bimg.JPEG, key.Quality)
	case "webp":
		outputBuffer, err = service.ProcessWebPWithBimg(buffer)
	default:
		outputBuffer, err = service.ProcessWebPWithBimg(buffer)
	}

	if err != nil {
		log.Printf("Image processing failed for %s: %v", key.MD5, err)
		service.MarkFailed(key)
		wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "error", MD5: key.MD5, Message: "processing_failed"})
		return
	}

	outPath := utils.OutputPathForKey(key)
	if err := os.WriteFile(outPath, outputBuffer, 0644); err != nil {
		log.Printf("Failed to save image %s: %v", key.MD5, err)
		service.MarkFailed(key)
		wsWriteJSON(writeLock, c, model.WSProgressMessage{Status: "error", MD5: key.MD5, Message: "save_failed"})
		return
	}

	service.MarkDone(key)

	wsWriteJSON(writeLock, c, model.WSProgressMessage{
		Status:  "done",
		MD5:     key.MD5,
		File:    base64.StdEncoding.EncodeToString(outputBuffer),
		Format:  key.Format,
		Quality: key.Quality,
	})
}

func wsWriteJSON(lock *sync.Mutex, c *websocket.Conn, v any) {
	lock.Lock()
	defer lock.Unlock()
	_ = c.WriteJSON(v)
}

func closeWebSocket(c *websocket.Conn) {
	if err := c.Close(); err != nil {
		log.Printf("Warning: failed to close websocket: %v", err)
	}
}