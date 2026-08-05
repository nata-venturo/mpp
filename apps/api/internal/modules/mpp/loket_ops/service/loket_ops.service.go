package service

import (
	"context"
	"errors"
	"time"

	antriandomain "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/domain"
	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/ws"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/repository"
)

var (
	// ErrNoTransition — the item is not in the state this action needs
	// (or the recall cap is reached). 409.
	ErrNoTransition = repository.ErrNoTransition
	// ErrLoketNotFound — unknown loket, or outside this tenant. 404.
	ErrLoketNotFound = errors.New("loket not found")
	// ErrAntrianNotFound — unknown queue item, or outside this tenant. 404.
	ErrAntrianNotFound = errors.New("antrian not found")
	// ErrNotYourLoket — the counter is held by a different operator, or
	// the caller has no open session there. 403.
	ErrNotYourLoket = errors.New("loket belongs to another operator")
)

type LoketOpsService struct {
	repo         *repository.LoketOpsRepository
	loketRepo    *loketrepo.LoketRepository
	antrianRepo  *antrianrepo.AntrianRepository
	instansiRepo *instansirepo.InstansiRepository
	antrian      *antriansvc.AntrianService
	cfg          *configsvc.ConfigService
	hub          *ws.Hub
	companyID    string
}

func NewLoketOpsService(
	repo *repository.LoketOpsRepository,
	loketRepo *loketrepo.LoketRepository,
	antrianRepo *antrianrepo.AntrianRepository,
	instansiRepo *instansirepo.InstansiRepository,
	antrian *antriansvc.AntrianService,
	cfg *configsvc.ConfigService,
	hub *ws.Hub,
	companyID string,
) *LoketOpsService {
	return &LoketOpsService{
		repo:         repo,
		loketRepo:    loketRepo,
		antrianRepo:  antrianRepo,
		instansiRepo: instansiRepo,
		antrian:      antrian,
		cfg:          cfg,
		hub:          hub,
		companyID:    companyID,
	}
}

// Session opens or closes the operator's shift at a loket. Opening a
// counter someone else already holds is a 403, not a takeover — two
// operators calling from one counter would call the same person twice.
func (s *LoketOpsService) Session(ctx context.Context, loketID, userID, action string) (*dto.SessionResponse, error) {
	loket, err := s.loketRepo.FindByID(ctx, loketID)
	if err != nil {
		return nil, err
	}
	if loket == nil {
		return nil, ErrLoketNotFound
	}

	if action == "close" {
		session, err := s.repo.CloseSession(ctx, loketID, userID)
		if err != nil {
			return nil, err
		}
		return toSessionResponse(session, loket.DisplayName()), nil
	}

	session, err := s.repo.OpenSession(ctx, loketID, userID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrNoTransition
	}
	if session.UserID != userID {
		return nil, ErrNotYourLoket
	}

	return toSessionResponse(session, loket.DisplayName()), nil
}

// CallNext pulls the head of the stream to this operator's counter and
// announces it.
func (s *LoketOpsService) CallNext(ctx context.Context, loketID, userID string) (*dto.AntrianActionResponse, error) {
	loket, err := s.requireOwnLoket(ctx, loketID, userID)
	if err != nil {
		return nil, err
	}

	layananIDs, err := s.loketRepo.ServedLayananIDs(ctx, loketID)
	if err != nil {
		return nil, err
	}
	if len(layananIDs) == 0 {
		// A counter mapped to no service can never call anyone; that is an
		// empty stream, not an error.
		return nil, nil
	}

	tx, err := s.repo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	id, err := s.repo.CallNext(ctx, tx, loketID, layananIDs, s.antrian.OperatingDay())
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil // nobody waiting
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	item, err := s.antrianRepo.FindByID(ctx, s.companyID, id)
	if err != nil || item == nil {
		return nil, err
	}

	resp := s.toActionResponse(ctx, item, loket.name)
	s.publish(ctx, item, "call.created", resp)

	return resp, nil
}

// Recall re-announces a number, up to three times total (BR-16).
func (s *LoketOpsService) Recall(ctx context.Context, antrianID, userID string) (*dto.AntrianActionResponse, error) {
	_, loket, err := s.requireOwnAntrian(ctx, antrianID, userID)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.Recall(ctx, antrianID); err != nil {
		return nil, err
	}

	updated, err := s.antrianRepo.FindByID(ctx, s.companyID, antrianID)
	if err != nil || updated == nil {
		return nil, err
	}

	resp := s.toActionResponse(ctx, updated, loket)
	s.publish(ctx, updated, "call.recalled", resp)

	return resp, nil
}

// Start moves the called citizen to SERVING and starts the clock.
func (s *LoketOpsService) Start(ctx context.Context, antrianID, userID string) (*dto.AntrianActionResponse, error) {
	_, loket, err := s.requireOwnAntrian(ctx, antrianID, userID)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := s.repo.Start(ctx, tx, antrianID, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	updated, err := s.antrianRepo.FindByID(ctx, s.companyID, antrianID)
	if err != nil || updated == nil {
		return nil, err
	}

	resp := s.toActionResponse(ctx, updated, loket)
	resp.TTSText = ""
	s.publish(ctx, updated, "serving.started", resp)

	return resp, nil
}

// Skip records a no-show and frees the counter.
func (s *LoketOpsService) Skip(ctx context.Context, antrianID, userID string) (*dto.AntrianActionResponse, error) {
	return s.close(ctx, antrianID, userID, "SKIPPED")
}

// Done closes the service, recording how long it took.
func (s *LoketOpsService) Done(ctx context.Context, antrianID, userID string) (*dto.AntrianActionResponse, error) {
	return s.close(ctx, antrianID, userID, "DONE")
}

// close is the shared tail of skip and done: transition the ticket, free
// the loket for the idle-longest ordering, publish, and report.
func (s *LoketOpsService) close(ctx context.Context, antrianID, userID, outcome string) (*dto.AntrianActionResponse, error) {
	item, loket, err := s.requireOwnAntrian(ctx, antrianID, userID)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var seconds int
	var doneAt time.Time

	if outcome == "DONE" {
		seconds, doneAt, err = s.repo.Done(ctx, tx, antrianID)
	} else {
		err = s.repo.Skip(ctx, tx, antrianID)
	}
	if err != nil {
		return nil, err
	}

	// The counter is free again — refresh its idle clock so fairness
	// ordering (BR-12) sees it as available from now.
	if item.LoketID != nil {
		if err := s.loketRepo.TouchIdle(ctx, tx, *item.LoketID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	updated, err := s.antrianRepo.FindByID(ctx, s.companyID, antrianID)
	if err != nil || updated == nil {
		return nil, err
	}

	resp := s.toActionResponse(ctx, updated, loket)
	resp.TTSText = ""
	if outcome == "DONE" {
		resp.DurasiDetik = &seconds
		formatted := doneAt.UTC().Format(time.RFC3339)
		resp.DoneAt = &formatted
	}
	s.publish(ctx, updated, "serving.ended", resp)

	return resp, nil
}

// requireOwnLoket proves the caller is the operator currently holding
// this counter. Server-side scope, not a UI convention: the loket id
// arrives in the request body.
func (s *LoketOpsService) requireOwnLoket(ctx context.Context, loketID, userID string) (*loketDisplay, error) {
	loket, err := s.loketRepo.FindByID(ctx, loketID)
	if err != nil {
		return nil, err
	}
	if loket == nil {
		return nil, ErrLoketNotFound
	}

	session, err := s.repo.ActiveSession(ctx, loketID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.UserID != userID {
		return nil, ErrNotYourLoket
	}

	return &loketDisplay{id: loket.ID, name: loket.DisplayName(), instansiID: loket.InstansiID}, nil
}

// requireOwnAntrian resolves a ticket and proves the caller holds the
// counter it is attached to.
func (s *LoketOpsService) requireOwnAntrian(ctx context.Context, antrianID, userID string) (*antriandomain.QueueItem, string, error) {
	item, err := s.antrianRepo.FindByID(ctx, s.companyID, antrianID)
	if err != nil {
		return nil, "", err
	}
	if item == nil {
		return nil, "", ErrAntrianNotFound
	}
	if item.LoketID == nil {
		return nil, "", ErrNoTransition
	}

	loket, err := s.requireOwnLoket(ctx, *item.LoketID, userID)
	if err != nil {
		return nil, "", err
	}

	return item, loket.name, nil
}

type loketDisplay struct {
	id         string
	name       string
	instansiID string
}

func (s *LoketOpsService) toActionResponse(ctx context.Context, item *antriandomain.QueueItem, loketName string) *dto.AntrianActionResponse {
	template := s.cfg.TTSText(ctx, s.repo.DB(), &item.InstansiID).Template

	name := loketName
	if item.LoketName != nil && *item.LoketName != "" {
		name = *item.LoketName
	}

	return &dto.AntrianActionResponse{
		AntrianID:   item.ID,
		Nomor:       item.Nomor,
		Status:      item.Status,
		LoketID:     item.LoketID,
		Loket:       &name,
		CallCount:   item.CallCount,
		PemohonName: item.PemohonName,
		LayananID:   item.JenisLayananID,
		LayananName: item.LayananName,
		TTSText:     BuildTTSText(template, item.Nomor, name),
	}
}

// publish fans a transition out to every audience that cares: the
// service stream (loket apps), the counter, the agency's TV display and
// the agency feed. Realtime is best-effort — REST stays authoritative.
func (s *LoketOpsService) publish(ctx context.Context, item *antriandomain.QueueItem, eventType string, resp *dto.AntrianActionResponse) {
	if s.hub == nil {
		return
	}

	data := map[string]any{
		"antrian_id": resp.AntrianID,
		"nomor":      resp.Nomor,
		"status":     resp.Status,
		"call_count": resp.CallCount,
		"layanan_id": resp.LayananID,
	}
	if resp.Loket != nil {
		data["loket"] = *resp.Loket
	}
	if resp.TTSText != "" {
		data["tts_text"] = resp.TTSText
	}
	if eventType == "serving.ended" {
		data["outcome"] = resp.Status
	}

	channels := []string{
		ws.ChannelLayanan(item.JenisLayananID),
		ws.ChannelDisplay(item.InstansiID),
		ws.ChannelMonitoring,
	}
	if item.LoketID != nil {
		channels = append(channels, ws.ChannelLoket(*item.LoketID))
	}
	if instansi, err := s.instansiRepo.FindByID(ctx, item.InstansiID); err == nil && instansi != nil {
		channels = append(channels, ws.ChannelInstansi(instansi.Prefix))
	}

	s.hub.PublishAll(ctx, channels, ws.Event{Type: eventType, Data: data})

	// Anything that changes the stream also changes what the waiting
	// screens should show.
	waiting, _, err := s.antrianRepo.ListWaiting(ctx, s.companyID, item.JenisLayananID, item.QueueDate, 1, 5)
	if err != nil {
		return
	}
	next := make([]string, 0, len(waiting))
	for i := range waiting {
		next = append(next, waiting[i].Nomor)
	}

	s.hub.PublishAll(ctx, channels, ws.Event{
		Type: "queue.updated",
		Data: map[string]any{
			"layanan_id":    item.JenisLayananID,
			"waiting_count": len(waiting),
			"next":          next,
		},
	})
}

func toSessionResponse(s *repository.Session, loketName string) *dto.SessionResponse {
	var closedAt *string
	if s.ClosedAt != nil {
		formatted := s.ClosedAt.UTC().Format(time.RFC3339)
		closedAt = &formatted
	}

	return &dto.SessionResponse{
		SessionID: s.ID,
		LoketID:   s.LoketID,
		LoketName: loketName,
		IsActive:  s.IsActive,
		OpenedAt:  s.OpenedAt.UTC().Format(time.RFC3339),
		ClosedAt:  closedAt,
	}
}
