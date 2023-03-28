package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"pirecorder/apperror"
	"pirecorder/config"
	"pirecorder/logger"
	"pirecorder/models"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/icza/mjpeg"
	"golang.org/x/sys/unix"
)

type App struct {
	isRecording       bool
	isUploading       bool
	isCamUp           bool
	recordName        string
	uploadName        string
	camStreamFrame    []byte
	recordStreamFrame []byte
	mux               *Mux
	logger            *logger.Logger
	uploader          *s3manager.Uploader
}

func NewApp(logger *logger.Logger) (*App, error) {
	s3config := config.GetConfig().S3Config

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(s3config.Region),
		Credentials:      credentials.NewStaticCredentials(s3config.AccessKey, s3config.SecretKey, ""),
		Endpoint:         aws.String("http://localhost:4566"),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		return nil, err
	}

	return &App{
		logger:   logger,
		uploader: s3manager.NewUploader(sess),
	}, nil
}

func (a *App) Start() {
	a.logger.LogInfo("Starting the app")
	videosFolder := config.GetConfig().VideosFolder
	_, err := os.Stat(videosFolder)

	if err != nil {
		err = os.MkdirAll(videosFolder, 0755)
		if err != nil {
			a.logger.LogError(err, "Error creating videos folder", "videosFolder", videosFolder)
			return
		}
	}

	a.UploadLogs()
	stream, err := a.StartCamera()

	if err != nil {
		a.logger.LogError(err, "Error starting camera")
		return
	}
	a.mux = NewMux(stream)
}

func (a *App) UploadLogs() {
	s3config := config.GetConfig().S3Config
	logFolder := config.GetConfig().LogFolder

	a.logger.LogInfo("Uploading logs to S3", "folder_name", logFolder)

	file, err := os.Open(logFolder)
	if err != nil {
		a.logger.LogError(err, "Error opening log folder", "folder_name", logFolder)
		return
	}

	filenames, err := file.Readdirnames(0)
	if err != nil {
		a.logger.LogError(err, "Error reading log folder", "folder_", logFolder)
		return
	}

	deviceHostName, err := os.Hostname()
	if err != nil {
		a.logger.LogError(err, "Error getting device hostname", "function", "UploadLogs")
		return
	}

	sort.Strings(filenames)
	filenames = filenames[:len(filenames)-1]

	a.logger.LogInfo("Uploading logs to S3")
	for _, filename := range filenames {
		localFilename := fmt.Sprintf("%s/%s", logFolder, filename)
		f, err := os.ReadFile(localFilename)
		if err != nil {
			a.logger.LogError(err, "Error reading log file", "filename", filename)
			continue
		}

		_, err = a.uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(s3config.Bucket),
			Key:         aws.String(fmt.Sprintf("%s/logs/%s", deviceHostName, filename)),
			Body:        aws.ReadSeekCloser(bytes.NewReader(f)),
			ContentType: aws.String("text/plain"),
		})

		if err != nil {
			a.logger.LogError(err, "Error uploading log file", "filename", filename)
			continue
		}

		if err = os.Remove(localFilename); err != nil {
			a.logger.LogError(err, "Error removing log file", "filename", filename)
		}
	}
}

func (a *App) StartCamera() (io.Reader, error) {
	a.logger.LogInfo("Starting the camera")
	cmdName := "ffmpeg"
	cmdArgs := []string{
		"-f", "v4l2",
		"-framerate", "30",
		"-video_size", "640x480",
		"-i", "/dev/video0",
		"-f", "mpjpeg",
		"-",
	}
	cmd := exec.Command(cmdName, cmdArgs...)
	pr, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	a.isCamUp = true

	return pr, nil
}

func (a *App) StartStream() (chan []byte, chan struct{}) {
	a.logger.LogInfo("Starting the stream")
	streamChan := make(chan []byte, 65535)
	closeChan := make(chan struct{})
	var prevFrame []byte
	ticker := time.Tick(33 * time.Millisecond) // 30 fps

	go func() {
		for {
			select {
			case <-closeChan:
				a.logger.LogInfo("Closing the stream")
				return
			case <-ticker:
				frame := a.mux.GetFrame()

				if bytes.Equal(frame, prevFrame) {
					continue
				}
				streamChan <- frame
				prevFrame = frame
			}
		}
	}()
	return streamChan, closeChan
}

func (a *App) StopStream() {
	a.logger.LogInfo("Stopping the stream")
}

func (a *App) StartRecording(filename string) error {
	if a.isRecording {
		a.isRecording = false
	}

	aw, err := mjpeg.New(fmt.Sprintf("%s/%s.avi", config.GetConfig().VideosFolder, filename), 640, 480, 30)

	if err != nil {
		a.logger.LogError(err, "Error creating video file", "file_name", filename)
		err := apperror.ServerError
		return err
	}

	a.isRecording = true
	a.recordName = fmt.Sprintf("%s.avi", filename)

	go func() {
		defer func() {
			_ = aw.Close()
			a.isRecording = false
			a.recordName = ""
		}()
		var prevFrame []byte
		ticker := time.Tick(33 * time.Millisecond) // 30 fps
		for a.isRecording {
			<-ticker
			frame := a.mux.GetFrame()
			if bytes.Equal(frame, prevFrame) {
				continue
			}
			err = aw.AddFrame(frame)
			if err != nil {
				a.logger.LogError(err, "Error writing video to file")
			}
			prevFrame = frame
		}
	}()
	return nil
}

func (a *App) StopRecording() {
	a.logger.LogInfo("Stopping camera recording", "file_name", a.recordName)
	if !a.isRecording {
		return
	}
	a.isRecording = false
}

func (a *App) UploadRecording(filename string) error {
	if a.isRecording {
		a.logger.LogError(errors.New("recording in progress"), "Cannot upload recording while recording is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while recording is in progress")
		return err
	}

	if a.isUploading {
		a.logger.LogError(errors.New("upload in progress"), "Cannot upload recording while another upload is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while another upload is in progress")
		return err
	}

	a.isUploading = true
	a.uploadName = fmt.Sprintf("%s.avi", filename)
	defer func() {
		a.isUploading = false
		a.uploadName = ""
	}()

	videosFolder := config.GetConfig().VideosFolder
	_, err := os.Stat(fmt.Sprintf("%s/%s", videosFolder, filename))

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			a.logger.LogError(err, "Provided file does not exist in specified folder", "folder_name", videosFolder, "file_name", filename)
			return apperror.NotFound
		}
		a.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	deviceHostName, err := os.Hostname()

	if err != nil {
		a.logger.LogError(err, "Error fetching device hostname", "action", "upload", "file_name", filename)
		return apperror.ServerError
	}

	s3Config := config.GetConfig().S3Config
	f := fmt.Sprintf("%s/%s", videosFolder, filename)
	fd, err := os.ReadFile(f)

	if err != nil {
		a.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	_, err = a.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(s3Config.Bucket),
		Key:         aws.String(fmt.Sprintf("%s/videos/%s", deviceHostName, filename)),
		ACL:         aws.String("private"),
		Body:        bytes.NewReader(fd),
		ContentType: aws.String("video/x-msvideo"),
	})

	if err != nil {
		a.logger.LogError(err, "Error uploading file to S3", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	a.logger.LogInfo("Successful upload to S3", "folder_name", videosFolder, "file_name", filename)

	if err = os.Remove(f); err != nil {
		a.logger.LogError(err, "Error deleting file", "folder_name", videosFolder, "file_name", filename)
		return apperror.ServerError
	}

	a.logger.LogInfo("Successful deletion of file", "folder_name", videosFolder, "file_name", filename)

	return nil
}

func (a *App) UploadRecordings() error {
	if a.isRecording {
		a.logger.LogError(errors.New("recording in progress"), "Cannot upload recording while recording is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while recording is in progress")
		return err
	}

	if a.isUploading {
		a.logger.LogError(errors.New("upload in progress"), "Cannot upload recording while another upload is in progress")
		err := apperror.ServiceUnavailable
		err = err.SetMessage("Cannot upload recording while another upload is in progress")
		return err
	}

	a.isUploading = true
	defer func() {
		a.isUploading = false
		a.uploadName = ""
	}()
	videosFolder := config.GetConfig().VideosFolder
	a.logger.LogInfo("Uploading All Videos to S3", "folder_name", videosFolder)
	fd, err := os.Open(videosFolder)

	if err != nil {
		a.logger.LogError(err, "Error opening videos folder", "function", "UploadAllRecordings", "folder_name", videosFolder)
		return apperror.ServerError
	}

	files, err := fd.Readdirnames(0)

	if err != nil {
		a.logger.LogError(err, "Error reading videos folder", "function", "UploadAllRecordings", "folder_name", videosFolder)
		return apperror.ServerError
	}

	deviceHostName, err := os.Hostname()

	if err != nil {
		a.logger.LogError(err, "Error fetching device hostname", "function", "UploadAllRecording")
		return apperror.ServerError
	}

	s3Config := config.GetConfig().S3Config

	for _, file := range files {
		if file[len(file)-4:] != ".avi" {
			continue
		}
		a.uploadName = fmt.Sprintf("%s.avi", file)
		a.logger.LogInfo("Uploading file to S3", "file_name", file)
		f := fmt.Sprintf("%s/%s", videosFolder, file)
		contents, err := os.ReadFile(f)

		if err != nil {
			a.logger.LogError(err, "Error reading file", "folder_name", videosFolder, "file_name", file)
			return apperror.ServerError
		}

		_, err = a.uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(s3Config.Bucket),
			Key:         aws.String(fmt.Sprintf("%s/videos/%s", deviceHostName, file)),
			ACL:         aws.String("private"),
			Body:        bytes.NewReader(contents),
			ContentType: aws.String("video/x-msvideo"),
		})

		if err != nil {
			a.logger.LogError(err, "Error uploading file to S3", "folder_name", videosFolder, "file_name", file)
			continue
		}

		a.logger.LogInfo("Successful upload to S3", "folder_name", videosFolder, "file_name", file)

		if err = os.Remove(f); err != nil {
			a.logger.LogError(err, "Error deleting file", "folder_name", videosFolder, "file_name", file)
		}

		a.logger.LogInfo("Successful deletion of file", "folder_name", videosFolder, "file_name", file)
	}

	return nil
}

func (a *App) FetchRecordings() ([]models.FileDetails, error) {
	videosFolder := config.GetConfig().VideosFolder
	a.logger.LogInfo("Fetching available recordings", "folder_name", videosFolder)

	fd, err := os.Open(videosFolder)

	if err != nil {
		a.logger.LogError(err, "Error opening videos folder", "folder_name", videosFolder)
		return nil, apperror.ServerError
	}

	defer func() { _ = fd.Close() }()

	files, err := fd.Readdirnames(0)

	if err != nil {
		a.logger.LogError(err, "Error reading videos folder", "folder_name", videosFolder)
		return nil, apperror.ServerError
	}

	var fileDetails []models.FileDetails

	for _, file := range files {
		if file[len(file)-4:] != ".avi" {
			continue
		}
		fileDetail := models.FileDetails{
			Filename: file,
		}

		if a.isRecording && file == a.recordName {
			fileDetail.Recording = true
		} else if a.isUploading && file == a.uploadName {
			fileDetail.Uploading = true
		}

		fileDetails = append(fileDetails, fileDetail)
	}

	return fileDetails, nil
}

func (a *App) AppStatus() *models.Status {
	var stat unix.Statfs_t

	if err := unix.Statfs("/home", &stat); err != nil {
		a.logger.LogError(err, "Error getting disk usage")
		return nil
	}

	availableBlocks := float32(stat.Bavail) * float32(stat.Bsize)
	totalBlocks := float32(stat.Blocks) * float32(stat.Bsize)
	availPercentage := (100 - ((availableBlocks / totalBlocks) * 100)) / 100

	availPercentage = float32(truncate(float64(availPercentage), 0.01))

	return &models.Status{
		CameraUp:  a.isCamUp,
		Recording: a.isRecording,
		Uploading: a.isUploading,
		DiskUsage: availPercentage,
	}
}

func truncate(num float64, unit float64) float64 {
	bf := big.NewFloat(0).SetPrec(1000).SetFloat64(num)
	bu := big.NewFloat(0).SetPrec(1000).SetFloat64(unit)

	bf.Quo(bf, bu)

	i := big.NewInt(0)
	bf.Int(i)
	bf.SetInt(i)

	f, _ := bf.Mul(bf, bu).Float64()

	return f
}
