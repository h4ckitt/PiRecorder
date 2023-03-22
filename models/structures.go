package models

type Status struct {
	CameraUp  bool    `json:"isCamUp"`
	Recording bool    `json:"isRecording"`
	Uploading bool    `json:"isUploading"`
	DiskUsage float32 `json:"diskUsage"`
}

type FileDetails struct {
	Filename  string `json:"filename"`
	Uploading bool   `json:"isUploading"`
	Recording bool   `json:"isRecording"`
}
