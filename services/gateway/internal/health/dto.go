package health

// Wire shapes of the gateway. They mirror packages/contracts/src/index.ts;
// dto_test.go checks them against packages/contracts/fixtures.

// ServiceName identifies this service in health responses.
const ServiceName = "gateway"

const (
	StatusOK          = "ok"
	StatusUnavailable = "unavailable"
	CheckUp           = "up"
	CheckDown         = "down"
)

// LivenessResponse is the body of GET /health/live.
type LivenessResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// DependencyCheck is one dependency inside a readiness report.
type DependencyCheck struct {
	Status string `json:"status"`
	// Message is a short sanitized reason: no hosts, credentials or stack traces.
	Message string `json:"message,omitempty"`
}

// ReadinessChecks lists the dependencies readiness covers.
type ReadinessChecks struct {
	Database DependencyCheck `json:"database"`
}

// ReadinessResponse is the body of GET /health/ready (200 or 503).
type ReadinessResponse struct {
	Status  string          `json:"status"`
	Service string          `json:"service"`
	Checks  ReadinessChecks `json:"checks"`
}

// GatewayPingResponse is the body of GET /internal/ping.
type GatewayPingResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// ErrorDetail is the machine-readable code and safe message of an error.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the envelope for every non-health failure.
type ErrorResponse struct {
	Error      ErrorDetail `json:"error"`
	StatusCode int         `json:"statusCode"`
	RequestID  string      `json:"requestId"`
	// Timestamp is ISO 8601; kept as a string so the wire format is explicit.
	Timestamp string `json:"timestamp"`
	Path      string `json:"path,omitempty"`
}
