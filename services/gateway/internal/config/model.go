package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Model is loaded explicitly by model clients, without gateway startup effects.
type Model struct {
	BaseURL string
	Name    string
}

func LoadModel() (Model, error) {
	return LoadModelFrom(os.LookupEnv)
}

// LoadModelFrom requires an explicit model tag; no candidate is hardcoded.
func LoadModelFrom(lookup LookupFunc) (Model, error) {
	reader := environmentReader{lookup: lookup}
	baseURL, _ := lookup("MODEL_BASE_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	name := reader.required("MODEL_NAME")
	if name != "" && strings.ContainsFunc(name, func(character rune) bool {
		return unicode.IsSpace(character) || unicode.IsControl(character)
	}) {
		reader.addProblem("MODEL_NAME must be an exact model tag without whitespace or control characters")
	}
	parsedURL, err := url.Parse(baseURL)
	validURL := err == nil && parsedURL != nil
	if validURL {
		validURL = (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") &&
			parsedURL.Hostname() != "" && parsedURL.User == nil &&
			parsedURL.RawQuery == "" && !parsedURL.ForceQuery &&
			parsedURL.Fragment == "" && parsedURL.RawFragment == "" && !strings.Contains(baseURL, "#") &&
			(parsedURL.Path == "" || parsedURL.Path == "/") && parsedURL.RawPath == ""
		if strings.HasSuffix(parsedURL.Host, ":") {
			validURL = false
		}
		if port := parsedURL.Port(); port != "" {
			portNumber, portError := strconv.Atoi(port)
			validURL = validURL && portError == nil && portNumber >= 1 && portNumber <= 65535
		}
	}
	if !validURL {
		reader.addProblem("MODEL_BASE_URL must be an HTTP or HTTPS origin without credentials, query, fragment or non-root path")
	}
	if len(reader.problems) > 0 {
		return Model{}, &ValidationError{Problems: reader.problems}
	}
	return Model{BaseURL: baseURL, Name: name}, nil
}
