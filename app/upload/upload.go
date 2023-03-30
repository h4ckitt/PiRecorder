package upload

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"pirecorder/apperror"
	"pirecorder/config"
	"pirecorder/logger"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Uploader struct {
	isUploading      bool
	videoIsRecording bool
	uploadName       string
	logger           *logger.Logger
	uploader         *s3manager.Uploader
}

func NewUploader(logger *logger.Logger) (*Uploader, error) {
	s3config := config.GetConfig().S3Config

	awsConfig := &aws.Config{
		Region:           aws.String(s3config.Region),
		Credentials:      credentials.NewStaticCredentials(s3config.AccessKey, s3config.SecretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	}

	if s3config.EndpointUrl != "" {
		awsConfig.Endpoint = aws.String(s3config.EndpointUrl)
	}

	sess, err := session.NewSession(awsConfig)

	if err != nil {
		return nil, err
	}

	return &Uploader{
		logger:   logger,
		uploader: s3manager.NewUploader(sess),
	}, nil
}

func (u *Uploader) UploadStats() (bool, string) {
	return u.isUploading, u.uploadName
}

func (u *Uploader) UploadLogs() {
	s3config := config.GetConfig().S3Config
	logFolder := config.GetConfig().LogFolder

	u.logger.LogInfo("Uploading logs to S3", "bucket", s3config.Bucket, "folder", logFolder)

	dir, err := os.Open(logFolder)

	if err != nil {
		u.logger.LogError(err, "Error opening log folder", "folder", logFolder)
		return
	}

	defer func() { _ = dir.Close() }()

	filenames, err := dir.Readdirnames(0)

	if err != nil {
		u.logger.LogError(err, "Error reading log folder", "folder", logFolder)
		return
	}

	deviceHostName, err := os.Hostname()

	if err != nil {
		u.logger.LogError(err, "Error getting device hostname", "function", "UploadLogs")
		return
	}

	sort.Strings(filenames)

	filenames = filenames[:len(filenames)-1] // remove last file, which is the current log file

	for _, filename := range filenames {
		localFilename := fmt.Sprintf("%s/%s", logFolder, filename)
		f, err := os.ReadFile(localFilename)

		if err != nil {
			u.logger.LogError(err, "Error reading log file", "filename", localFilename)
			continue
		}

		_, err = u.uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(s3config.Bucket),
			Key:         aws.String(fmt.Sprintf("%s/logs/%s", deviceHostName, filename)),
			Body:        aws.ReadSeekCloser(bytes.NewReader(f)),
			ContentType: aws.String("text/plain"),
		})

		if err != nil {
			u.logger.LogError(err, "Error uploading log file", "filename", filename)
			continue
		}

		if err := os.Remove(localFilename); err != nil {
			u.logger.LogError(err, "Error removing log file", "filename", filename)
		}
	}
}

func (u *Uploader) UploadRecording(filename string) error {
	if u.videoIsRecording {
		u.logger.LogError(errors.New("recording in progress"), "Cannot upload recording while recording is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while recording is in progress")
		return err
	}

	if u.isUploading {
		u.logger.LogError(errors.New("upload in progress"), "Cannot upload recording while another upload is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while another upload is in progress")
		return err
	}

	u.isUploading = true
	u.uploadName = fmt.Sprintf("%s.avi", filename)
	defer func() {
		u.isUploading = false
		u.uploadName = ""
	}()

	videosFolder := config.GetConfig().VideosFolder
	_, err := os.Stat(fmt.Sprintf("%s/%s", videosFolder, filename))

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			u.logger.LogError(err, "Provided file does not exist in specified folder", "folder_name", videosFolder, "file_name", filename)
			return apperror.NotFound
		}
		u.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	deviceHostName, err := os.Hostname()

	if err != nil {
		u.logger.LogError(err, "Error fetching device hostname", "action", "upload", "file_name", filename)
		return apperror.ServerError
	}

	s3Config := config.GetConfig().S3Config
	f := fmt.Sprintf("%s/%s", videosFolder, filename)
	fd, err := os.ReadFile(f)

	if err != nil {
		u.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	_, err = u.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(s3Config.Bucket),
		Key:         aws.String(fmt.Sprintf("%s/videos/%s", deviceHostName, filename)),
		ACL:         aws.String("private"),
		Body:        bytes.NewReader(fd),
		ContentType: aws.String("video/x-msvideo"),
	})

	if err != nil {
		u.logger.LogError(err, "Error uploading file to S3", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	u.logger.LogInfo("Successful upload to S3", "folder_name", videosFolder, "file_name", filename)

	if err = performCallBack(filename); err != nil {
		u.logger.LogError(err, "Error performing callback", "folder_name", videosFolder, "file_name", filename)
	}

	if err = os.Remove(f); err != nil {
		u.logger.LogError(err, "Error deleting file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	u.logger.LogInfo("Successful deletion of file", "folder_name", videosFolder, "file_name", filename)

	return nil
}

func (u *Uploader) UploadRecordings() error {
	if u.videoIsRecording {
		u.logger.LogError(errors.New("recording in progress"), "Cannot upload recording while recording is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while recording is in progress")
		return err
	}

	if u.isUploading {
		u.logger.LogError(errors.New("upload in progress"), "Cannot upload recording while another upload is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while another upload is in progress")
		return err
	}

	u.isUploading = true
	defer func() {
		u.isUploading = false
		u.uploadName = ""
	}()
	var files []string
	videosFolder := config.GetConfig().VideosFolder
	audiosFolder := config.GetConfig().AudiosFolder
	u.logger.LogInfo("Uploading All Videos And Audios to S3", "video_folderName", videosFolder, "audio_folderName", audiosFolder)
	fd, err := os.Open(videosFolder)

	if err != nil {
		u.logger.LogError(err, "Error opening videos folder", "function", "UploadAllRecordings", "folder_name", videosFolder)
		return apperror.ServerError
	}

	videoFiles, err := fd.Readdirnames(0)

	if err != nil {
		u.logger.LogError(err, "Error reading videos folder", "function", "UploadAllRecordings", "folder_name", videosFolder)
		return apperror.ServerError
	}

	files = append(files, videoFiles...)

	fd, err = os.Open(audiosFolder)

	if err != nil {
		u.logger.LogError(err, "Error opening audios folder", "function", "UploadAllRecordings", "folder_name", audiosFolder)
		return apperror.ServerError
	}

	audioFiles, err := fd.Readdirnames(0)

	if err != nil {
		u.logger.LogError(err, "Error reading audios folder", "function", "UploadAllRecordings", "folder_name", audiosFolder)
		return apperror.ServerError
	}

	files = append(files, audioFiles...)

	_ = fd.Close()

	deviceHostName, err := os.Hostname()

	if err != nil {
		u.logger.LogError(err, "Error fetching device hostname", "function", "UploadAllRecording")
		return apperror.ServerError
	}

	s3Config := config.GetConfig().S3Config
	var (
		ext            string
		f              string
		contentType    string
		remoteFileName string
	)

	for _, file := range files {
		ext = file[len(file)-4:]
		if ext != ".avi" && ext != ".wav" {
			continue
		}

		u.uploadName = fmt.Sprintf("%s.%s", file, ext)
		u.logger.LogInfo("Uploading file to S3", "file_name", file)

		switch ext {
		case ".avi":
			f = fmt.Sprintf("%s/%s", videosFolder, file)
			contentType = "video/x-msvideo"
			remoteFileName = fmt.Sprintf("%s/videos/%s", deviceHostName, file)
		case ".wav":
			f = fmt.Sprintf("%s/%s", audiosFolder, file)
			contentType = "audio/x-wav"
			remoteFileName = fmt.Sprintf("%s/audios/%s", deviceHostName, file)
		}

		contents, err := os.ReadFile(f)

		if err != nil {
			u.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", file)
			return apperror.ServerError
		}

		_, err = u.uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(s3Config.Bucket),
			Key:         aws.String(remoteFileName),
			ACL:         aws.String("private"),
			Body:        bytes.NewReader(contents),
			ContentType: aws.String(contentType),
		})

		if err != nil {
			u.logger.LogError(err, "Error uploading file to S3", "folder_name", videosFolder, "file_name", file)
			continue
		}

		u.logger.LogInfo("Successful upload to S3", "folder_name", videosFolder, "file_name", file)

		if err = performCallBack(file); err != nil {
			u.logger.LogError(err, "Error performing callback", "folder_name", videosFolder, "file_name", file)
		}

		if err = os.Remove(f); err != nil {
			u.logger.LogError(err, "Error deleting file", "folder_name", videosFolder, "file_name", file)
		}

		u.logger.LogInfo("Successful deletion of file", "folder_name", videosFolder, "file_name", file)
	}

	return nil
}

func performCallBack(filename string) error {
	resp, err := http.Get(fmt.Sprintf("https://videos-service.herokuapp.com/%s", filename))

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("received non 200 status code")
	}

	return nil
}
