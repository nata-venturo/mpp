package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/domain"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// ErrDuplicateToken signals a collision on booking.qr_token. It is
// astronomically unlikely (128 bits of entropy); the caller regenerates
// once rather than failing the citizen's booking.
var ErrDuplicateToken = errors.New("duplicate qr token")

type BookingRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewBookingRepository(db *pgxpool.Pool, companyID string) *BookingRepository {
	return &BookingRepository{db: db, companyID: companyID}
}

// DB exposes the pool so services can open their own transaction.
func (r *BookingRepository) DB() *pgxpool.Pool {
	return r.db
}

// UpsertPemohon dedupes an applicant by phone — a citizen who books
// twice is one person, not two rows. Without a phone we always insert:
// there is nothing to match on.
func (r *BookingRepository) UpsertPemohon(ctx context.Context, tx pgx.Tx, p *domain.Pemohon) (string, error) {
	if p.Phone != nil && *p.Phone != "" {
		const findQuery = `
			SELECT id FROM mpp.pemohon
			WHERE phone = $1
			ORDER BY created_at ASC
			LIMIT 1`

		var id string
		err := tx.QueryRow(ctx, findQuery, *p.Phone).Scan(&id)
		switch {
		case err == nil:
			// Refresh the contact details from the newest submission.
			const updateQuery = `
				UPDATE mpp.pemohon
				SET name = $2,
				    email = COALESCE($3, email),
				    nik_hash = COALESCE($4, nik_hash),
				    updated_at = NOW()
				WHERE id = $1`
			if _, err := tx.Exec(ctx, updateQuery, id, p.Name, p.Email, p.NIKHash); err != nil {
				logger.Error("Failed to refresh pemohon", logger.Err(err))
				return "", err
			}
			return id, nil
		case !errors.Is(err, pgx.ErrNoRows):
			logger.Error("Failed to look up pemohon", logger.Err(err))
			return "", err
		}
	}

	const insertQuery = `
		INSERT INTO mpp.pemohon (name, phone, email, nik_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	var id string
	if err := tx.QueryRow(ctx, insertQuery, p.Name, p.Phone, p.Email, p.NIKHash).Scan(&id); err != nil {
		logger.Error("Failed to create pemohon", logger.Err(err))
		return "", err
	}

	return id, nil
}

// Create inserts the booking row. This is the ONLY place qr_token is
// written — keeping the single writer means swapping the raw token for a
// hash later is a one-line change here plus one in FindByToken.
//
// ponytail: the token is stored raw so check-in stays a single indexed
// lookup. Hash it (and hash the probe in FindByToken) if the DB ever
// becomes a lower-trust store than the app.
func (r *BookingRepository) Create(ctx context.Context, tx pgx.Tx, b *domain.Booking) error {
	const query = `
		INSERT INTO mpp.booking
			(pemohon_id, instansi_id, jenis_layanan_id, tanggal, channel,
			 qr_token, qr_expires_at, status)
		VALUES ($1, $2, $3, $4, $5::mpp.booking_channel, $6, $7, 'BOOKED')
		RETURNING id, created_at, updated_at`

	err := tx.QueryRow(ctx, query,
		b.PemohonID, b.InstansiID, b.JenisLayananID, b.Tanggal, b.Channel,
		b.QRToken, b.QRExpiresAt,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "booking_qr_token_key") {
			return ErrDuplicateToken
		}
		logger.Error("Failed to create booking", logger.Err(err))
		return err
	}

	b.Status = "BOOKED"
	return nil
}

// isUniqueViolation reports whether err is a Postgres 23505 raised by a
// specific constraint — so a token collision can be retried without
// swallowing every other integrity error.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

const detailColumns = `
	b.id, b.pemohon_id, b.instansi_id, b.jenis_layanan_id, b.tanggal, b.channel,
	b.qr_token, b.qr_expires_at, b.status, b.checked_in_at, b.created_at, b.updated_at,
	p.name, p.phone, i.name, i.prefix, l.name`

const detailFrom = `
	FROM mpp.booking b
	JOIN mpp.pemohon p ON p.id = b.pemohon_id
	JOIN mpp.instansi i ON i.id = b.instansi_id AND i.deleted_at IS NULL
	JOIN mpp.jenis_layanan l ON l.id = b.jenis_layanan_id`

func scanDetail(row pgx.Row) (*domain.Detail, error) {
	var d domain.Detail
	err := row.Scan(
		&d.ID, &d.PemohonID, &d.InstansiID, &d.JenisLayananID, &d.Tanggal, &d.Channel,
		&d.QRToken, &d.QRExpiresAt, &d.Status, &d.CheckedInAt, &d.CreatedAt, &d.UpdatedAt,
		&d.PemohonName, &d.PemohonPhone, &d.InstansiName, &d.InstansiPrefix, &d.LayananName,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// FindByID returns the booking with the catalog names attached, or
// (nil, nil) when it is missing or outside this tenant.
func (r *BookingRepository) FindByID(ctx context.Context, id string) (*domain.Detail, error) {
	query := `SELECT ` + detailColumns + detailFrom + `
		WHERE b.id = $1 AND i.company_id = $2 AND b.deleted_at IS NULL`

	d, err := scanDetail(r.db.QueryRow(ctx, query, id, r.companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find booking", logger.Err(err))
		return nil, err
	}

	return d, nil
}

// FindByToken resolves a QR token to its booking. Paired with Create as
// the only two places the token column is touched.
func (r *BookingRepository) FindByToken(ctx context.Context, token string) (*domain.Detail, error) {
	query := `SELECT ` + detailColumns + detailFrom + `
		WHERE b.qr_token = $1 AND i.company_id = $2 AND b.deleted_at IS NULL`

	d, err := scanDetail(r.db.QueryRow(ctx, query, token, r.companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find booking by token", logger.Err(err))
		return nil, err
	}

	return d, nil
}

// MarkCheckedIn flips BOOKED → CHECKED_IN. The status guard is what makes
// a reused (or concurrently double-scanned) token a 0-row result instead
// of a second check-in.
func (r *BookingRepository) MarkCheckedIn(ctx context.Context, tx pgx.Tx, bookingID string) (bool, error) {
	const query = `
		UPDATE mpp.booking
		SET status = 'CHECKED_IN', checked_in_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'BOOKED' AND deleted_at IS NULL
		RETURNING id`

	var id string
	err := tx.QueryRow(ctx, query, bookingID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		logger.Error("Failed to mark booking checked in", logger.Err(err))
		return false, err
	}

	return true, nil
}
