package httpserver

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"starter/services/gateway/internal/logging"
)

// requestIDHeader is "x-request-id" in Go's canonical header form.
const requestIDHeader = "X-Request-Id"

const maximumRequestIDLength = 64

// Middleware wraps a handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares so the first one listed runs outermost.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for index := len(middlewares) - 1; index >= 0; index-- {
		handler = middlewares[index](handler)
	}
	return handler
}

// RequestID reuses a safe inbound x-request-id or generates one, echoes it on
// the response and attaches a logger carrying request_id to the context.
func RequestID(baseLogger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			requestID := request.Header.Get(requestIDHeader)
			if !isSafeRequestID(requestID) {
				requestID = newRequestID()
			}
			responseWriter.Header().Set(requestIDHeader, requestID)

			requestLogger := baseLogger.With("request_id", requestID)
			next.ServeHTTP(responseWriter, request.WithContext(logging.WithLogger(request.Context(), requestLogger)))
		})
	}
}

// isSafeRequestID bounds client input before it reaches logs and headers:
// 1-64 characters of [A-Za-z0-9._-].
func isSafeRequestID(requestID string) bool {
	if len(requestID) == 0 || len(requestID) > maximumRequestIDLength {
		return false
	}
	for _, character := range requestID {
		isLetter := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
		isDigit := character >= '0' && character <= '9'
		if !isLetter && !isDigit && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var randomBytes [16]byte
	// crypto/rand.Read never returns an error since Go 1.24 (it aborts instead).
	_, _ = rand.Read(randomBytes[:])
	return hex.EncodeToString(randomBytes[:])
}

// statusRecorder remembers the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	if recorder.statusCode == 0 {
		recorder.statusCode = statusCode
	}
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if recorder.statusCode == 0 {
		recorder.statusCode = http.StatusOK
	}
	return recorder.ResponseWriter.Write(body)
}

// Unwrap lets http.ResponseController reach the real writer.
func (recorder *statusRecorder) Unwrap() http.ResponseWriter { return recorder.ResponseWriter }

// AccessLog writes one line per request with method, path, status, duration
// and request_id only: no query string, headers or bodies.
func AccessLog(baseLogger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{ResponseWriter: responseWriter}

			// Deferred so a panicking handler is still logged.
			defer func() {
				level := slog.LevelInfo
				if strings.HasPrefix(request.URL.Path, "/health/") {
					level = slog.LevelDebug // probes are frequent
				}
				logging.FromContext(request.Context(), baseLogger).Log(request.Context(), level, "http request",
					"method", request.Method,
					"path", request.URL.Path,
					"status", recorder.statusCode,
					"duration_ms", time.Since(startedAt).Milliseconds(),
				)
			}()
			next.ServeHTTP(recorder, request)
		})
	}
}

// Recover turns a handler panic into the generic JSON 500 envelope.
func Recover(baseLogger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			defer func() {
				panicValue := recover()
				if panicValue == nil {
					return
				}
				// ErrAbortHandler is the sanctioned way to abort a response.
				if panicValue == http.ErrAbortHandler {
					panic(panicValue)
				}
				// Details stay in the server log; the client gets a fixed message.
				logging.FromContext(request.Context(), baseLogger).Error("handler panic",
					"panic", panicValue,
					"stack", string(debug.Stack()),
				)
				writeError(responseWriter, request, http.StatusInternalServerError, "internal_error", "Internal server error.")
			}()
			next.ServeHTTP(responseWriter, request)
		})
	}
}

// RequireServiceToken guards a route with "Authorization: Bearer <token>".
func RequireServiceToken(expectedToken logging.Secret) Middleware {
	// Hashing both sides gives equal lengths, so the comparison leaks nothing.
	expectedHash := sha256.Sum256([]byte(expectedToken.Reveal()))
	hasExpectedToken := expectedToken.Len() > 0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			presentedToken, hasBearerToken := bearerToken(request.Header.Get("Authorization"))
			presentedHash := sha256.Sum256([]byte(presentedToken))
			tokensMatch := subtle.ConstantTimeCompare(presentedHash[:], expectedHash[:]) == 1

			// An empty expected token must never authenticate anyone.
			if !hasBearerToken || !tokensMatch || !hasExpectedToken {
				responseWriter.Header().Set("WWW-Authenticate", `Bearer realm="internal"`)
				writeError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid service token.")
				return
			}
			next.ServeHTTP(responseWriter, request)
		})
	}
}

// bearerToken extracts the token; the scheme is case-insensitive (RFC 9110).
func bearerToken(authorizationHeader string) (string, bool) {
	scheme, token, hasSeparator := strings.Cut(authorizationHeader, " ")
	if !hasSeparator || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
