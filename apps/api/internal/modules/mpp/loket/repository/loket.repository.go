package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/domain"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

type LoketRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewLoketRepository(db *pgxpool.Pool, companyID string) *LoketRepository {
	return &LoketRepository{db: db, companyID: companyID}
}

// FindByInstansi lists an agency's active lokets ordered by code.
func (r *LoketRepository) FindByInstansi(ctx context.Context, instansiID string) ([]domain.Loket, error) {
	query := `
		SELECT l.id, l.instansi_id, l.code, l.name, l.status, l.last_idle_at, l.is_active
		FROM mpp.loket l
		JOIN mpp.instansi i ON i.id = l.instansi_id AND i.deleted_at IS NULL
		WHERE l.instansi_id = $1 AND i.company_id = $2
		  AND l.deleted_at IS NULL AND l.is_active = TRUE
		ORDER BY l.code ASC`

	rows, err := r.db.Query(ctx, query, instansiID, r.companyID)
	if err != nil {
		logger.Error("Failed to list loket", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Loket, 0)
	for rows.Next() {
		var l domain.Loket
		if err := rows.Scan(&l.ID, &l.InstansiID, &l.Code, &l.Name, &l.Status, &l.LastIdleAt, &l.IsActive); err != nil {
			return nil, err
		}
		list = append(list, l)
	}

	return list, rows.Err()
}

// FindByID returns one loket inside the tenant, or (nil, nil).
func (r *LoketRepository) FindByID(ctx context.Context, id string) (*domain.Loket, error) {
	query := `
		SELECT l.id, l.instansi_id, l.code, l.name, l.status, l.last_idle_at, l.is_active
		FROM mpp.loket l
		JOIN mpp.instansi i ON i.id = l.instansi_id AND i.deleted_at IS NULL
		WHERE l.id = $1 AND i.company_id = $2 AND l.deleted_at IS NULL`

	var l domain.Loket
	err := r.db.QueryRow(ctx, query, id, r.companyID).Scan(
		&l.ID, &l.InstansiID, &l.Code, &l.Name, &l.Status, &l.LastIdleAt, &l.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find loket", logger.Err(err))
		return nil, err
	}

	return &l, nil
}

// ServedLayananIDs returns the services this loket may call from.
func (r *LoketRepository) ServedLayananIDs(ctx context.Context, loketID string) ([]string, error) {
	query := `
		SELECT ll.jenis_layanan_id
		FROM mpp.loket_layanan ll
		JOIN mpp.jenis_layanan l ON l.id = ll.jenis_layanan_id
		WHERE ll.loket_id = $1 AND l.deleted_at IS NULL AND l.is_active = TRUE`

	rows, err := r.db.Query(ctx, query, loketID)
	if err != nil {
		logger.Error("Failed to list loket services", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// CountOpenForLayanan counts OPEN lokets eligible for a service — the
// n_loket term of the ETA formula (BR-29).
func (r *LoketRepository) CountOpenForLayanan(ctx context.Context, layananID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM mpp.loket l
		JOIN mpp.loket_layanan ll ON ll.loket_id = l.id
		WHERE ll.jenis_layanan_id = $1
		  AND l.status = 'OPEN' AND l.is_active = TRUE AND l.deleted_at IS NULL`

	var n int
	if err := r.db.QueryRow(ctx, query, layananID).Scan(&n); err != nil {
		logger.Error("Failed to count open lokets", logger.Err(err))
		return 0, err
	}

	return n, nil
}

// TouchIdle refreshes last_idle_at so the idle-longest ordering (BR-12)
// stays fair. Called inside the transaction that frees the loket.
func (r *LoketRepository) TouchIdle(ctx context.Context, tx pgx.Tx, loketID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE mpp.loket SET last_idle_at = NOW(), updated_at = NOW() WHERE id = $1`, loketID)
	if err != nil {
		logger.Error("Failed to refresh loket idle", logger.Err(err))
	}
	return err
}
