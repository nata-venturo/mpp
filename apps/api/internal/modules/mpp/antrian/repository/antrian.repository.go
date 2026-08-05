package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/domain"
	"github.com/ndollem/mpp/apps/api/internal/shared/dbx"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// ErrDuplicateSeq means the DB backstop caught a sequence that Redis
// handed out twice (only possible after a counter loss). The caller
// re-allocates instead of failing the citizen.
var ErrDuplicateSeq = errors.New("duplicate queue sequence")

// counterTTL outlives one operating day with room for late closing, so
// stale counters expire on their own even if the daily reset never runs.
const counterTTL = 36 * time.Hour

type AntrianRepository struct {
	db    *pgxpool.Pool
	redis *goredis.Client
}

func NewAntrianRepository(db *pgxpool.Pool, redis *goredis.Client) *AntrianRepository {
	return &AntrianRepository{db: db, redis: redis}
}

func (r *AntrianRepository) DB() *pgxpool.Pool {
	return r.db
}

// counterKey is per agency per operating day — queue numbers carry the
// agency prefix (A-014), so the sequence is shared by all of that
// agency's services, never re-keyed per service.
func counterKey(instansiID string, day time.Time) string {
	return fmt.Sprintf("mpp:counter:%s:%s", instansiID, day.Format("20060102"))
}

// NextSeq allocates the next queue number atomically.
//
// Redis INCR is the authority; the DB unique index is the backstop. On a
// cold key (first ticket of the day, or a flushed/restarted Redis) the
// counter is seeded from MAX(nomor_seq) already in Postgres, so a
// mid-day Redis loss resumes at 6 rather than restarting at 1. SETNX
// makes that seeding safe when several requests race into it.
//
// `q` is the caller's transaction: the cold-start probe reads Postgres,
// and doing that on the pool while a transaction is open would burn a
// second connection (see dbx.Querier).
func (r *AntrianRepository) NextSeq(ctx context.Context, q dbx.Querier, instansiID string, day time.Time) (int, error) {
	key := counterKey(instansiID, day)

	exists, err := r.redis.Exists(ctx, key).Result()
	if err != nil {
		logger.Error("Failed to probe queue counter", logger.Err(err))
		return 0, err
	}

	if exists == 0 {
		maxSeq, err := r.MaxSeq(ctx, q, instansiID, day)
		if err != nil {
			return 0, err
		}
		if err := r.redis.SetNX(ctx, key, maxSeq, counterTTL).Err(); err != nil {
			logger.Error("Failed to seed queue counter", logger.Err(err))
			return 0, err
		}
	}

	seq, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		logger.Error("Failed to increment queue counter", logger.Err(err))
		return 0, err
	}

	// Re-arm the TTL: an INCR on a key that was created by an earlier
	// path (or whose expiry was dropped) would otherwise live forever.
	if err := r.redis.Expire(ctx, key, counterTTL).Err(); err != nil {
		logger.Warn("Failed to refresh queue counter TTL", logger.Err(err))
	}

	return int(seq), nil
}

// MaxSeq is the highest number already issued by an agency on a day.
func (r *AntrianRepository) MaxSeq(ctx context.Context, q dbx.Querier, instansiID string, day time.Time) (int, error) {
	const query = `
		SELECT COALESCE(MAX(nomor_seq), 0)
		FROM mpp.antrian
		WHERE instansi_id = $1 AND queue_date = $2`

	var maxSeq int
	if err := q.QueryRow(ctx, query, instansiID, day).Scan(&maxSeq); err != nil {
		logger.Error("Failed to read max queue sequence", logger.Err(err))
		return 0, err
	}

	return maxSeq, nil
}

// Create inserts a WAITING ticket. A unique violation on
// (instansi, queue_date, nomor_seq) surfaces as ErrDuplicateSeq.
func (r *AntrianRepository) Create(ctx context.Context, tx pgx.Tx, a *domain.Antrian) error {
	const query = `
		INSERT INTO mpp.antrian
			(booking_id, pemohon_id, instansi_id, jenis_layanan_id,
			 nomor, nomor_seq, queue_date, source, status, priority, queued_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::mpp.antrian_source, 'WAITING', $9, NOW())
		RETURNING id, queued_at`

	err := tx.QueryRow(ctx, query,
		a.BookingID, a.PemohonID, a.InstansiID, a.JenisLayananID,
		a.Nomor, a.NomorSeq, a.QueueDate, a.Source, a.Priority,
	).Scan(&a.ID, &a.QueuedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "antrian_instansi_day_seq_key" {
			return ErrDuplicateSeq
		}
		logger.Error("Failed to create antrian", logger.Err(err))
		return err
	}

	a.Status = domain.StatusWaiting
	return nil
}

const itemColumns = `
	a.id, a.booking_id, a.pemohon_id, a.instansi_id, a.jenis_layanan_id,
	a.nomor, a.nomor_seq, a.queue_date, a.source, a.status, a.loket_id,
	a.call_count, a.priority, a.queued_at, a.called_at, a.served_at, a.done_at,
	p.name, l.name, lk.name`

const itemFrom = `
	FROM mpp.antrian a
	JOIN mpp.pemohon p ON p.id = a.pemohon_id
	JOIN mpp.jenis_layanan l ON l.id = a.jenis_layanan_id
	JOIN mpp.instansi i ON i.id = a.instansi_id AND i.deleted_at IS NULL
	LEFT JOIN mpp.loket lk ON lk.id = a.loket_id`

func scanItem(row pgx.Row) (*domain.QueueItem, error) {
	var it domain.QueueItem
	err := row.Scan(
		&it.ID, &it.BookingID, &it.PemohonID, &it.InstansiID, &it.JenisLayananID,
		&it.Nomor, &it.NomorSeq, &it.QueueDate, &it.Source, &it.Status, &it.LoketID,
		&it.CallCount, &it.Priority, &it.QueuedAt, &it.CalledAt, &it.ServedAt, &it.DoneAt,
		&it.PemohonName, &it.LayananName, &it.LoketName,
	)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// FindByID returns one ticket inside the tenant, or (nil, nil).
func (r *AntrianRepository) FindByID(ctx context.Context, companyID, id string) (*domain.QueueItem, error) {
	query := `SELECT ` + itemColumns + itemFrom + `
		WHERE a.id = $1 AND i.company_id = $2`

	it, err := scanItem(r.db.QueryRow(ctx, query, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find antrian", logger.Err(err))
		return nil, err
	}

	return it, nil
}

// ListWaiting returns today's waiting stream for one service in call
// order: priority first (BOOKING_PRIORITY mode), then arrival.
func (r *AntrianRepository) ListWaiting(ctx context.Context, companyID, layananID string, day time.Time, page, limit int) ([]domain.QueueItem, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM mpp.antrian a
		JOIN mpp.instansi i ON i.id = a.instansi_id AND i.deleted_at IS NULL
		WHERE a.jenis_layanan_id = $1 AND i.company_id = $2
		  AND a.queue_date = $3 AND a.status = 'WAITING'`

	var total int64
	if err := r.db.QueryRow(ctx, countQuery, layananID, companyID, day).Scan(&total); err != nil {
		logger.Error("Failed to count waiting queue", logger.Err(err))
		return nil, 0, err
	}

	query := `SELECT ` + itemColumns + itemFrom + `
		WHERE a.jenis_layanan_id = $1 AND i.company_id = $2
		  AND a.queue_date = $3 AND a.status = 'WAITING'
		ORDER BY a.priority DESC, a.queued_at ASC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(ctx, query, layananID, companyID, day, limit, (page-1)*limit)
	if err != nil {
		logger.Error("Failed to list waiting queue", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]domain.QueueItem, 0)
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *it)
	}

	return list, total, rows.Err()
}

// CountAhead counts the tickets that will be served before this one —
// the `position` term of the ETA formula (BR-29). CALLED items count:
// they still occupy a counter.
func (r *AntrianRepository) CountAhead(ctx context.Context, layananID string, day time.Time, priority int, queuedAt time.Time) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM mpp.antrian
		WHERE jenis_layanan_id = $1 AND queue_date = $2
		  AND status IN ('WAITING', 'CALLED')
		  AND ( priority > $3 OR (priority = $3 AND queued_at < $4) )`

	var n int
	if err := r.db.QueryRow(ctx, query, layananID, day, priority, queuedAt).Scan(&n); err != nil {
		logger.Error("Failed to count queue position", logger.Err(err))
		return 0, err
	}

	return n, nil
}
