// Package repository is the writer of the runtime schema's passports, runs, jobs and audit
// events. Every query is schema-qualified and scoped by the verified organization; a record id
// is a reference, never authorization. Transactions are short and never span a model or tool
// request. Stored actions and approvals belong to the gate (internal/policy).
package repository

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrInvalid rejects an unusable argument before any database work.
	ErrInvalid = errors.New("invalid runtime repository operation")
	// ErrNotFound means no row exists for the organization and id; it does not reveal whether the
	// id exists in another organization.
	ErrNotFound = errors.New("runtime record not found")
	// ErrInvalidTransition means the requested state change is not allowed from the stored state.
	ErrInvalidTransition = errors.New("invalid runtime state transition")
	// ErrUnavailable hides driver errors, which can name hosts and users.
	ErrUnavailable = errors.New("runtime storage unavailable")
)

// Beginner starts a transaction: a pool, or an enclosing transaction (a savepoint).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository writes and reads runtime records.
type Repository struct {
	database Beginner
}

// New returns a repository over the gateway pool (or an enclosing transaction in tests).
func New(database Beginner) *Repository {
	return &Repository{database: database}
}

// Tx is one open runtime transaction. Its methods share the transaction, so a state change
// and the event it produces commit together or not at all.
type Tx struct {
	transaction pgx.Tx
}

// Raw exposes the transaction to a component that must join it, for example a local demo
// effect that commits with its execution record (GO-34). The caller must not commit it.
func (tx Tx) Raw() pgx.Tx {
	return tx.transaction
}

// InTransaction runs work in one transaction: it commits when work returns nil and rolls back
// otherwise. Errors from work are returned unchanged; storage errors become ErrUnavailable.
func (repository *Repository) InTransaction(ctx context.Context, work func(Tx) error) error {
	if repository == nil || repository.database == nil {
		return ErrUnavailable
	}
	if ctx == nil || work == nil {
		return ErrInvalid
	}
	transaction, err := repository.database.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	// Rollback after a successful commit is a no-op.
	defer func() { _ = transaction.Rollback(context.WithoutCancel(ctx)) }()
	if err := work(Tx{transaction: transaction}); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validUUID accepts the lowercase form PostgreSQL returns for uuid columns.
func validUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

// storageError maps a driver error to a safe repository error.
func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return ErrUnavailable
}
