package config

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func Load() {
	configFile := flag.String("config", "config.yaml", "Path to the config file")
	flag.Parse()

	viper.SetConfigFile(*configFile)
	viper.SetConfigType("yaml")
	
	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config read warning: %v (using defaults if missing)", err)
	}
	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	// 解析并验证配置
	parseAndValidateConfig()
}

func setDefaults() {
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.output_dir", "output")
	viper.SetDefault("upload.max_upload_size", "10MB")
	viper.SetDefault("upload.max_concurrent_tasks", 3)
	viper.SetDefault("download.download_url", "download/")
	viper.SetDefault("upload.max_age", 24)
}

func parseAndValidateConfig() {
	MaxUploadSizeBytes = ParseSize(AppConfig.Upload.MaxUploadSize)
	MaxAgeHour = AppConfig.Upload.MaxAge

	if MaxAgeHour <= 0 {
		log.Printf("Config Upload.MaxAge invalid, set to 24 Hours default, raw value: %v", MaxAgeHour)
		MaxAgeHour = 24
	}

	if MaxUploadSizeBytes <= 0 {
		MaxUploadSizeBytes = ParseSize("10MB")
	}
}

func ParseSize(s string) int64 {
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