package middleware

import (
	"log"
	"os"
	"time"

	"image-processor/config"
)

func StartCleanupRoutine() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			maxAge := time.Duration(config.MaxAgeHour) * time.Hour

			outputDir := config.AppConfig.Server.OutputDir
			if outputDir == "" {
				outputDir = "output"
			}

			files, err := os.ReadDir(outputDir)
			if err != nil {
				log.Printf("Failed to read output dir for cleanup: %v", err)
				continue
			}

			now := time.Now()
			deletedCount := 0

			for _, f := range files {
				if f.IsDir() {
					continue
				}

				info, err := f.Info()
				if err != nil {
					continue
				}

				if now.Sub(info.ModTime()) > maxAge {
					path := outputDir + "/" + f.Name()
					if err := os.Remove(path); err == nil {
						deletedCount++
					} else {
						log.Printf("Failed to remove file %s: %v", path, err)
					}
				}
			}

			if deletedCount > 0 {
				log.Printf("Cron Cleanup: Deleted %d expired files from %s directory.", deletedCount, outputDir)
			}
		}
	}()
}