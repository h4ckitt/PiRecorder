package audio

import (
	"errors"
	"fmt"
	"os"
	"pirecorder/apperror"
	"pirecorder/config"
	"pirecorder/logger"

	"github.com/jfreymuth/pulse"
)

type Mic struct {
	isMicUp     bool
	isRecording bool
	filename    string
	logger      *logger.Logger
	recorder    *pulse.Client
	audioClose  chan struct{}
}

func NewMic(logger *logger.Logger) (*Mic, error) {
	logger.LogInfo("Starting audio checks")

	logger.LogInfo("Checking if audios folder exists.....")
	audiosFolder := config.GetConfig().AudiosFolder
	_, err := os.Stat(audiosFolder)

	if err != nil {
		logger.LogWarning(err, "audios folder doesn't exist, creating it .......")
		if err = os.MkdirAll(audiosFolder, 0755); err != nil {
			logger.LogError(err, "Failed to create audios folder", "folder", audiosFolder)
			return &Mic{isMicUp: false}, err
		}
		logger.LogInfo("audios folder created successfully")
	}

	client, err := pulse.NewClient()
	isMicUp := true

	if err != nil {
		logger.LogError(err, "Error creating pulse client, mic probably not available")
		isMicUp = false
	}
	return &Mic{
		isMicUp:  isMicUp,
		logger:   logger,
		recorder: client,
	}, nil
}

func (m *Mic) StartRecording(filename string) error {
	if !m.isMicUp {
		return errors.New("mic not available")
	}
	if m.isRecording {
		m.isRecording = false
	}

	file, err := NewFile(fmt.Sprintf("%s/%s.wav", config.GetConfig().AudiosFolder, filename), 44100, 32, 1)

	if err != nil {
		m.logger.LogError(err, "Error creating audio file", "filename", filename)
		return apperror.ServerError
	}

	m.isRecording = true
	m.filename = filename
	m.audioClose = make(chan struct{})

	go func() {
		defer func() {
			_ = file.Close()
			m.isRecording = false
			m.filename = ""
		}()
		stream, err := m.recorder.NewRecord(pulse.Float32Writer(file.WriteSamples))

		if err != nil {
			m.logger.LogError(err, "Error creating pulse stream")
			return
		}

		stream.Start()

		<-m.audioClose
		fmt.Println("Stopping streammmm")
		stream.Stop()
	}()
	return nil
}

func (m *Mic) StopRecording() {
	m.logger.LogInfo("Stopping audio recording", "filename", m.filename)
	if m.isRecording {
		close(m.audioClose)
	}
}
