package config

type Config struct {
	Server struct {
		Port      int    `mapstructure:"port"`
		OutputDir string `mapstructure:"output_dir"`
	} `mapstructure:"server"`
	Upload struct {
		MaxUploadSize      string `mapstructure:"max_upload_size"`
		MaxConcurrentTasks int    `mapstructure:"max_concurrent_tasks"`
		MaxAge             int    `mapstructure:"max_age"`
	} `mapstructure:"upload"`
	Download struct {
		DownloadUrl string `mapstructure:"download_url"`
	} `mapstructure:"download"`
}

var (
	AppConfig          Config
	MaxUploadSizeBytes int64
	MaxAgeHour         int
)