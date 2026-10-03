package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/testdb"
)

func TestProductionChainRequiresItsDependencies(t *testing.T) {
	if _, err := NewProductionChain(nil, catalog.NewLoader(), ChainConfig{Logger: slog.Default()}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil pool: %v", err)
	}
}

// Without a configured model the chain is still built (health and admission keep working), but
// no model request can be dispatched.
func TestUnconfiguredModelFailsEveryCallClosed(t *testing.T) {
	pool := testdb.Open(t)
	chain, err := NewProductionChain(pool, catalog.NewLoader(), ChainConfig{Logger: slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if chain.Worker == nil || chain.Loop == nil || chain.Gate == nil || chain.Inspector == nil || chain.Settings == nil {
		t.Fatalf("incomplete chain: %+v", chain)
	}
	if _, err = (unconfiguredProvider{}).Chat(context.Background(), model.Request{}); !errors.Is(err, ErrModelNotConfigured) {
		t.Fatalf("unconfigured provider: %v", err)
	}
	stepper, _ := NewStepper(chain.ModelCaller, &fakeRecorder{}, "unconfigured-model")
	if _, err = stepper.Step(context.Background(), Run{OrganizationID: "org", RunID: "run", AllowedModels: []string{"qwen3.5:4b"}}, testContext); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("unconfigured model step: %v", err)
	}
}
