// Package dbx holds the tiny database seam shared by the MPP modules.
package dbx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
//
// It exists so a repository read can run on the CALLER'S transaction.
// Reaching for the pool while a transaction is open checks out a second
// connection; once concurrent transactions outnumber the pool, every one
// of them waits for a connection none of them will release — a deadlock
// that only appears under load. Any read that happens inside a
// transaction must therefore take a Querier and be handed the tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
