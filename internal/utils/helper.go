package utils

import (
	"fmt"
	"os"
	"strings"

	"image-processor/internal/model"
)

func IsSupportedFormat(f string) bool {
	return f == "webp" || f == "png" || f == "jpg"
}

func NormalizeFormat(f string) string {
	ff := strings.ToLower(strings.TrimSpace(f))
	if ff == "" {
		return "webp"
	}
	if ff == "jpeg" {
		return "jpg"
	}
	return ff
}

func OutputPathForKey(key model.ProcessKey) string {
	if key.Format == "jpg" {
		return fmt.Sprintf("output/%s.jpg_%d", key.MD5, key.Quality)
	}
	return fmt.Sprintf("output/%s.%s", key.MD5, key.Format)
}

func EnsureOutputDir() error {
	return os.MkdirAll("output", os.ModePerm)
}

func FindOutputFile(md5Str string) string {
	formats := []string{"webp", "png", "jpg"}
	var outputPath string

	for _, f := range formats {
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
	return outputPath
}