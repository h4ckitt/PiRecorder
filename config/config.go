package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var Conf Config

func Load() {
	var err error

	_, err = os.Stat(".env")

	if err != nil {
		log.Println(".env file does not exist\nReading from the environment directly")
	} else {
		err = godotenv.Load(".env")

		if err != nil {
			log.Fatal(err)
		}
	}

	Conf = Config{
		LogFolder:    os.Getenv("LOG_FOLDER"),
		VideosFolder: os.Getenv("VIDEOS_FOLDER"),
		S3Config: S3{
			Bucket:    os.Getenv("S3_BUCKET_NAME"),
			AccessKey: os.Getenv("S3_ACCESS_KEY"),
			SecretKey: os.Getenv("S3_SECRET_KEY"),
			Region:    os.Getenv("S3_REGION"),
		},
		Port: func() string {
			port := os.Getenv("PORT")
			if port == "" {
				return "8080"
			}
			return port
		}(),
	}
}

func GetConfig() Config {
	return Conf
}
