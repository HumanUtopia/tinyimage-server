package model

type ProcessKey struct {
	MD5     string
	Format  string
	Quality int
}

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