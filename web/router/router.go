package router

import (
	"net/http"
	"pirecorder/logger"
	"pirecorder/web/controller"

	"github.com/gorilla/mux"
)

func InitRouter(controller *controller.Controller, logger *logger.Logger) *mux.Router {
	router := mux.NewRouter()
	router.Use(logger.LogRequest)

	router.HandleFunc("/api/status", controller.DeviceStatus).Methods(http.MethodGet)

	filerouter := router.PathPrefix("/file").Subrouter()
	filerouter.HandleFunc("/upload", controller.UploadFile).Methods(http.MethodPost)
	filerouter.HandleFunc("/upload-list", controller.ListFiles).Methods(http.MethodGet)
	filerouter.HandleFunc("/upload-all", controller.UploadAllFiles).Methods(http.MethodPost)

	camerarouter := router.PathPrefix("/camera").Subrouter()
	camerarouter.HandleFunc("/start-recording", controller.StartRecording).Methods(http.MethodPost)
	camerarouter.HandleFunc("/stop-recording", controller.StopRecording).Methods(http.MethodPost)
	camerarouter.HandleFunc("/stream.mjpeg", controller.ShowStream).Methods(http.MethodGet)

	return router
}
