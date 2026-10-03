package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// ErrMalformed means a document does not decode strictly into its contract type.
var ErrMalformed = errors.New("document does not match its contract")

// DecodeStrict decodes exactly one JSON document into target, rejecting unknown fields and
// trailing data. It does not check required fields or value ranges; callers validate those.
func DecodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrMalformed
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrMalformed
	}
	return nil
}
