// Package testdb provides explicit PostgreSQL integration-test connections.
// It never creates schemas, runs migrations or supplies a fallback database.
package testdb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/logging"
)

// connectionTimeout covers connecting under load: pnpm test:db runs every package in parallel
// against one database, and a short timeout made unrelated tests fail as "unavailable".
const connectionTimeout = 15 * time.Second

type lookupFunc func(string) (string, bool)

// Open skips only wholly unconfigured optional database tests. Partial settings
// and any unavailable explicitly configured database fail the test.
func Open(test testing.TB) *pgxpool.Pool {
	test.Helper()
	options, configured, err := readOptions(os.LookupEnv)
	if err != nil {
		test.Fatal(err)
		return nil
	}
	if !configured {
		test.Skip("PostgreSQL integration test skipped: POSTGRES_* not configured")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	pool, err := connect(ctx, options)
	if err != nil {
		test.Fatal(err)
		return nil
	}
	test.Cleanup(pool.Close)
	return pool
}

// ID returns a cryptographically random UUID v4 for isolated fixture records.
func ID(test testing.TB) string {
	test.Helper()
	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		test.Fatal("generate test database identifier")
		return ""
	}
	identifier[6] = (identifier[6] & 0x0f) | 0x40
	identifier[8] = (identifier[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", identifier[0:4], identifier[4:6], identifier[6:8], identifier[8:10], identifier[10:16])
}

func readOptions(lookup lookupFunc) (database.Options, bool, error) {
	options := database.Options{Host: "localhost", Port: 5432, ConnectTimeout: connectionTimeout}
	values := make(map[string]string)
	present := make(map[string]bool)
	configured := false
	for _, name := range []string{"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
		values[name], present[name] = lookup(name)
		configured = configured || present[name]
	}
	required, _ := lookup("TEST_DATABASE_REQUIRED")
	configured = configured || required == "1"
	if !configured {
		return options, false, nil
	}
	var problems []string
	for _, name := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
		if !present[name] || strings.TrimSpace(values[name]) == "" {
			problems = append(problems, name)
		}
	}
	if present["POSTGRES_HOST"] {
		if strings.TrimSpace(values["POSTGRES_HOST"]) == "" {
			problems = append(problems, "POSTGRES_HOST")
		} else {
			options.Host = strings.TrimSpace(values["POSTGRES_HOST"])
		}
	}
	if present["POSTGRES_PORT"] {
		raw := values["POSTGRES_PORT"]
		digits := raw != ""
		for _, character := range raw {
			digits = digits && character >= '0' && character <= '9'
		}
		port, err := strconv.Atoi(raw)
		if !digits || err != nil || port < 1 || port > 65535 {
			problems = append(problems, "POSTGRES_PORT")
		} else {
			options.Port = port
		}
	}
	if len(problems) > 0 {
		return database.Options{}, true, errors.New("invalid test database configuration: " + strings.Join(problems, ", "))
	}
	options.User = values["POSTGRES_USER"]
	options.Password = logging.NewSecret(values["POSTGRES_PASSWORD"])
	options.Database = values["POSTGRES_DB"]
	return options, true, nil
}

func connect(ctx context.Context, options database.Options) (*pgxpool.Pool, error) {
	options.ConnectTimeout = connectionTimeout
	pool, err := database.NewPool(ctx, options)
	if err != nil {
		return nil, errors.New("initialize test database connection from POSTGRES_* variables")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("test database unavailable; check POSTGRES_* variables")
	}
	return pool, nil
}
