package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/shared/dbx"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// MaxCallCount is BR-16: a number is called at most three times before
// it is skipped. The DB CHECK on antrian.call_count enforces the same
// ceiling, so a bug here fails loudly instead of silently over-calling.
const MaxCallCount = 3

// ErrNoTransition means the guarded UPDATE matched no row: the item was
// not in the state the action requires (or the recall cap is reached).
// Handlers map it to 409.
var ErrNoTransition = errors.New("illegal queue transition")

// Session is an operator's open shift at a loket.
type Session struct {
	ID       string
	LoketID  string
	UserID   string
	OpenedAt time.Time
	ClosedAt *time.Time
	IsActive bool
}

type LoketOpsRepository struct {
	db *pgxpool.Pool
}

func NewLoketOpsRepository(db *pgxpool.Pool) *LoketOpsRepository {
	return &LoketOpsRepository{db: db}
}

func (r *LoketOpsRepository) DB() *pgxpool.Pool {
	return r.db
}

// OpenSession claims a loket for an operator. The partial unique index
// (one active session per loket) is the arbiter: on conflict we return
// the session that already holds the counter so the caller can tell
// "you already have it" from "someone else has it".
func (r *LoketOpsRepository) OpenSession(ctx context.Context, loketID, userID string) (*Session, error) {
	const insert = `
		INSERT INTO mpp.loket_session (loket_id, user_id, is_active)
		VALUES ($1, $2, TRUE)
		ON CONFLICT DO NOTHING
		RETURNING id, loket_id, user_id, opened_at, closed_at, is_active`

	var s Session
	err := r.db.QueryRow(ctx, insert, loketID, userID).Scan(
		&s.ID, &s.LoketID, &s.UserID, &s.OpenedAt, &s.ClosedAt, &s.IsActive)
	if err == nil {
		return &s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		logger.Error("Failed to open loket session", logger.Err(err))
		return nil, err
	}

	return r.ActiveSession(ctx, loketID)
}

// ActiveSession returns the open shift at a loket, or (nil, nil).
func (r *LoketOpsRepository) ActiveSession(ctx context.Context, loketID string) (*Session, error) {
	const query = `
		SELECT id, loket_id, user_id, opened_at, closed_at, is_active
		FROM mpp.loket_session
		WHERE loket_id = $1 AND is_active = TRUE AND closed_at IS NULL`

	var s Session
	err := r.db.QueryRow(ctx, query, loketID).Scan(
		&s.ID, &s.LoketID, &s.UserID, &s.OpenedAt, &s.ClosedAt, &s.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to read loket session", logger.Err(err))
		return nil, err
	}

	return &s, nil
}

// CloseSession ends the operator's shift.
func (r *LoketOpsRepository) CloseSession(ctx context.Context, loketID, userID string) (*Session, error) {
	const query = `
		UPDATE mpp.loket_session
		SET is_active = FALSE, closed_at = NOW()
		WHERE loket_id = $1 AND user_id = $2 AND is_active = TRUE AND closed_at IS NULL
		RETURNING id, loket_id, user_id, opened_at, closed_at, is_active`

	var s Session
	err := r.db.QueryRow(ctx, query, loketID, userID).Scan(
		&s.ID, &s.LoketID, &s.UserID, &s.OpenedAt, &s.ClosedAt, &s.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoTransition
		}
		logger.Error("Failed to close loket session", logger.Err(err))
		return nil, err
	}

	return &s, nil
}

// CallNext takes the head of the waiting stream for the services this
// loket serves and marks it CALLED.
//
// The inner SELECT ... FOR UPDATE SKIP LOCKED is what keeps two
// operators pressing "Panggil" at the same instant from grabbing the
// same person: the second one skips the locked row and takes the next.
// Ordering is priority-then-arrival, which is FIFO for a FIFO agency
// (every row has priority 0) and booking-first for BOOKING_PRIORITY.
//
// Returns ("", nil) when the stream is empty — an empty queue is a
// normal answer, not an error.
func (r *LoketOpsRepository) CallNext(ctx context.Context, q dbx.Querier, loketID string, layananIDs []string, day time.Time) (string, error) {
	const query = `
		UPDATE mpp.antrian
		SET status = 'CALLED', loket_id = $1, called_at = NOW(),
		    call_count = 1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM mpp.antrian
			WHERE jenis_layanan_id = ANY($2::uuid[])
			  AND queue_date = $3
			  AND status = 'WAITING'
			ORDER BY priority DESC, queued_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`

	var id string
	if err := q.QueryRow(ctx, query, loketID, layananIDs, day).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		logger.Error("Failed to call next", logger.Err(err))
		return "", err
	}

	return id, nil
}

// Recall bumps the call counter, refusing the 4th attempt (BR-16).
func (r *LoketOpsRepository) Recall(ctx context.Context, id string) (int, error) {
	const query = `
		UPDATE mpp.antrian
		SET call_count = call_count + 1, called_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'CALLED' AND call_count < $2
		RETURNING call_count`

	var count int
	if err := r.db.QueryRow(ctx, query, id, MaxCallCount).Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNoTransition
		}
		logger.Error("Failed to recall", logger.Err(err))
		return 0, err
	}

	return count, nil
}

// Start moves CALLED → SERVING and opens the serving session that later
// yields the service duration.
func (r *LoketOpsRepository) Start(ctx context.Context, tx pgx.Tx, id, userID string) error {
	const update = `
		UPDATE mpp.antrian
		SET status = 'SERVING', served_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'CALLED'
		RETURNING loket_id`

	var loketID *string
	if err := tx.QueryRow(ctx, update, id).Scan(&loketID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoTransition
		}
		logger.Error("Failed to start serving", logger.Err(err))
		return err
	}
	if loketID == nil {
		return ErrNoTransition
	}

	const insert = `
		INSERT INTO mpp.serving_session (antrian_id, loket_id, user_id, started_at)
		VALUES ($1, $2, $3, NOW())`

	if _, err := tx.Exec(ctx, insert, id, *loketID, userID); err != nil {
		logger.Error("Failed to open serving session", logger.Err(err))
		return err
	}

	return nil
}

// Skip records a no-show. Allowed from CALLED (nobody came) — a SERVING
// item is finished with done, not skipped.
func (r *LoketOpsRepository) Skip(ctx context.Context, tx pgx.Tx, id string) error {
	const update = `
		UPDATE mpp.antrian
		SET status = 'SKIPPED', updated_at = NOW()
		WHERE id = $1 AND status = 'CALLED'
		RETURNING id`

	var updated string
	if err := tx.QueryRow(ctx, update, id).Scan(&updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoTransition
		}
		logger.Error("Failed to skip antrian", logger.Err(err))
		return err
	}

	return r.closeServingSession(ctx, tx, id, "SKIPPED")
}

// Done closes the service and reports how long it took.
func (r *LoketOpsRepository) Done(ctx context.Context, tx pgx.Tx, id string) (int, time.Time, error) {
	const update = `
		UPDATE mpp.antrian
		SET status = 'DONE', done_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'SERVING'
		RETURNING done_at`

	var doneAt time.Time
	if err := tx.QueryRow(ctx, update, id).Scan(&doneAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, time.Time{}, ErrNoTransition
		}
		logger.Error("Failed to finish antrian", logger.Err(err))
		return 0, time.Time{}, err
	}

	if err := r.closeServingSession(ctx, tx, id, "DONE"); err != nil {
		return 0, time.Time{}, err
	}

	// Duration comes from the serving session, not from the ticket: it
	// measures counter time, which is what the reports are about.
	const durationQuery = `
		SELECT COALESCE(EXTRACT(EPOCH FROM (ended_at - started_at)), 0)::int
		FROM mpp.serving_session
		WHERE antrian_id = $1
		ORDER BY started_at DESC
		LIMIT 1`

	var seconds int
	if err := tx.QueryRow(ctx, durationQuery, id).Scan(&seconds); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, doneAt, nil
		}
		logger.Error("Failed to compute service duration", logger.Err(err))
		return 0, doneAt, err
	}

	return seconds, doneAt, nil
}

func (r *LoketOpsRepository) closeServingSession(ctx context.Context, tx pgx.Tx, antrianID, outcome string) error {
	const query = `
		UPDATE mpp.serving_session
		SET ended_at = NOW(), outcome = $2::mpp.serving_outcome
		WHERE antrian_id = $1 AND ended_at IS NULL`

	if _, err := tx.Exec(ctx, query, antrianID, outcome); err != nil {
		logger.Error("Failed to close serving session", logger.Err(err))
		return err
	}

	return nil
}
