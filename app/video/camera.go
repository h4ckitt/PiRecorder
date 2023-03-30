package video

import (
	"bytes"
	"fmt"
	"github.com/icza/mjpeg"
	"io"
	"os"
	"os/exec"
	"pirecorder/config"
	"pirecorder/logger"
	"time"
)

type Camera struct {
	isRecording bool
	isCamUp     bool
	recordName  string
	mux         *Mux
	logger      *logger.Logger
}

func NewCamera(logger *logger.Logger) (*Camera, error) {
	logger.LogInfo("Starting video checks")

	logger.LogInfo("Checking if videos folder exists.....")
	videosFolder := config.GetConfig().VideosFolder
	_, err := os.Stat(videosFolder)

	if err != nil {
		logger.LogWarning(err, "videos folder doesn't exist, creating it .......")
		if err = os.MkdirAll(videosFolder, 0755); err != nil {
			logger.LogError(err, "Failed to create videos folder")
			return &Camera{isCamUp: false}, err
		}
		logger.LogInfo("videos folder created successfully")
	}

	var mux *Mux
	stream, err := StartCamera()
	camUp := true

	if err != nil {
		logger.LogError(err, "Failed to start camera")
		camUp = false
	} else {
		mux = NewMux(stream)
	}

	return &Camera{
		isCamUp: camUp,
		mux:     mux,
		logger:  logger,
	}, nil
}

func StartCamera() (io.Reader, error) {
	var cmd *exec.Cmd

	switch config.GetConfig().Environment {
	case "dev":
		cmd = exec.Command("ffmpeg", "-hide_banner", "-f", "v4l2", "-framerate", "30", "-video_size", "640x480", "-i", "/dev/video0", "-f", "mpjpeg", "-")
	case "prod":
		cmd = exec.Command("raspivid", "-o", "-", "-t", "0", "-w", "640", "-h", "480", "-fps", "30", "-cd", "MJPEG")
	default:
		return nil, fmt.Errorf("unknown environment")
	}

	pr, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return pr, nil
}

func (c *Camera) CamStatus() bool {
	return c.isCamUp
}

func (c *Camera) RecordingStats() (bool, string) {
	return c.isRecording, c.recordName
}

func (c *Camera) StartStream() (chan []byte, chan struct{}, error) {
	if !c.isCamUp {
		return nil, nil, fmt.Errorf("camera is not up")
	}
	c.logger.LogInfo("Starting video stream")
	streamChan := make(chan []byte, 65535)
	closeChan := make(chan struct{})

	var previousFrame []byte

	ticker := time.Tick(33 * time.Millisecond) // 30 fps

	go func() {
		for {
			select {
			case <-closeChan:
				c.logger.LogInfo("Closing video stream")
				return
			case <-ticker:
				frame := c.mux.GetFrame()

				if len(frame) == 0 || bytes.Equal(frame, previousFrame) {
					continue
				}

				streamChan <- frame
				previousFrame = frame
			}
		}
	}()

	return streamChan, closeChan, nil
}

func (c *Camera) StartRecording(filename string) error {
	if !c.isCamUp {
		return fmt.Errorf("camera is not up")
	}

	if c.isRecording {
		c.isRecording = false
	}

	aw, err := mjpeg.New(fmt.Sprintf("%s/%s.avi", config.GetConfig().VideosFolder, filename), 640, 480, 30)

	if err != nil {
		c.logger.LogError(err, "Error creating video file", "filename", filename)
		return err
	}

	c.isRecording = true
	c.recordName = fmt.Sprintf("%s.avi", filename)

	go func() {
		defer func() {
			_ = aw.Close()
			c.isRecording = false
			c.recordName = ""
		}()

		var previousFrame []byte
		ticker := time.Tick(33 * time.Millisecond) // 30 fps

		for c.isRecording {
			<-ticker
			frame := c.mux.GetFrame()

			if len(frame) == 0 || bytes.Equal(frame, previousFrame) {
				continue
			}

			err = aw.AddFrame(frame)

			if err != nil {
				c.logger.LogError(err, "Error adding frame to video file", "filename", filename)
			}
			previousFrame = frame
		}
	}()

	return nil
}

func (c *Camera) StopRecording() {
	c.logger.LogInfo("Stopping video recording", "filename", c.recordName)
	if c.isRecording {
		c.isRecording = false
	}
}
