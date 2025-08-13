package main

import (
    "bytes"
    "crypto/md5"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "image"
    _ "image/gif"
    "image/png"
    _ "image/jpeg"
    "io"
    "log"
    "os"
    "os/exec"
    "strings"
    "sync"
    "time"
    "strconv"
    _ "golang.org/x/image/webp"
    _ "golang.org/x/image/bmp"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/websocket/v2"
    "github.com/h2non/bimg"
    "github.com/spf13/viper"
)

// ===========================
// 结构优化与重复处理控制
// ===========================

// 统一标识处理任务的结构体
type ProcessKey struct {
    MD5     string // 图片hash
    Format  string // 目标格式
    Quality int    // jpg的质量(其它格式为0)
}

// processedTasks 记录已处理过的任务
var (
    queue        = make(map[ProcessKey]string) // 任务key -> filename
    statusMap    = make(map[ProcessKey]string) // 任务key -> status
    queueMutex   = &sync.Mutex{}
    processed    = make(map[ProcessKey]bool)   // 已处理过的任务key
    toDelete     = make([]string, 0)
    deleteLock   = &sync.Mutex{}
    count        = 0

    AppConfig          Config
    MaxUploadSizeBytes int64
    taskSemaphore      chan struct{}
)

type Config struct {
    Server struct {
        Port int `mapstructure:"port"`
    } `mapstructure:"server"`
    Upload struct {
        MaxUploadSize      string `mapstructure:"max_upload_size"`
        MaxConcurrentTasks int    `mapstructure:"max_concurrent_tasks"`
    } `mapstructure:"upload"`
}

// WebSocket 消息体
type WSUploadRequest struct {
    Filename string `json:"filename"`
    Format   string `json:"format"`
    Data     string `json:"data"`
    Quality  int    `json:"quality,omitempty"`
}

type WSProgressMessage struct {
    Status  string `json:"status"`
    MD5     string `json:"md5,omitempty"`
    Message string `json:"message,omitempty"`
    File    string `json:"file,omitempty"`
    Format  string `json:"format,omitempty"`
    Quality int    `json:"quality,omitempty"`
}

func main() {
    loadConfig()
    if AppConfig.Upload.MaxConcurrentTasks <= 0 {
        AppConfig.Upload.MaxConcurrentTasks = 3
    }
    taskSemaphore = make(chan struct{}, AppConfig.Upload.MaxConcurrentTasks)

    app := fiber.New(fiber.Config{
        BodyLimit: int(MaxUploadSizeBytes),
    })

    app.Get("/ws", websocket.New(WebSocketHandler))

    // 优化后的上传接口
    app.Post("/upload", func(c *fiber.Ctx) error {
        fileHeader, err := c.FormFile("picture")
        if err != nil {
            return c.Status(fiber.StatusBadRequest).SendString("Missing file")
        }
        if fileHeader.Size > MaxUploadSizeBytes {
            return c.Status(fiber.StatusRequestEntityTooLarge).SendString("File too large")
        }

        format := normalizeFormat(c.FormValue("format", "webp"))
        if !isSupportedFormat(format) {
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
        defer file.Close()

        inputBuffer, err := io.ReadAll(file)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file")
        }

        hash := md5.Sum(inputBuffer)
        md5Str := hex.EncodeToString(hash[:])
        key := ProcessKey{MD5: md5Str, Format: format, Quality: quality}

        queueMutex.Lock()
        if processed[key] {
            queueMutex.Unlock()
            return c.Status(fiber.StatusOK).JSON(fiber.Map{
                "message": "Image already processed for this format/quality",
                "md5":     md5Str,
                "format":  format,
                "quality": quality,
            })
        }
        queue[key] = fileHeader.Filename
        statusMap[key] = "queued"
        queueMutex.Unlock()

        c.Set("Content-Type", "application/json")
        _ = c.Status(fiber.StatusAccepted).JSON(fiber.Map{
            "message": "Your image is queued for processing",
            "md5":     md5Str,
            "format":  format,
            "quality": quality,
        })

        go processTask(key, inputBuffer)

        return nil
    })

    app.Get("/queue/:md5", func(c *fiber.Ctx) error {
        md5Str := c.Params("md5")
        queueMutex.Lock()
        // 查找所有相关key
        var items []fiber.Map
        for k, filename := range queue {
            if k.MD5 == md5Str {
                items = append(items, fiber.Map{
                    "md5":     k.MD5,
                    "filename": filename,
                    "format":  k.Format,
                    "quality": k.Quality,
                })
            }
        }
        queueMutex.Unlock()
        if len(items) == 0 {
            return c.Status(fiber.StatusNotFound).SendString("MD5 not found in queue")
        }
        return c.JSON(items)
    })

    app.Get("/status/:md5", func(c *fiber.Ctx) error {
        md5Str := c.Params("md5")
        queueMutex.Lock()
        var items []fiber.Map
        for k, stat := range statusMap {
            if k.MD5 == md5Str {
                items = append(items, fiber.Map{
                    "md5":     k.MD5,
                    "status":  stat,
                    "format":  k.Format,
                    "quality": k.Quality,
                })
            }
        }
        queueMutex.Unlock()
        if len(items) == 0 {
            return c.Status(fiber.StatusNotFound).SendString("MD5 not found")
        }
        return c.JSON(items)
    })

    app.Get("/download/:md5", func(c *fiber.Ctx) error {
        md5Str := c.Params("md5")
        formats := []string{"webp", "png", "jpg"}
        var outputPath string

        for _, f := range formats {
            // jpg可能有多个quality
            if f == "jpg" {
                for q := 1; q <= 100; q++ {
                    path := fmt.Sprintf("output/%s.jpg_%d", md5Str, q)
                    if _, err := os.Stat(path); err == nil {
                        outputPath = path
                        break
                    }
                }
            } else {
                path := fmt.Sprintf("output/%s.%s", md5Str, f)
                if _, err := os.Stat(path); err == nil {
                    outputPath = path
                    break
                }
            }
            if outputPath != "" {
                break
            }
        }

        if outputPath == "" {
            return c.Status(fiber.StatusNotFound).SendString("File not found")
        }

        deleteLock.Lock()
        toDelete = append(toDelete, outputPath)
        count++
        deleteLock.Unlock()

        return c.SendFile(outputPath)
    })

    go func() {
        for {
            time.Sleep(10 * time.Second)
            deleteLock.Lock()
            if count >= 50 {
                for _, path := range toDelete {
                    _ = os.Remove(path)
                }
                toDelete = []string{}
                count = 0
                log.Println("Deleted 50 processed files")
            }
            deleteLock.Unlock()
        }
    }()

    addr := fmt.Sprintf(":%d", AppConfig.Server.Port)
    log.Fatal(app.Listen(addr))
}

// ===========================
// WebSocket 优化
// ===========================
func WebSocketHandler(c *websocket.Conn) {
    defer c.Close()
    writeLock := &sync.Mutex{}

    for {
        _, msg, err := c.ReadMessage()
        if err != nil {
            return
        }
        var req WSUploadRequest
        if err := json.Unmarshal(msg, &req); err != nil {
            wsWriteJSON(writeLock, c, WSProgressMessage{Status: "error", Message: "invalid_json"})
            continue
        }

        format := normalizeFormat(req.Format)
        if !isSupportedFormat(format) {
            wsWriteJSON(writeLock, c, WSProgressMessage{Status: "unsupported_format", Message: "only webp/png/jpg supported"})
            continue
        }

        data, err := base64.StdEncoding.DecodeString(req.Data)
        if err != nil {
            wsWriteJSON(writeLock, c, WSProgressMessage{Status: "decode_error", Message: "invalid base64"})
            continue
        }
        if int64(len(data)) > MaxUploadSizeBytes {
            wsWriteJSON(writeLock, c, WSProgressMessage{Status: "size_exceeded", Message: "file too large"})
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
        key := ProcessKey{MD5: md5Str, Format: format, Quality: quality}

        queueMutex.Lock()
        already := processed[key]
        queueMutex.Unlock()
        if already {
            path := outputPathForKey(key)
            if buf, err := os.ReadFile(path); err == nil {
                wsWriteJSON(writeLock, c, WSProgressMessage{
                    Status: "done", MD5: md5Str, File: base64.StdEncoding.EncodeToString(buf), Format: format, Quality: quality,
                })
                continue
            }
        }

        queueMutex.Lock()
        queue[key] = req.Filename
        statusMap[key] = "queued"
        queueMutex.Unlock()

        wsWriteJSON(writeLock, c, WSProgressMessage{Status: "queued", MD5: md5Str, Format: format, Quality: quality})

        go func(key ProcessKey, buffer []byte) {
            taskSemaphore <- struct{}{}
            defer func() { <-taskSemaphore }()

            queueMutex.Lock()
            statusMap[key] = "processing"
            queueMutex.Unlock()

            wsWriteJSON(writeLock, c, WSProgressMessage{Status: "processing", MD5: key.MD5, Format: key.Format, Quality: key.Quality})

            if err := ensureOutputDir(); err != nil {
                markFailed(key)
                wsWriteJSON(writeLock, c, WSProgressMessage{Status: "error", MD5: key.MD5, Message: "mkdir_failed"})
                return
            }

            var outputBuffer []byte
            var err error

            switch key.Format {
            case "png":
                outputBuffer, err = processPNGWithPngquant(buffer)
            case "jpg":
                outputBuffer, err = processWithBimgQuality(buffer, bimg.JPEG, key.Quality)
            case "webp":
                outputBuffer, err = processWebPWithBimg(buffer)
            default:
                outputBuffer, err = processWebPWithBimg(buffer)
            }

            if err != nil {
                log.Printf("Image processing failed for %s: %v", key.MD5, err)
                markFailed(key)
                wsWriteJSON(writeLock, c, WSProgressMessage{Status: "error", MD5: key.MD5, Message: "processing_failed"})
                return
            }

            outPath := outputPathForKey(key)
            if err := os.WriteFile(outPath, outputBuffer, 0644); err != nil {
                log.Printf("Failed to save image %s: %v", key.MD5, err)
                markFailed(key)
                wsWriteJSON(writeLock, c, WSProgressMessage{Status: "error", MD5: key.MD5, Message: "save_failed"})
                return
            }

            markDone(key)

            wsWriteJSON(writeLock, c, WSProgressMessage{
                Status: "done",
                MD5:    key.MD5,
                File:   base64.StdEncoding.EncodeToString(outputBuffer),
                Format: key.Format,
                Quality: key.Quality,
            })
        }(key, data)
    }
}

func wsWriteJSON(lock *sync.Mutex, c *websocket.Conn, v any) {
    lock.Lock()
    defer lock.Unlock()
    _ = c.WriteJSON(v)
}

// ===========================
// 处理逻辑
// ===========================

func processWebPWithBimg(buf []byte) ([]byte, error) {
    options := bimg.Options{
        Quality:       80,
        Type:          bimg.WEBP,
        Compression:   6,
        StripMetadata: true,
    }
    return bimg.NewImage(buf).Process(options)
}

func processWithBimgQuality(buf []byte, t bimg.ImageType, quality int) ([]byte, error) {
    options := bimg.Options{
        Quality:       quality,
        Type:          t,
        StripMetadata: true,
    }
    return bimg.NewImage(buf).Process(options)
}

func processPNGWithPngquant(input []byte) ([]byte, error) {
    img, _, err := image.Decode(bytes.NewReader(input))
    if err != nil {
        return nil, fmt.Errorf("decode input: %w", err)
    }
    var pngBuf bytes.Buffer
    if err := png.Encode(&pngBuf, img); err != nil {
        return nil, fmt.Errorf("encode to png: %w", err)
    }
    var out bytes.Buffer
    var stderr bytes.Buffer
    cmd := exec.Command("pngquant",
        "--quality=65-85", "--speed", "1", "--strip", "--output", "-", "-",
    )
    cmd.Stdin = &pngBuf
    cmd.Stdout = &out
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("pngquant failed: %v, stderr: %s", err, stderr.String())
    }
    return out.Bytes(), nil
}

// ===========================
// 辅助函数
// ===========================
func ensureOutputDir() error {
    return os.MkdirAll("output", os.ModePerm)
}

func markFailed(key ProcessKey) {
    queueMutex.Lock()
    defer queueMutex.Unlock()
    statusMap[key] = "failed"
}

func markDone(key ProcessKey) {
    queueMutex.Lock()
    defer queueMutex.Unlock()
    delete(queue, key)
    statusMap[key] = "done"
    processed[key] = true
}

func isSupportedFormat(f string) bool {
    return f == "webp" || f == "png" || f == "jpg"
}

func normalizeFormat(f string) string {
    ff := strings.ToLower(strings.TrimSpace(f))
    if ff == "" {
        return "webp"
    }
    if ff == "jpeg" {
        return "jpg"
    }
    return ff
}

// 输出文件名优化
func outputPathForKey(key ProcessKey) string {
    if key.Format == "jpg" {
        return fmt.Sprintf("output/%s.jpg_%d", key.MD5, key.Quality)
    }
    return fmt.Sprintf("output/%s.%s", key.MD5, key.Format)
}

// 任务处理
func processTask(key ProcessKey, buffer []byte) {
    taskSemaphore <- struct{}{}
    defer func() { <-taskSemaphore }()

    queueMutex.Lock()
    statusMap[key] = "processing"
    queueMutex.Unlock()

    if err := ensureOutputDir(); err != nil {
        markFailed(key)
        return
    }

    var outputBuffer []byte
    var err error
    switch key.Format {
    case "png":
        outputBuffer, err = processPNGWithPngquant(buffer)
    case "jpg":
        outputBuffer, err = processWithBimgQuality(buffer, bimg.JPEG, key.Quality)
    case "webp":
        outputBuffer, err = processWebPWithBimg(buffer)
    default:
        outputBuffer, err = processWebPWithBimg(buffer)
    }

    if err != nil {
        log.Printf("Image processing failed for %s: %v", key.MD5, err)
        markFailed(key)
        return
    }

    outPath := outputPathForKey(key)
    if err := os.WriteFile(outPath, outputBuffer, 0644); err != nil {
        log.Printf("Failed to save image %s: %v", key.MD5, err)
        markFailed(key)
        return
    }
    markDone(key)
    log.Printf("Image %s processed and saved to %s", key.MD5, outPath)
}

// ===========================
// 配置与工具
// ===========================
func loadConfig() {
    viper.SetConfigFile("config.yaml")
    viper.SetConfigType("yaml")
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("upload.max_upload_size", "10MB")
    viper.SetDefault("upload.max_concurrent_tasks", 3)

    if err := viper.ReadInConfig(); err != nil {
        log.Printf("Config read warning: %v (using defaults if missing)", err)
    }
    if err := viper.Unmarshal(&AppConfig); err != nil {
        log.Fatalf("Unable to decode config: %v", err)
    }

    MaxUploadSizeBytes = parseSize(AppConfig.Upload.MaxUploadSize)
    if MaxUploadSizeBytes <= 0 {
        MaxUploadSizeBytes = parseSize("10MB")
    }
}

func parseSize(s string) int64 {
    str := strings.TrimSpace(strings.ToUpper(s))
    mult := int64(1)
    switch {
    case strings.HasSuffix(str, "GB"):
        mult = 1024 * 1024 * 1024
        str = strings.TrimSuffix(str, "GB")
    case strings.HasSuffix(str, "G"):
        mult = 1024 * 1024 * 1024
        str = strings.TrimSuffix(str, "G")
    case strings.HasSuffix(str, "MB"):
        mult = 1024 * 1024
        str = strings.TrimSuffix(str, "MB")
    case strings.HasSuffix(str, "M"):
        mult = 1024 * 1024
        str = strings.TrimSuffix(str, "M")
    case strings.HasSuffix(str, "KB"):
        mult = 1024
        str = strings.TrimSuffix(str, "KB")
    case strings.HasSuffix(str, "K"):
        mult = 1024
        str = strings.TrimSuffix(str, "K")
    case strings.HasSuffix(str, "B"):
        mult = 1
        str = strings.TrimSuffix(str, "B")
    }
    str = strings.TrimSpace(str)
    var val int64
    _, err := fmt.Sscanf(str, "%d", &val)
    if err != nil {
        return 0
    }
    return val * mult
}
