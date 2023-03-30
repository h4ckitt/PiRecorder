package config

type Config struct {
	Environment  string
	LogFolder    string
	VideosFolder string
	AudiosFolder string
	Port         string
	S3Config     S3
	SSLConfig    SSL
}

type S3 struct {
	AccessKey   string
	SecretKey   string
	Region      string
	Bucket      string
	EndpointUrl string
}

type SSL struct {
	CertFile string
	KeyFile  string
}
