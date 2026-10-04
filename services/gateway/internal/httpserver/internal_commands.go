package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"

	"starter/services/gateway/internal/operatorcontext"
)

// InternalCommand is one internal product route. NewHandler always registers it behind the
// service token and the verified operator context; there is no way to register it without them.
type InternalCommand struct {
	// Pattern is a net/http pattern with method, for example "POST /internal/runs".
	Pattern string
	Handler http.Handler
}

// RequireOperatorContext verifies the X-Operator-Context token and passes the verified operator
// to the handler through the request context. A missing verifier rejects every request.
func RequireOperatorContext(verifier *operatorcontext.Verifier) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			headerValues := request.Header.Values(operatorcontext.HeaderName)
			if verifier == nil || len(headerValues) != 1 {
				writeError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
				return
			}
			operator, err := verifier.Verify(headerValues[0])
			if err != nil {
				writeError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
				return
			}
			next.ServeHTTP(responseWriter, request.WithContext(operatorcontext.WithOperator(request.Context(), operator)))
		})
	}
}

// WriteError sends the shared ErrorResponse envelope from an internal command handler.
func WriteError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, code, message string) {
	writeError(responseWriter, request, statusCode, code, message)
}

// errMalformedBody covers every rejected request body; the client learns only that it was bad.
var errMalformedBody = errors.New("malformed request body")

// DecodeJSONBody reads at most maximumBytes of a JSON body into target: one document, valid
// UTF-8, no unknown fields, no duplicate or case-variant keys, no trailing data. On failure it has
// already answered 400 bad_request and returns false; the handler must stop.
func DecodeJSONBody(responseWriter http.ResponseWriter, request *http.Request, maximumBytes int64, target any) bool {
	if err := decodeJSONBody(responseWriter, request, maximumBytes, target); err != nil {
		writeError(responseWriter, request, http.StatusBadRequest, "bad_request", "The request body is not a valid command.")
		return false
	}
	return true
}

func decodeJSONBody(responseWriter http.ResponseWriter, request *http.Request, maximumBytes int64, target any) error {
	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return errMalformedBody
		}
	}
	// One byte over the limit tells an oversized body apart from one exactly at the limit.
	body, err := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, maximumBytes))
	if err != nil || !utf8.Valid(body) {
		return errMalformedBody
	}
	if err := rejectDuplicateKeys(body); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errMalformedBody
	}
	// decoder.More is false before a stray closing brace or bracket, so it would let `{...}}`
	// through; only the end of the input is acceptable after the document.
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errMalformedBody
	}
	return requireExactKeys(body, target)
}
