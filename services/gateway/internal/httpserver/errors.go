package httpserver

import (
	"net/http"
	"path"
	"time"

	"starter/services/gateway/internal/health"
)

// timestampLayout is ISO 8601 in UTC with milliseconds, like JavaScript's toISOString.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// writeError sends the shared ErrorResponse envelope.
func writeError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, code, message string) {
	// The RequestID middleware runs outermost and has already set the header.
	requestID := responseWriter.Header().Get(requestIDHeader)
	if requestID == "" {
		requestID = newRequestID()
		responseWriter.Header().Set(requestIDHeader, requestID)
	}
	health.WriteJSON(responseWriter, statusCode, health.ErrorResponse{
		Error:      health.ErrorDetail{Code: code, Message: message},
		StatusCode: statusCode,
		RequestID:  requestID,
		Timestamp:  time.Now().UTC().Format(timestampLayout),
		Path:       request.URL.Path,
	})
}

// statusCapture records what the mux's built-in handlers would have sent.
type statusCapture struct {
	header     http.Header
	statusCode int
}

func (capture *statusCapture) Header() http.Header { return capture.header }

func (capture *statusCapture) WriteHeader(statusCode int) {
	if capture.statusCode == 0 {
		capture.statusCode = statusCode
	}
}

func (capture *statusCapture) Write(body []byte) (int, error) {
	capture.WriteHeader(http.StatusOK)
	return len(body), nil
}

// withJSONErrors replaces the mux's plain-text 404, 405 and redirect answers
// with the JSON envelope, so only the registered exact routes ever respond.
func withJSONErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		routeHandler, matchedPattern := mux.Handler(request)
		isExactPath := request.URL.Path == path.Clean(request.URL.Path)
		if matchedPattern != "" && isExactPath {
			routeHandler.ServeHTTP(responseWriter, request)
			return
		}

		// No route matched: ask the mux whether it is a 405 (known path) or a 404.
		capture := &statusCapture{header: http.Header{}}
		routeHandler.ServeHTTP(capture, request)
		if capture.statusCode == http.StatusMethodNotAllowed {
			if allowedMethods := capture.header.Get("Allow"); allowedMethods != "" {
				responseWriter.Header().Set("Allow", allowedMethods)
			}
			writeError(responseWriter, request, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
			return
		}
		writeError(responseWriter, request, http.StatusNotFound, "not_found", "Route not found.")
	})
}
