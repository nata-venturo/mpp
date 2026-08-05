package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/domain"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// ErrQuotaFull means no seat could be reserved for the requested
// (agency, service, date) — either the row is exhausted or no quota was
// configured at all. Both are "full" from the citizen's point of view.
var ErrQuotaFull = errors.New("quota full")

type KuotaRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewKuotaRepository(db *pgxpool.Pool, companyID string) *KuotaRepository {
	return &KuotaRepository{db: db, companyID: companyID}
}

// FindSlot resolves the quota row that governs a booking: the per-service
// row when one exists, otherwise the agency-wide row. Returns (nil, nil)
// when neither is configured — the caller renders that as remaining 0.
func (r *KuotaRepository) FindSlot(ctx context.Context, instansiID string, layananID *string, date time.Time) (*domain.Slot, error) {
	if layananID != nil && *layananID != "" {
		slot, err := r.findOne(ctx, instansiID, layananID, date)
		if err != nil || slot != nil {
			return slot, err
		}
	}

	return r.findOne(ctx, instansiID, nil, date)
}

func (r *KuotaRepository) findOne(ctx context.Context, instansiID string, layananID *string, date time.Time) (*domain.Slot, error) {
	// The join pins the row to this building's tenant — quota is read on a
	// public endpoint, so it must never leak another company's agency.
	query := `
		SELECT k.id, k.instansi_id, k.jenis_layanan_id, k.tanggal, k.kuota, k.terpakai
		FROM mpp.kuota_booking k
		JOIN mpp.instansi i ON i.id = k.instansi_id AND i.deleted_at IS NULL
		WHERE k.instansi_id = $1 AND i.company_id = $2 AND k.tanggal = $3
		  AND ( ($4::uuid IS NULL AND k.jenis_layanan_id IS NULL)
		     OR ($4::uuid IS NOT NULL AND k.jenis_layanan_id = $4::uuid) )`

	var s domain.Slot
	err := r.db.QueryRow(ctx, query, instansiID, r.companyID, date, layananID).Scan(
		&s.ID, &s.InstansiID, &s.JenisLayananID, &s.Tanggal, &s.Kuota, &s.Terpakai,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to read quota slot", logger.Err(err))
		return nil, err
	}

	return &s, nil
}

// consumeSQL reserves one seat. The `terpakai < kuota` guard plus the row
// lock Postgres takes for the UPDATE is what makes concurrent bookings
// safe: a loser sees the winner's committed value and matches 0 rows.
const consumeSQL = `
	UPDATE mpp.kuota_booking
	SET terpakai = terpakai + 1, updated_at = NOW()
	WHERE instansi_id = $1
	  AND tanggal = $2
	  AND ( ($3::uuid IS NULL AND jenis_layanan_id IS NULL)
	     OR ($3::uuid IS NOT NULL AND jenis_layanan_id = $3::uuid) )
	  AND terpakai < kuota
	RETURNING id`

// Consume reserves one seat inside the caller's transaction.
//
// Precedence mirrors FindSlot: a per-service row wins when it exists, and
// an EXISTING but exhausted per-service row is final — it must not spill
// over onto the agency-wide pool. Only a missing per-service row falls
// back. Returns ErrQuotaFull when nothing could be reserved.
func (r *KuotaRepository) Consume(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error {
	if layananID != nil && *layananID != "" {
		var id string
		err := tx.QueryRow(ctx, consumeSQL, instansiID, date, layananID).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			logger.Error("Failed to consume service quota", logger.Err(err))
			return err
		}

		exists, err := r.slotExists(ctx, tx, instansiID, layananID, date)
		if err != nil {
			return err
		}
		if exists {
			return ErrQuotaFull
		}
	}

	var id string
	err := tx.QueryRow(ctx, consumeSQL, instansiID, date, nil).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrQuotaFull
		}
		logger.Error("Failed to consume agency quota", logger.Err(err))
		return err
	}

	return nil
}

// Release gives a seat back (booking cancelled / expired — BR-07). Exact
// inverse of Consume, including the per-service precedence.
func (r *KuotaRepository) Release(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error {
	const releaseSQL = `
		UPDATE mpp.kuota_booking
		SET terpakai = terpakai - 1, updated_at = NOW()
		WHERE instansi_id = $1
		  AND tanggal = $2
		  AND ( ($3::uuid IS NULL AND jenis_layanan_id IS NULL)
		     OR ($3::uuid IS NOT NULL AND jenis_layanan_id = $3::uuid) )
		  AND terpakai > 0
		RETURNING id`

	if layananID != nil && *layananID != "" {
		var id string
		err := tx.QueryRow(ctx, releaseSQL, instansiID, date, layananID).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			logger.Error("Failed to release service quota", logger.Err(err))
			return err
		}

		exists, err := r.slotExists(ctx, tx, instansiID, layananID, date)
		if err != nil {
			return err
		}
		if exists {
			// Row exists but is already at zero — nothing to give back.
			return nil
		}
	}

	var id string
	err := tx.QueryRow(ctx, releaseSQL, instansiID, date, nil).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		logger.Error("Failed to release agency quota", logger.Err(err))
		return err
	}

	return nil
}

func (r *KuotaRepository) slotExists(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM mpp.kuota_booking
			WHERE instansi_id = $1 AND tanggal = $2
			  AND ( ($3::uuid IS NULL AND jenis_layanan_id IS NULL)
			     OR ($3::uuid IS NOT NULL AND jenis_layanan_id = $3::uuid) )
		)`

	var exists bool
	if err := tx.QueryRow(ctx, query, instansiID, date, layananID).Scan(&exists); err != nil {
		logger.Error("Failed to check quota slot", logger.Err(err))
		return false, err
	}

	return exists, nil
}
