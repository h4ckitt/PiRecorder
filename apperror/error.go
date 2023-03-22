package apperror

import "net/http"

type Apperror struct {
	status  int
	message string
	err     error
}

var (
	ServiceUnavailable = Apperror{status: http.StatusServiceUnavailable, message: "Server Not Ready To Process This Request"}
	ServerError        = Apperror{status: http.StatusInternalServerError, message: "Internal Server Error"}
	InvalidRequest     = Apperror{status: http.StatusBadRequest, message: "Invalid Request Body Received"}
	NotFound           = Apperror{status: http.StatusNotFound, message: "Resource Not Found On This Server"}
)

func (e Apperror) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.message
}

func (e Apperror) SetMessage(message string) Apperror {
	e.message = message
	return e
}

func (e Apperror) Is(target error) bool {
	t, ok := target.(Apperror)

	if !ok {
		return false
	}

	if t.status != e.status {
		return false
	}
	return true
}

func (e Apperror) StatusAndMessage() (int, string) {
	return e.status, e.message
}
