package config

type Config struct {
	LogFolder    string
	VideosFolder string
	S3Config     S3
}

type S3 struct {
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
}
