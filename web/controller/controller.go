package controller

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"pirecorder/app"
	"pirecorder/apperror"
	"pirecorder/logger"
	"pirecorder/web/helper"
)

type Controller struct {
	logger *logger.Logger
	app    *app.App
}

func NewController(app *app.App, logger *logger.Logger) *Controller {
	return &Controller{
		app:    app,
		logger: logger,
	}
}

func (c *Controller) ShowStream(w http.ResponseWriter, r *http.Request) {
	mimewriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mimewriter.Boundary()))
	partHeader := make(textproto.MIMEHeader)
	partHeader.Add("Content-Type", "image/jpeg")
	frames, closeChan := c.app.StartStream()
	defer close(closeChan)

	/*fr := <-frames

	pic, err := os.Create("test.jpg")
	fmt.Println(err)
	pic.Write(fr)
	pic.Close()

	part, err := mimewriter.CreatePart(partHeader)
	if err != nil {
		c.logger.LogError(err, "Error creating part")
		return
	}
	//fmt.Println(frame)
	if _, err = part.Write(fr); err != nil {
		c.logger.LogError(err, "Error writing frame")
	}*/

	for frame := range frames {
		if len(frame) == 0 {
			continue
		}
		part, err := mimewriter.CreatePart(partHeader)
		if err != nil {
			c.logger.LogError(err, "Error creating part")
			return
		}
		//fmt.Println(frame)
		_, err = part.Write(frame)

		if err != nil {
			c.logger.LogError(err, "Error writing frame")
			continue
		}
		//fmt.Println(n, " Frames Written")
	}
}

func (c *Controller) StartRecording(w http.ResponseWriter, r *http.Request) {
	p := struct {
		Filename string `json:"filename"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		c.logger.LogError(err, "Error getting filename for recording from request")
		helper.ReturnFailure(w, apperror.InvalidRequest)
		return
	}

	if err := c.app.StartRecording(p.Filename); err != nil {
		c.logger.LogError(err, "Error starting recording", "filename", p.Filename)
		helper.ReturnFailure(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *Controller) StopRecording(w http.ResponseWriter, r *http.Request) {
	c.app.StopRecording()
	c.logger.LogInfo("stopping recording")
}

func (c *Controller) UploadFile(w http.ResponseWriter, r *http.Request) {
	c.logger.LogInfo("upload file request received")

	file := struct {
		FileName string `json:"fileName"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
		helper.ReturnFailure(w, apperror.InvalidRequest)
		return
	}

	fmt.Println(file)

	if err := c.app.UploadRecording(file.FileName); err != nil {
		helper.ReturnFailure(w, err)
		return
	}

	helper.ReturnSuccess(w, nil)
}

func (c *Controller) ListFiles(w http.ResponseWriter, r *http.Request) {
	c.logger.LogInfo("list files request received")

	files, err := c.app.FetchRecordings()

	if err != nil {
		helper.ReturnFailure(w, err)
		return
	}

	helper.ReturnSuccess(w, files)
}

func (c *Controller) UploadAllFiles(w http.ResponseWriter, _ *http.Request) {
	c.logger.LogInfo("upload all files request received")
	err := c.app.UploadRecordings()

	if err != nil {
		helper.ReturnFailure(w, err)
		return
	}

	helper.ReturnSuccess(w, nil)
}

func (c *Controller) DeviceStatus(w http.ResponseWriter, r *http.Request) {
	c.logger.LogInfo("fetching device status")
	helper.ReturnSuccess(w, c.app.AppStatus())
}