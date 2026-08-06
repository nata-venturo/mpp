package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/domain"
	"github.com/ndollem/mpp/apps/api/internal/shared/dbx"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// InstansiRepository reads the MPP catalog. Every query is pinned to the
// building's company_id — the tenant boundary for the whole domain.
type InstansiRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewInstansiRepository(db *pgxpool.Pool, companyID string) *InstansiRepository {
	return &InstansiRepository{db: db, companyID: companyID}
}

const instansiColumns = `
	id, company_id, name, slug, prefix, description, logo_url,
	operating_hours, queue_mode, is_active, created_at, updated_at`

func scanInstansi(row pgx.Row) (*domain.Instansi, error) {
	var i domain.Instansi
	err := row.Scan(
		&i.ID, &i.CompanyID, &i.Name, &i.Slug, &i.Prefix, &i.Description, &i.LogoURL,
		&i.OperatingHours, &i.QueueMode, &i.IsActive, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// FindAll lists active agencies of the tenant, ordered by name.
func (r *InstansiRepository) FindAll(ctx context.Context) ([]domain.Instansi, error) {
	query := `SELECT ` + instansiColumns + `
		FROM mpp.instansi
		WHERE company_id = $1 AND deleted_at IS NULL AND is_active = TRUE
		ORDER BY name ASC`

	rows, err := r.db.Query(ctx, query, r.companyID)
	if err != nil {
		logger.Error("Failed to list instansi", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Instansi, 0)
	for rows.Next() {
		i, err := scanInstansi(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *i)
	}

	return list, rows.Err()
}

// FindByID returns one agency, or (nil, nil) when it does not exist
// inside this tenant.
func (r *InstansiRepository) FindByID(ctx context.Context, id string) (*domain.Instansi, error) {
	query := `SELECT ` + instansiColumns + `
		FROM mpp.instansi
		WHERE id = $1 AND company_id = $2 AND deleted_at IS NULL`

	i, err := scanInstansi(r.db.QueryRow(ctx, query, id, r.companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find instansi", logger.Err(err))
		return nil, err
	}

	return i, nil
}

// FindLayananByInstansi returns the agency's active services with their
// document requirements attached (one extra query, not N).
func (r *InstansiRepository) FindLayananByInstansi(ctx context.Context, instansiID string) ([]domain.Layanan, error) {
	query := `
		SELECT l.id, l.instansi_id, l.name, l.description, l.estimasi_durasi_menit,
		       l.requires_fo_verification, l.is_active
		FROM mpp.jenis_layanan l
		JOIN mpp.instansi i ON i.id = l.instansi_id AND i.deleted_at IS NULL
		WHERE l.instansi_id = $1 AND i.company_id = $2
		  AND l.deleted_at IS NULL AND l.is_active = TRUE
		ORDER BY l.name ASC`

	rows, err := r.db.Query(ctx, query, instansiID, r.companyID)
	if err != nil {
		logger.Error("Failed to list layanan", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Layanan, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var l domain.Layanan
		if err := rows.Scan(
			&l.ID, &l.InstansiID, &l.Name, &l.Description, &l.EstimasiDurasiMenit,
			&l.RequiresFOVerification, &l.IsActive,
		); err != nil {
			return nil, err
		}
		l.Syarat = make([]domain.SyaratDokumen, 0)
		list = append(list, l)
		ids = append(ids, l.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return list, nil
	}

	syarat, err := r.findSyarat(ctx, ids)
	if err != nil {
		return nil, err
	}
	for idx := range list {
		if s, ok := syarat[list[idx].ID]; ok {
			list[idx].Syarat = s
		}
	}

	return list, nil
}

func (r *InstansiRepository) findSyarat(ctx context.Context, layananIDs []string) (map[string][]domain.SyaratDokumen, error) {
	query := `
		SELECT id, jenis_layanan_id, name, is_required, notes, sort
		FROM mpp.syarat_dokumen
		WHERE jenis_layanan_id = ANY($1::uuid[])
		ORDER BY sort ASC, name ASC`

	rows, err := r.db.Query(ctx, query, layananIDs)
	if err != nil {
		logger.Error("Failed to list syarat dokumen", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.SyaratDokumen)
	for rows.Next() {
		var s domain.SyaratDokumen
		if err := rows.Scan(&s.ID, &s.JenisLayananID, &s.Name, &s.IsRequired, &s.Notes, &s.Sort); err != nil {
			return nil, err
		}
		out[s.JenisLayananID] = append(out[s.JenisLayananID], s)
	}

	return out, rows.Err()
}

// FindActiveLayanan resolves a (instansi, layanan) pair in one round trip
// and proves both are active and belong together. Returns (nil, nil, nil)
// when the pair is invalid — callers map that to 404.
func (r *InstansiRepository) FindActiveLayanan(ctx context.Context, instansiID, layananID string) (*domain.Layanan, *domain.Instansi, error) {
	return r.FindActiveLayananWith(ctx, r.db, instansiID, layananID)
}

// FindActiveLayananWith is FindActiveLayanan on a caller-supplied
// querier. Callers already inside a transaction MUST use this and pass
// their tx — see dbx.Querier for why borrowing a second connection
// deadlocks under load.
func (r *InstansiRepository) FindActiveLayananWith(ctx context.Context, q dbx.Querier, instansiID, layananID string) (*domain.Layanan, *domain.Instansi, error) {
	query := `
		SELECT l.id, l.instansi_id, l.name, l.description, l.estimasi_durasi_menit,
		       l.requires_fo_verification, l.is_active,
		       i.id, i.company_id, i.name, i.slug, i.prefix, i.description, i.logo_url,
		       i.operating_hours, i.queue_mode, i.is_active, i.created_at, i.updated_at
		FROM mpp.jenis_layanan l
		JOIN mpp.instansi i ON i.id = l.instansi_id
		WHERE l.id = $1 AND l.instansi_id = $2 AND i.company_id = $3
		  AND l.deleted_at IS NULL AND l.is_active = TRUE
		  AND i.deleted_at IS NULL AND i.is_active = TRUE`

	var l domain.Layanan
	var i domain.Instansi
	err := q.QueryRow(ctx, query, layananID, instansiID, r.companyID).Scan(
		&l.ID, &l.InstansiID, &l.Name, &l.Description, &l.EstimasiDurasiMenit,
		&l.RequiresFOVerification, &l.IsActive,
		&i.ID, &i.CompanyID, &i.Name, &i.Slug, &i.Prefix, &i.Description, &i.LogoURL,
		&i.OperatingHours, &i.QueueMode, &i.IsActive, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		logger.Error("Failed to resolve layanan", logger.Err(err))
		return nil, nil, err
	}

	l.Syarat = make([]domain.SyaratDokumen, 0)
	return &l, &i, nil
}
