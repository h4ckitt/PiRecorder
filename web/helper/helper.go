package helper

import (
	"encoding/json"
	"errors"
	"net/http"
	"pirecorder/apperror"
	"strconv"
)

func ReturnFailure(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var errr apperror.Apperror
	switch {
	case errors.As(err, &errr):
		code, msg := errr.StatusAndMessage()
		w.WriteHeader(code)
		w.Header().Set("status", strconv.Itoa(code))
		json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}
}

func ReturnSuccess(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("status", strconv.Itoa(http.StatusOK))
	w.WriteHeader(http.StatusOK)

	if data == nil {
		return
	}

	if msg, ok := data.(map[string]string); ok {
		json.NewEncoder(w).Encode(msg)
		return
	}
	json.NewEncoder(w).Encode(data)
}
