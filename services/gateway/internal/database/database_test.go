package database

import (
	"context"
	"testing"
	"time"

	"starter/services/gateway/internal/logging"
)

func TestBuildPoolConfigKeepsSpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
		database string
	}{
		{"plain", "starter", "plain-password", "starter"},
		{"url delimiters", "starter", `p@ss:w/rd?#&%=+ 'x"y\z`, "starter"},
		{"percent sequences", "app user", "100%25 %zz", "app db"},
		{"unicode", "starter", "pässwörd-✓", "starter"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			poolConfig, err := buildPoolConfig(Options{
				Host:           "db.internal",
				Port:           6543,
				User:           testCase.user,
				Password:       logging.NewSecret(testCase.password),
				Database:       testCase.database,
				ConnectTimeout: 2 * time.Second,
			})
			if err != nil {
				t.Fatalf("buildPoolConfig: %v", err)
			}
			connectionConfig := poolConfig.ConnConfig
			if connectionConfig.Password != testCase.password {
				t.Errorf("password was altered by escaping")
			}
			if connectionConfig.User != testCase.user || connectionConfig.Database != testCase.database {
				t.Errorf("user/database = %q/%q, want %q/%q", connectionConfig.User, connectionConfig.Database, testCase.user, testCase.database)
			}
			if connectionConfig.Host != "db.internal" || connectionConfig.Port != 6543 {
				t.Errorf("address = %s:%d, want db.internal:6543", connectionConfig.Host, connectionConfig.Port)
			}
			if poolConfig.MinConns != 0 || connectionConfig.ConnectTimeout != 2*time.Second {
				t.Errorf("MinConns = %d, ConnectTimeout = %s", poolConfig.MinConns, connectionConfig.ConnectTimeout)
			}
		})
	}
}

func TestNewPoolStartsWhilePostgresIsDown(t *testing.T) {
	// Port 1 on loopback refuses connections immediately.
	pool, err := NewPool(context.Background(), Options{
		Host:           "127.0.0.1",
		Port:           1,
		User:           "starter",
		Password:       logging.NewSecret("unused-password"),
		Database:       "starter",
		ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewPool must not dial at startup: %v", err)
	}
	defer pool.Close()

	pingContext, cancelPing := context.WithTimeout(context.Background(), time.Second)
	defer cancelPing()
	if err := pool.Ping(pingContext); err == nil {
		t.Error("Ping succeeded against a closed port")
	}
}
