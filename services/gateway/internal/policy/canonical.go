// Package policy is the action gate: it canonicalizes proposed actions, decides allow, deny or
// approval required, and owns exact-action approvals.
package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"
)

// CanonicalizationVersion is part of every hashed action, so a change to the encoding rules
// can never collide with digests computed under the old rules (GO-04).
const CanonicalizationVersion = 1

// ToolName is the name of one of the four registered tools.
type ToolName string

const (
	ToolReadInvoice  ToolName = "read_invoice"
	ToolReadVendor   ToolName = "read_vendor"
	ToolCreateReport ToolName = "create_report"
	ToolQueueReport  ToolName = "queue_report"
)

// The two fixed report templates of the prototype.
const (
	TemplateInternalInvestigation = "internal_investigation_v1"
	TemplateVendorReconciliation  = "vendor_reconciliation_v1"
)

// maximumIdentifierBytes bounds every identifier argument; longer values are rejected, not cut.
const maximumIdentifierBytes = 256

var (
	// ErrUnknownTool means the proposal names a tool that is not registered.
	ErrUnknownTool = errors.New("tool is not registered")
	// ErrInvalidArguments means the arguments do not decode strictly into the tool's typed form.
	ErrInvalidArguments = errors.New("tool arguments are invalid")
	// ErrInvalidAction means a field of the canonical action is missing or malformed.
	ErrInvalidAction = errors.New("canonical action is invalid")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Arguments are the typed, validated arguments of one registered tool.
type Arguments interface {
	Tool() ToolName
	validate() error
}

// ReadInvoiceArguments are the arguments of read_invoice.
type ReadInvoiceArguments struct {
	InvoiceID string `json:"invoice_id"`
}

// ReadVendorArguments are the arguments of read_vendor.
type ReadVendorArguments struct {
	VendorID string `json:"vendor_id"`
}

// CreateReportArguments are the arguments of create_report. SourceInvoiceIDs keeps its order.
type CreateReportArguments struct {
	Template         string   `json:"template"`
	SourceInvoiceIDs []string `json:"source_invoice_ids"`
}

// QueueReportArguments are the arguments of queue_report. RecipientReference names a trusted
// directory entry, never an address found in content.
type QueueReportArguments struct {
	ReportID           string `json:"report_id"`
	RecipientReference string `json:"recipient_reference"`
}

func (ReadInvoiceArguments) Tool() ToolName  { return ToolReadInvoice }
func (ReadVendorArguments) Tool() ToolName   { return ToolReadVendor }
func (CreateReportArguments) Tool() ToolName { return ToolCreateReport }
func (QueueReportArguments) Tool() ToolName  { return ToolQueueReport }

func (arguments ReadInvoiceArguments) validate() error {
	return validateIdentifier("invoice_id", arguments.InvoiceID)
}

func (arguments ReadVendorArguments) validate() error {
	return validateIdentifier("vendor_id", arguments.VendorID)
}

func (arguments CreateReportArguments) validate() error {
	if arguments.Template != TemplateInternalInvestigation && arguments.Template != TemplateVendorReconciliation {
		return fmt.Errorf("%w: template is not a registered template", ErrInvalidArguments)
	}
	if len(arguments.SourceInvoiceIDs) == 0 {
		return fmt.Errorf("%w: source_invoice_ids is empty", ErrInvalidArguments)
	}
	seenSourceInvoiceIDs := make(map[string]bool, len(arguments.SourceInvoiceIDs))
	for _, invoiceID := range arguments.SourceInvoiceIDs {
		if err := validateIdentifier("source_invoice_ids", invoiceID); err != nil {
			return err
		}
		if seenSourceInvoiceIDs[invoiceID] {
			return fmt.Errorf("%w: source_invoice_ids repeats a value", ErrInvalidArguments)
		}
		seenSourceInvoiceIDs[invoiceID] = true
	}
	return nil
}

func (arguments QueueReportArguments) validate() error {
	if !uuidPattern.MatchString(arguments.ReportID) {
		return fmt.Errorf("%w: report_id is not a lowercase UUID", ErrInvalidArguments)
	}
	return validateIdentifier("recipient_reference", arguments.RecipientReference)
}

// validateIdentifier accepts a non-empty, bounded value without control characters. The value is
// never trimmed or normalized: a value that would need it is rejected.
func validateIdentifier(fieldName, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is empty", ErrInvalidArguments, fieldName)
	}
	if len(value) > maximumIdentifierBytes {
		return fmt.Errorf("%w: %s is longer than %d bytes", ErrInvalidArguments, fieldName, maximumIdentifierBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%w: %s contains a control character", ErrInvalidArguments, fieldName)
		}
	}
	return nil
}

// DecodeArguments strictly decodes raw JSON arguments into the typed form of the named tool:
// invalid UTF-8, unknown, duplicate, missing or null fields, wrong types and trailing data are
// rejected before any canonical form exists.
func DecodeArguments(tool ToolName, rawArguments []byte) (Arguments, error) {
	switch tool {
	case ToolReadInvoice:
		return decodeTyped[ReadInvoiceArguments](rawArguments)
	case ToolReadVendor:
		return decodeTyped[ReadVendorArguments](rawArguments)
	case ToolCreateReport:
		return decodeTyped[CreateReportArguments](rawArguments)
	case ToolQueueReport:
		return decodeTyped[QueueReportArguments](rawArguments)
	default:
		return nil, ErrUnknownTool
	}
}

func decodeTyped[T Arguments](rawArguments []byte) (Arguments, error) {
	// encoding/json silently replaces invalid UTF-8, so check the raw bytes first.
	if !utf8.Valid(rawArguments) {
		return nil, fmt.Errorf("%w: not valid UTF-8", ErrInvalidArguments)
	}
	if err := rejectDuplicateKeysAndNulls(rawArguments); err != nil {
		return nil, err
	}
	var typedArguments T
	decoder := json.NewDecoder(bytes.NewReader(rawArguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&typedArguments); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArguments, describeDecodeError(err))
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data after the arguments object", ErrInvalidArguments)
	}
	if err := requireAllFields(rawArguments, typedArguments); err != nil {
		return nil, err
	}
	if err := typedArguments.validate(); err != nil {
		return nil, err
	}
	return typedArguments, nil
}

// describeDecodeError keeps the decoder's reason but never echoes argument values.
func describeDecodeError(err error) string {
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return fmt.Sprintf("field %q has the wrong type", typeError.Field)
	}
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return "malformed JSON"
	}
	return "does not match the tool's fields"
}

// rejectDuplicateKeysAndNulls walks the JSON tokens: the arguments must be one object, no object
// may repeat a key, and no value may be null (absent and null are never treated as the same).
func rejectDuplicateKeysAndNulls(rawArguments []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(rawArguments))
	firstToken, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
	}
	if delimiter, isDelimiter := firstToken.(json.Delim); !isDelimiter || delimiter != '{' {
		return fmt.Errorf("%w: arguments must be a JSON object", ErrInvalidArguments)
	}
	return walkObject(decoder)
}

func walkObject(decoder *json.Decoder) error {
	seenKeys := map[string]bool{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
		}
		key, _ := keyToken.(string)
		if seenKeys[key] {
			return fmt.Errorf("%w: field %q appears more than once", ErrInvalidArguments, key)
		}
		seenKeys[key] = true
		if err := walkValue(decoder, key); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil { // closing brace
		return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
	}
	return nil
}

func walkValue(decoder *json.Decoder, fieldName string) error {
	valueToken, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
	}
	switch typedToken := valueToken.(type) {
	case nil:
		return fmt.Errorf("%w: field %q is null", ErrInvalidArguments, fieldName)
	case json.Delim:
		if typedToken == '{' {
			return walkObject(decoder)
		}
		if typedToken == '[' {
			for decoder.More() {
				if err := walkValue(decoder, fieldName); err != nil {
					return err
				}
			}
			if _, err := decoder.Token(); err != nil { // closing bracket
				return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
			}
		}
	}
	return nil
}

// requireAllFields checks the exact key set: every argument field is required (the typed zero
// value must never stand in for an absent one), and keys must match exactly, because
// encoding/json would otherwise accept "INVOICE_ID" as invoice_id.
func requireAllFields(rawArguments []byte, typedArguments Arguments) error {
	var presentFields map[string]json.RawMessage
	if err := json.Unmarshal(rawArguments, &presentFields); err != nil {
		return fmt.Errorf("%w: malformed JSON", ErrInvalidArguments)
	}
	requiredFields := requiredFieldNames(typedArguments.Tool())
	for _, fieldName := range requiredFields {
		if _, present := presentFields[fieldName]; !present {
			return fmt.Errorf("%w: field %q is missing", ErrInvalidArguments, fieldName)
		}
	}
	if len(presentFields) != len(requiredFields) {
		return fmt.Errorf("%w: unknown field", ErrInvalidArguments)
	}
	return nil
}

func requiredFieldNames(tool ToolName) []string {
	switch tool {
	case ToolReadInvoice:
		return []string{"invoice_id"}
	case ToolReadVendor:
		return []string{"vendor_id"}
	case ToolCreateReport:
		return []string{"template", "source_invoice_ids"}
	case ToolQueueReport:
		return []string{"report_id", "recipient_reference"}
	}
	return nil
}

// CanonicalArguments returns the one compact JSON encoding of validated arguments: fields in the
// struct's fixed order, no insignificant whitespace, HTML characters not escaped.
func CanonicalArguments(arguments Arguments) ([]byte, error) {
	if arguments == nil {
		return nil, fmt.Errorf("%w: no arguments", ErrInvalidArguments)
	}
	if err := arguments.validate(); err != nil {
		return nil, err
	}
	return compactJSON(arguments)
}

func compactJSON(value any) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n")), nil
}

// ResourceVersion names one affected record and the version the action was proposed against.
type ResourceVersion struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

// CanonicalAction is everything an action's digest covers (GO-04). The digest and the mutable
// execution status are deliberately not part of it.
type CanonicalAction struct {
	ActionID          string
	RunID             string
	Arguments         Arguments
	PassportID        string
	PolicyRevisionID  int64
	Recipient         *string
	AffectedResources []ResourceVersion
	OutboundContent   *string
	ExpiresAt         *time.Time
}

// hashedAction is the fixed-order wire form that is hashed. Absent optional values are null.
type hashedAction struct {
	CanonicalizationVersion int               `json:"canonicalization_version"`
	ActionID                string            `json:"action_id"`
	RunID                   string            `json:"run_id"`
	Tool                    ToolName          `json:"tool"`
	Arguments               json.RawMessage   `json:"arguments"`
	PassportID              string            `json:"passport_id"`
	PolicyRevisionID        int64             `json:"policy_revision_id"`
	Recipient               *string           `json:"recipient"`
	AffectedResources       []ResourceVersion `json:"affected_resources"`
	OutboundContent         *string           `json:"outbound_content"`
	ExpiresAt               *string           `json:"expires_at"`
}

// Encode returns the canonical bytes of the action, the input of its digest.
func (action CanonicalAction) Encode() ([]byte, error) {
	if err := action.validate(); err != nil {
		return nil, err
	}
	canonicalArguments, err := CanonicalArguments(action.Arguments)
	if err != nil {
		return nil, err
	}
	var expiresAt *string
	if action.ExpiresAt != nil {
		// UTC with full precision: an expiry is never rounded.
		formattedExpiry := action.ExpiresAt.UTC().Format(time.RFC3339Nano)
		expiresAt = &formattedExpiry
	}
	affectedResources := action.AffectedResources
	if affectedResources == nil {
		affectedResources = []ResourceVersion{} // one representation for "no resources"
	}
	return compactJSON(hashedAction{
		CanonicalizationVersion: CanonicalizationVersion,
		ActionID:                action.ActionID,
		RunID:                   action.RunID,
		Tool:                    action.Arguments.Tool(),
		Arguments:               canonicalArguments,
		PassportID:              action.PassportID,
		PolicyRevisionID:        action.PolicyRevisionID,
		Recipient:               action.Recipient,
		AffectedResources:       affectedResources,
		OutboundContent:         action.OutboundContent,
		ExpiresAt:               expiresAt,
	})
}

// Digest is SHA-256 over the canonical bytes. It identifies the stored action and detects a
// change; it never authenticates an author or authorizes the action.
func (action CanonicalAction) Digest() ([sha256.Size]byte, error) {
	canonicalBytes, err := action.Encode()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(canonicalBytes), nil
}

func (action CanonicalAction) validate() error {
	identifierFields := []struct{ name, value string }{
		{"action_id", action.ActionID}, {"run_id", action.RunID}, {"passport_id", action.PassportID},
	}
	for _, identifierField := range identifierFields {
		if !uuidPattern.MatchString(identifierField.value) {
			return fmt.Errorf("%w: %s is not a lowercase UUID", ErrInvalidAction, identifierField.name)
		}
	}
	if action.Arguments == nil {
		return fmt.Errorf("%w: no arguments", ErrInvalidAction)
	}
	if action.PolicyRevisionID <= 0 {
		return fmt.Errorf("%w: policy_revision_id must be positive", ErrInvalidAction)
	}
	if action.Recipient != nil {
		if err := validateIdentifier("recipient", *action.Recipient); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAction, err)
		}
	}
	if action.OutboundContent != nil && !utf8.ValidString(*action.OutboundContent) {
		return fmt.Errorf("%w: outbound_content is not valid UTF-8", ErrInvalidAction)
	}
	for _, resource := range action.AffectedResources {
		if err := validateIdentifier("affected_resources.kind", resource.Kind); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAction, err)
		}
		if err := validateIdentifier("affected_resources.id", resource.ID); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAction, err)
		}
		if resource.Version <= 0 {
			return fmt.Errorf("%w: affected_resources.version must be positive", ErrInvalidAction)
		}
	}
	return nil
}
