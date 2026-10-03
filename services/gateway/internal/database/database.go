// Package database owns the PostgreSQL connection pool.
package database

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/logging"
)

const (
	maxConnections        = 10
	maxConnectionLifetime = 30 * time.Minute
	maxConnectionIdleTime = 5 * time.Minute
	healthCheckPeriod     = 30 * time.Second
)

// Options are the discrete connection settings (no connection string).
type Options struct {
	Host           string
	Port           int
	User           string
	Password       logging.Secret
	Database       string
	ConnectTimeout time.Duration
}

// NewPool builds a pool without connecting: the first Ping or query dials.
// The service can therefore start while PostgreSQL is down.
func NewPool(ctx context.Context, options Options) (*pgxpool.Pool, error) {
	poolConfig, err := buildPoolConfig(options)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, poolConfig)
}

func buildPoolConfig(options Options) (*pgxpool.Config, error) {
	// net/url escapes special characters in the user, password and database name.
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(options.User, options.Password.Reveal()),
		Host:   net.JoinHostPort(options.Host, strconv.Itoa(options.Port)),
		Path:   "/" + options.Database,
	}
	poolConfig, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {
		// The parse error is dropped on purpose: it could echo connection details.
		return nil, errors.New("build database pool configuration from POSTGRES_* variables")
	}

	// MinConns 0 keeps startup lazy and lets Close return immediately.
	poolConfig.MinConns = 0
	poolConfig.MinIdleConns = 0
	poolConfig.MaxConns = maxConnections
	poolConfig.MaxConnLifetime = maxConnectionLifetime
	poolConfig.MaxConnIdleTime = maxConnectionIdleTime
	poolConfig.HealthCheckPeriod = healthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = options.ConnectTimeout
	return poolConfig, nil
}
