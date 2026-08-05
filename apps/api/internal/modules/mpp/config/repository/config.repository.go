package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/shared/dbx"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// ConfigRepository reads mpp.system_config. Values are JSONB blobs keyed
// by config_key, scoped either to one agency or globally (instansi_id
// NULL). A per-agency row always wins over the global default.
type ConfigRepository struct {
	db *pgxpool.Pool
}

func NewConfigRepository(db *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// Get returns the effective value for a key, or (nil, nil) when neither
// an agency override nor a global default exists.
//
// `q` is the caller's querier: config is read from inside the check-in /
// enqueue transaction, and going to the pool there would check out a
// second connection and deadlock under load (see dbx.Querier).
func (r *ConfigRepository) Get(ctx context.Context, q dbx.Querier, instansiID *string, key string) (json.RawMessage, error) {
	// ORDER BY puts the agency row (instansi_id NOT NULL) first, so LIMIT 1
	// resolves the override precedence in a single round trip.
	const query = `
		SELECT config_value
		FROM mpp.system_config
		WHERE config_key = $1
		  AND ( instansi_id IS NULL
		     OR ($2::uuid IS NOT NULL AND instansi_id = $2::uuid) )
		ORDER BY (instansi_id IS NULL) ASC
		LIMIT 1`

	var raw []byte
	err := q.QueryRow(ctx, query, key, instansiID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to read system config", logger.Err(err))
		return nil, err
	}

	return json.RawMessage(raw), nil
}
