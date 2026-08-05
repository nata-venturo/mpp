package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketopssvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/repository"
)

// ErrInstansiNotFound — unknown agency, or outside this tenant.
var ErrInstansiNotFound = errors.New("instansi not found")

// nextUpLimit is how many upcoming numbers a TV shows. More than this
// is unreadable across a waiting hall.
const nextUpLimit = 5

type DisplayService struct {
	repo         *repository.DisplayRepository
	instansiRepo *instansirepo.InstansiRepository
	antrian      *antriansvc.AntrianService
	cfg          *configsvc.ConfigService
	// db serves the config read. The display path is read-only and never
	// runs inside a transaction, so the pool is the right querier here.
	db *pgxpool.Pool
}

func NewDisplayService(
	db *pgxpool.Pool,
	repo *repository.DisplayRepository,
	instansiRepo *instansirepo.InstansiRepository,
	antrian *antriansvc.AntrianService,
	cfg *configsvc.ConfigService,
) *DisplayService {
	return &DisplayService{
		db:           db,
		repo:         repo,
		instansiRepo: instansiRepo,
		antrian:      antrian,
		cfg:          cfg,
	}
}

// Snapshot is the TV's whole picture: who is being called at which
// counter (with the phrase to speak) and who is next.
//
// The device polls this on load and after a reconnect; live updates
// arrive over WebSocket. Keeping both paths on the same shape means a
// TV that loses its socket degrades to "slightly stale", not "blank".
func (s *DisplayService) Snapshot(ctx context.Context, instansiID string) (*dto.DisplayResponse, error) {
	instansi, err := s.instansiRepo.FindByID(ctx, instansiID)
	if err != nil {
		return nil, err
	}
	if instansi == nil {
		return nil, ErrInstansiNotFound
	}

	day := s.antrian.OperatingDay()

	calls, err := s.repo.CurrentCalls(ctx, instansiID, day)
	if err != nil {
		return nil, err
	}

	upcoming, err := s.repo.NextUp(ctx, instansiID, day, nextUpLimit)
	if err != nil {
		return nil, err
	}

	template := s.cfg.TTSText(ctx, s.db, &instansiID).Template

	current := make([]dto.CurrentCall, 0, len(calls))
	for _, c := range calls {
		current = append(current, dto.CurrentCall{
			AntrianID: c.AntrianID,
			Nomor:     c.Nomor,
			Status:    c.Status,
			LoketID:   c.LoketID,
			Loket:     c.LoketName,
			TTSText:   loketopssvc.BuildTTSText(template, c.Nomor, c.LoketName),
		})
	}

	next := make([]dto.NextUp, 0, len(upcoming))
	for _, u := range upcoming {
		next = append(next, dto.NextUp{
			AntrianID: u.AntrianID,
			Nomor:     u.Nomor,
			Layanan:   u.LayananName,
		})
	}

	return &dto.DisplayResponse{
		Instansi: dto.InstansiRef{
			ID:     instansi.ID,
			Name:   instansi.Name,
			Prefix: instansi.Prefix,
		},
		Current: current,
		Next:    next,
	}, nil
}

// SnapshotForChannel implements ws.SnapshotProvider: a device that
// subscribes to `display:<instansi_id>` gets the same picture the REST
// snapshot would have given it, so reconnects re-sync in one frame.
func (s *DisplayService) SnapshotForChannel(ctx context.Context, channel string) (map[string]any, error) {
	instansiID, ok := strings.CutPrefix(channel, "display:")
	if !ok {
		// Other channels (layanan:, loket:, monitoring) carry deltas only;
		// an empty snapshot is the correct answer for them.
		return nil, nil
	}

	snapshot, err := s.Snapshot(ctx, instansiID)
	if err != nil {
		if errors.Is(err, ErrInstansiNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return map[string]any{
		"instansi": snapshot.Instansi,
		"current":  snapshot.Current,
		"next":     snapshot.Next,
	}, nil
}
