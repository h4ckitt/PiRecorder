package main

import (
	"fmt"
	"log"
	"net/http"
	"pirecorder/app"
	"pirecorder/config"
	"pirecorder/logger"
	"pirecorder/web/controller"
	"pirecorder/web/router"
	"time"
)

func main() {
	config.Load()
	logfile := fmt.Sprintf("pirecorder_logs_%s.log", time.Now().Format("2006-01-02_15:04:05"))

	logman, err := logger.NewLogger(fmt.Sprintf("%s/%s", config.GetConfig().LogFolder, logfile))

	if err != nil {
		log.Fatal(err)
	}

	svc, err := app.NewApp(logman)

	if err != nil {
		logman.LogError(err, "Error creating app")
	}

	ctrl := controller.NewController(svc, logman)
	r := router.InitRouter(ctrl, logman)

	logman.LogInfo("Starting server on port 8080")

	if err = http.ListenAndServe(fmt.Sprintf(":%s", config.GetConfig().Port), r); err != nil {
		logman.LogError(err, "Error starting server")
	}
}
