package app

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"pirecorder/app/audio"
	"pirecorder/app/helper"
	"pirecorder/app/upload"
	"pirecorder/app/video"
	"pirecorder/apperror"
	"pirecorder/config"
	"pirecorder/logger"
	"pirecorder/models"
)

type App struct {
	camera   *video.Camera
	mic      *audio.Mic
	uploader *upload.Uploader
	logger   *logger.Logger
}

func NewApp(logger *logger.Logger) (*App, error) {
	var (
		videoErr  bool
		uploadErr bool
	)
	logger.LogInfo("Initializing camera")
	cam, err := video.NewCamera(logger)

	if err != nil {
		logger.LogError(err, "Error initializing camera")
		videoErr = true
	}

	logger.LogInfo("Initializing microphone")
	mic, err := audio.NewMic(logger)

	if err != nil {
		logger.LogError(err, "Error initializing microphone")
	}

	logger.LogInfo("Initializing uploader")
	uploader, err := upload.NewUploader(logger)

	if err != nil {
		logger.LogError(err, "Error initializing uploader")
		uploadErr = true
	}

	if videoErr && uploadErr {
		err := errors.New("error initializing camera and uploader")
		logger.LogError(err, "Error initializing camera and uploader")
		return nil, err
	}

	uploader.UploadLogs()

	return &App{
		camera:   cam,
		mic:      mic,
		logger:   logger,
		uploader: uploader,
	}, nil
}

func (a *App) StartStream() (chan []byte, chan struct{}, error) {
	stream, closeChan, err := a.camera.StartStream()

	if err != nil {
		a.logger.LogError(err, "Error starting camera stream")
		err = apperror.ServerError.SetMessage(err.Error())
	}

	return stream, closeChan, err
}

func (a *App) StopStream() {
	a.logger.LogInfo("Stopping the stream")
}

func (a *App) StartRecording(filename string) error {
	var (
		camErr bool
		micErr bool
	)
	if err := a.camera.StartRecording(filename); err != nil {
		a.logger.LogError(err, "Error starting camera recording")
		camErr = true
	}

	a.uploader.InformRecordingStart()

	if err := a.mic.StartRecording(filename); err != nil {
		a.logger.LogError(err, "Error starting mic recording")
		micErr = true
	}

	if camErr && micErr {
		err := errors.New("error starting camera and mic recording")
		a.logger.LogError(err, "Error starting camera and mic recording")
		return apperror.ServerError.SetMessage(err.Error())
	}
	return nil
}

func (a *App) StopRecording() {
	a.camera.StopRecording()
	a.mic.StopRecording()
	a.uploader.InformRecordingStop()
}

func (a *App) UploadRecording(filename string) error {
	return a.uploader.UploadRecording(filename)
}

func (a *App) UploadRecordings() error {
	return a.uploader.UploadRecordings()
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

	files, err := helper.FetchFiles()

	if err != nil {
		a.logger.LogError(err, "Error reading videos folder", "folder_name", videosFolder)
		return nil, apperror.ServerError
	}

	var (
		fileDetails []models.FileDetails
		ext         string
	)

	for _, file := range files {
		ext = file[len(file)-4:]
		if ext != ".avi" && ext != ".wav" {
			continue
		}
		fileDetail := models.FileDetails{
			Filename: file,
		}

		if camRecording, filename := a.camera.RecordingStats(); camRecording && file == filename {
			fileDetail.Recording = true
		} else if fileUploading, filename := a.uploader.UploadStats(); fileUploading && file == filename {
			fileDetail.Uploading = true
		}

		fileDetails = append(fileDetails, fileDetail)
	}

	return fileDetails, nil
}

func (a *App) AppStatus() *models.Status {
	var stat unix.Statfs_t
	recordStat, _ := a.camera.RecordingStats()
	uploadStat, _ := a.uploader.UploadStats()

	if err := unix.Statfs("/home", &stat); err != nil {
		a.logger.LogError(err, "Error getting disk usage")
		return nil
	}

	availableBlocks := float32(stat.Bavail) * float32(stat.Bsize)
	totalBlocks := float32(stat.Blocks) * float32(stat.Bsize)
	availPercentage := (100 - ((availableBlocks / totalBlocks) * 100)) / 100

	availPercentage = float32(helper.Truncate(float64(availPercentage), 0.01))

	return &models.Status{
		CameraUp:  a.camera.CamStatus(),
		Recording: recordStat,
		Uploading: uploadStat,
		DiskUsage: availPercentage,
	}
}
