package logger

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
)

type Logger struct {
	logger *logrus.Logger
}

func NewLogger(filename string) (*Logger, error) {
	dirname := filepath.Dir(filename)
	fmt.Println(dirname)
	_, err := os.Stat(dirname)

	if err != nil {
		err = os.MkdirAll(dirname, 0755)
		if err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return nil, err
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	//logger.SetReportCaller(true)
	logger.SetOutput(file)

	return &Logger{
		logger: logger,
	}, nil
}

// [a, b, c, d, e, f]

func convertToFields(values []any) (fields logrus.Fields) {
	fields = make(logrus.Fields)
	for i := 0; i <= len(values)-2; i += 2 {
		if val, ok := values[i+1].(string); ok {
			fields[values[i].(string)] = val
		}
	}
	return
}

// LogRequest : Logging Middleware
func (l *Logger) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		l.logger.WithFields(logrus.Fields{
			"method": r.Method,
			"uri":    r.RequestURI,
			"status": w.Header().Get("status"),
		}).Info("request received")
	})
}

func (l *Logger) LogError(err error, msg string, extras ...any) {
	if len(extras) > 0 && len(extras)%2 == 0 {
		extras = append(extras, "message", msg)
		l.logger.WithFields(convertToFields(extras)).Errorln(err)
		return
	}
	l.logger.WithFields(logrus.Fields{
		"message": msg,
	}).Errorln(err)
}

func (l *Logger) LogInfo(msg string, extras ...any) {
	if len(extras) > 0 && len(extras)%2 == 0 {
		l.logger.WithFields(convertToFields(extras)).Infoln(msg)
		return
	}
	l.logger.Infoln(msg)
}

func (l *Logger) LogWarning(err error, msg string, extras ...any) {
	if len(extras) > 0 && len(extras)%2 == 0 {
		extras = append(extras, "message", msg)
		l.logger.WithFields(convertToFields(extras)).Warnln(err)
		return
	}
	l.logger.WithFields(logrus.Fields{
		"message": msg,
	}).Warnln(err)
}
