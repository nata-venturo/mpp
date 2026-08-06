package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// Current is one counter's active call — what the TV shows in big type.
type Current struct {
	AntrianID string
	Nomor     string
	Status    string
	LoketID   string
	LoketName string
	CalledAt  *time.Time
}

// Upcoming is one of the next numbers in line.
type Upcoming struct {
	AntrianID   string
	Nomor       string
	LayananName string
}

type DisplayRepository struct {
	db *pgxpool.Pool
}

func NewDisplayRepository(db *pgxpool.Pool) *DisplayRepository {
	return &DisplayRepository{db: db}
}

// CurrentCalls returns the live call per counter, newest first.
//
// DISTINCT ON keeps exactly one row per loket: a counter that called,
// then started serving, then called again must appear once, showing its
// latest activity — not once per historical call.
func (r *DisplayRepository) CurrentCalls(ctx context.Context, instansiID string, day time.Time) ([]Current, error) {
	const query = `
		SELECT DISTINCT ON (a.loket_id)
		       a.id, a.nomor, a.status::text, a.loket_id,
		       COALESCE(l.name, l.code), a.called_at
		FROM mpp.antrian a
		JOIN mpp.loket l ON l.id = a.loket_id
		WHERE a.instansi_id = $1 AND a.queue_date = $2
		  AND a.status IN ('CALLED', 'SERVING')
		ORDER BY a.loket_id, a.called_at DESC NULLS LAST`

	rows, err := r.db.Query(ctx, query, instansiID, day)
	if err != nil {
		logger.Error("Failed to read display current calls", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]Current, 0)
	for rows.Next() {
		var c Current
		if err := rows.Scan(&c.AntrianID, &c.Nomor, &c.Status, &c.LoketID, &c.LoketName, &c.CalledAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}

	return list, rows.Err()
}

// NextUp returns the head of the agency's waiting queue across all its
// services, in call order.
func (r *DisplayRepository) NextUp(ctx context.Context, instansiID string, day time.Time, limit int) ([]Upcoming, error) {
	const query = `
		SELECT a.id, a.nomor, l.name
		FROM mpp.antrian a
		JOIN mpp.jenis_layanan l ON l.id = a.jenis_layanan_id
		WHERE a.instansi_id = $1 AND a.queue_date = $2 AND a.status = 'WAITING'
		ORDER BY a.priority DESC, a.queued_at ASC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, instansiID, day, limit)
	if err != nil {
		logger.Error("Failed to read display next-up", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]Upcoming, 0)
	for rows.Next() {
		var u Upcoming
		if err := rows.Scan(&u.AntrianID, &u.Nomor, &u.LayananName); err != nil {
			return nil, err
		}
		list = append(list, u)
	}

	return list, rows.Err()
}
