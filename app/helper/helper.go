package helper

import (
	"errors"
	"math/big"
	"os"
	"pirecorder/apperror"
	"pirecorder/config"
)

func FetchFiles() ([]string, error) {
	var (
		files      []string
		videoError bool
		audioError bool
		//err        error
	)

	videosFolder := config.GetConfig().VideosFolder
	audiosFolder := config.GetConfig().AudiosFolder

	fd, err := os.Open(videosFolder)

	if err != nil {
		videoError = true
	}

	videoFiles, err := fd.Readdirnames(0)

	if err != nil {
		videoError = true
	} else {
		files = append(files, videoFiles...)
	}

	_ = fd.Close()

	fd, err = os.Open(audiosFolder)

	if err != nil {
		audioError = true
	}

	audioFiles, err := fd.Readdirnames(0)

	if err != nil {
		audioError = true
	} else {
		files = append(files, audioFiles...)
	}

	_ = fd.Close()

	if videoError && audioError {
		err = errors.New("error reading videos and audios folder")
		return nil, apperror.ServerError.SetMessage(err.Error())
	}

	return files, nil
}

func Truncate(num float64, unit float64) float64 {
	bf := big.NewFloat(0).SetPrec(1000).SetFloat64(num)
	bu := big.NewFloat(0).SetPrec(1000).SetFloat64(unit)

	bf.Quo(bf, bu)

	i := big.NewInt(0)
	bf.Int(i)
	bf.SetInt(i)

	f, _ := bf.Mul(bf, bu).Float64()

	return f
}
