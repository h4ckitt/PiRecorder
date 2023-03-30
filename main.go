package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
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

	sslConfig := config.GetConfig().SSLConfig

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.GetConfig().Port),
		Handler: r,
	}
	var useSSL bool

	if sslConfig.CertFile != "" && sslConfig.KeyFile != "" {
		if _, err = os.Stat(sslConfig.CertFile); os.IsNotExist(err) {
			logman.LogError(err, "SSL Cert file not found")
		} else if _, err = os.Stat(sslConfig.KeyFile); os.IsNotExist(err) {
			logman.LogError(err, "SSL Key file not found")
		} else {
			cert, _ := tls.LoadX509KeyPair(sslConfig.CertFile, sslConfig.KeyFile)
			server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			useSSL = true
		}
	}

	if useSSL {
		logman.LogInfo(fmt.Sprintf("Starting server on port %s with SSL", config.GetConfig().Port))
		if err = server.ListenAndServeTLS("", ""); err != nil {
			logman.LogError(err, "Error starting server")
		}
	} else {
		logman.LogInfo(fmt.Sprintf("Starting server on port %s without SSL", config.GetConfig().Port))
		if err = server.ListenAndServe(); err != nil {
			logman.LogError(err, "Error starting server")
		}
	}
}
