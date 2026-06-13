package service

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/exec"

	"github.com/h2non/bimg"

	"image-processor/internal/model"
	"image-processor/internal/utils"
)

func ProcessWebPWithBimg(buf []byte) ([]byte, error) {
	options := bimg.Options{
		Quality:       80,
		Type:          bimg.WEBP,
		Compression:   6,
		StripMetadata: true,
	}
	return bimg.NewImage(buf).Process(options)
}

func ProcessWithBimgQuality(buf []byte, t bimg.ImageType, quality int) ([]byte, error) {
	options := bimg.Options{
		Quality:       quality,
		Type:          t,
		StripMetadata: true,
	}
	return bimg.NewImage(buf).Process(options)
}

func ProcessPNGWithPngquant(input []byte) ([]byte, error) {
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

func ProcessImage(key model.ProcessKey, buffer []byte) ([]byte, error) {
	var outputBuffer []byte
	var err error

	switch key.Format {
	case "png":
		outputBuffer, err = ProcessPNGWithPngquant(buffer)
	case "jpg":
		outputBuffer, err = ProcessWithBimgQuality(buffer, bimg.JPEG, key.Quality)
	case "webp":
		outputBuffer, err = ProcessWebPWithBimg(buffer)
	default:
		outputBuffer, err = ProcessWebPWithBimg(buffer)
	}

	return outputBuffer, err
}

func ProcessTask(key model.ProcessKey, buffer []byte) {
	taskSemaphore <- struct{}{}
	defer func() { <-taskSemaphore }()

	MarkProcessing(key)

	if err := utils.EnsureOutputDir(); err != nil {
		MarkFailed(key)
		return
	}

	outputBuffer, err := ProcessImage(key, buffer)
	if err != nil {
		log.Printf("Image processing failed for %s: %v", key.MD5, err)
		MarkFailed(key)
		return
	}

	outPath := utils.OutputPathForKey(key)
	if err := os.WriteFile(outPath, outputBuffer, 0644); err != nil {
		log.Printf("Failed to save image %s: %v", key.MD5, err)
		MarkFailed(key)
		return
	}

	MarkDone(key)
	log.Printf("Image %s processed and saved to %s", key.MD5, outPath)
}

var taskSemaphore chan struct{}

func InitSemaphore() {
	if taskSemaphore != nil {
		return
	}
	
	maxConcurrent := 3 // default value
	// You can use config here if needed
	taskSemaphore = make(chan struct{}, maxConcurrent)
}

func GetSemaphore() chan struct{} {
	return taskSemaphore
}