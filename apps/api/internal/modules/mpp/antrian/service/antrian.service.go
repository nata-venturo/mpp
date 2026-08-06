package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	bookingdomain "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/domain"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// ErrLayananNotFound covers a missing/inactive agency-service pair.
var ErrLayananNotFound = errors.New("layanan not found")

// enqueueAttempts bounds the re-allocation loop that runs when the DB
// unique index rejects a sequence Redis handed out twice.
const enqueueAttempts = 3

// EnqueueInput is everything needed to put someone into a queue. It is
// shared by check-in (source BOOKING) and walk-in (source WALK_IN) so
// numbering exists in exactly one implementation.
type EnqueueInput struct {
	BookingID  *string
	PemohonID  string
	InstansiID string
	LayananID  string
	Source     string
}

type AntrianService struct {
	repo         *repository.AntrianRepository
	instansiRepo *instansirepo.InstansiRepository
	loketRepo    *loketrepo.LoketRepository
	bookingRepo  *bookingrepo.BookingRepository
	cfg          *configsvc.ConfigService
	companyID    string
	loc          *time.Location
}

func NewAntrianService(
	repo *repository.AntrianRepository,
	instansiRepo *instansirepo.InstansiRepository,
	loketRepo *loketrepo.LoketRepository,
	bookingRepo *bookingrepo.BookingRepository,
	cfg *configsvc.ConfigService,
	companyID string,
	loc *time.Location,
) *AntrianService {
	return &AntrianService{
		repo:         repo,
		instansiRepo: instansiRepo,
		loketRepo:    loketRepo,
		bookingRepo:  bookingRepo,
		cfg:          cfg,
		companyID:    companyID,
		loc:          loc,
	}
}

// OperatingDay is the local calendar day a ticket belongs to. The server
// runs in UTC, so "today" has to be asked of the operating timezone or
// every ticket issued after 17:00 UTC lands on the wrong day.
func (s *AntrianService) OperatingDay() time.Time {
	now := time.Now().In(s.loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatNomor renders a sequence as the printed queue number (BR-04).
// `{prefix}` and `{seq}` are substituted; Pad zero-pads the sequence.
func FormatNomor(prefix string, seq int, format configsvc.NumberFormat) string {
	digits := strconv.Itoa(seq)
	for len(digits) < format.Pad {
		digits = "0" + digits
	}

	out := strings.ReplaceAll(format.Pattern, "{prefix}", prefix)
	return strings.ReplaceAll(out, "{seq}", digits)
}

// Enqueue allocates a number and inserts a WAITING ticket inside the
// caller's transaction — so check-in and enqueue commit or fail as one.
//
// The insert runs inside a savepoint: a unique-index rejection would
// otherwise poison the outer transaction and take the caller's work
// (e.g. the booking status flip) down with it.
func (s *AntrianService) Enqueue(ctx context.Context, tx pgx.Tx, in EnqueueInput) (*domain.Antrian, error) {
	// Every read below runs on the caller's tx, not the pool — see
	// dbx.Querier: a pool read here deadlocks once concurrent check-ins
	// outnumber the connection pool.
	layanan, instansi, err := s.instansiRepo.FindActiveLayananWith(ctx, tx, in.InstansiID, in.LayananID)
	if err != nil {
		return nil, err
	}
	if layanan == nil || instansi == nil {
		return nil, ErrLayananNotFound
	}

	day := s.OperatingDay()
	format := s.cfg.NumberFormat(ctx, tx, &in.InstansiID)

	priority := 0
	if instansi.QueueMode == domain.ModeBookingPriority && in.Source == domain.SourceBooking {
		priority = 1
	}

	for attempt := 0; attempt < enqueueAttempts; attempt++ {
		seq, err := s.repo.NextSeq(ctx, tx, in.InstansiID, day)
		if err != nil {
			return nil, err
		}

		item := &domain.Antrian{
			BookingID:      in.BookingID,
			PemohonID:      in.PemohonID,
			InstansiID:     in.InstansiID,
			JenisLayananID: in.LayananID,
			Nomor:          FormatNomor(instansi.Prefix, seq, format),
			NomorSeq:       seq,
			QueueDate:      day,
			Source:         in.Source,
			Priority:       priority,
		}

		sp, err := tx.Begin(ctx)
		if err != nil {
			return nil, err
		}

		err = s.repo.Create(ctx, sp, item)
		if err == nil {
			if err := sp.Commit(ctx); err != nil {
				return nil, err
			}
			return item, nil
		}

		_ = sp.Rollback(ctx)
		if !errors.Is(err, repository.ErrDuplicateSeq) {
			return nil, err
		}
		logger.Warn("Queue sequence collided — re-allocating",
			logger.String("instansi_id", in.InstansiID))
	}

	return nil, repository.ErrDuplicateSeq
}

// WalkIn registers an applicant who arrived without a booking and puts
// them straight into the stream. No quota is consumed: quota governs
// advance bookings, not people already standing in the building.
func (s *AntrianService) WalkIn(ctx context.Context, req *dto.WalkInRequest) (*dto.AntrianResponse, error) {
	tx, err := s.repo.DB().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	pemohonID, err := s.bookingRepo.UpsertPemohon(ctx, tx, &bookingdomain.Pemohon{
		Name:  req.Pemohon.Name,
		Phone: &req.Pemohon.Phone,
		Email: req.Pemohon.Email,
	})
	if err != nil {
		return nil, err
	}

	item, err := s.Enqueue(ctx, tx, EnqueueInput{
		PemohonID:  pemohonID,
		InstansiID: req.InstansiID,
		LayananID:  req.LayananID,
		Source:     domain.SourceWalkIn,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.ToAntrianResponse(ctx, item), nil
}

// ToAntrianResponse packages a ticket with its live ETA.
func (s *AntrianService) ToAntrianResponse(ctx context.Context, item *domain.Antrian) *dto.AntrianResponse {
	return &dto.AntrianResponse{
		AntrianID:   item.ID,
		Nomor:       item.Nomor,
		NomorSeq:    item.NomorSeq,
		QueueStatus: item.Status,
		EtaMenit:    s.EstimateWait(ctx, item),
		InstansiID:  item.InstansiID,
		LayananID:   item.JenisLayananID,
		QueuedAt:    item.QueuedAt.UTC().Format(time.RFC3339),
	}
}

// EstimateWait implements BR-29:
//
//	eta = ceil(position / max(open_lokets, 1)) * estimasi_durasi_menit
//
// where position is how many tickets are ahead in the same service. An
// error anywhere degrades to 0 — a missing estimate must never block a
// ticket from being issued.
func (s *AntrianService) EstimateWait(ctx context.Context, item *domain.Antrian) int {
	ahead, err := s.repo.CountAhead(ctx, item.JenisLayananID, item.QueueDate, item.Priority, item.QueuedAt)
	if err != nil || ahead <= 0 {
		return 0
	}

	layanan, _, err := s.instansiRepo.FindActiveLayanan(ctx, item.InstansiID, item.JenisLayananID)
	if err != nil || layanan == nil {
		return 0
	}

	lokets, err := s.loketRepo.CountOpenForLayanan(ctx, item.JenisLayananID)
	if err != nil || lokets < 1 {
		lokets = 1
	}

	rounds := (ahead + lokets - 1) / lokets // integer ceil
	return rounds * layanan.EstimasiDurasiMenit
}

// Queue reads today's waiting stream for one service.
func (s *AntrianService) Queue(ctx context.Context, layananID string, page, limit int) ([]dto.QueueItemResponse, int64, error) {
	list, total, err := s.repo.ListWaiting(ctx, s.companyID, layananID, s.OperatingDay(), page, limit)
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.QueueItemResponse, 0, len(list))
	for i := range list {
		out = append(out, ToQueueItemResponse(&list[i]))
	}

	return out, total, nil
}

// ToQueueItemResponse maps one stream row to its wire shape.
func ToQueueItemResponse(it *domain.QueueItem) dto.QueueItemResponse {
	return dto.QueueItemResponse{
		AntrianID: it.ID,
		Nomor:     it.Nomor,
		Status:    it.Status,
		Source:    it.Source,
		CallCount: it.CallCount,
		Loket:     it.LoketName,
		QueuedAt:  it.QueuedAt.UTC().Format(time.RFC3339),
	}
}
