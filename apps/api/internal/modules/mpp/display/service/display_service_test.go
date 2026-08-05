package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	antriandto "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/dto"
	antrianrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/repository"
	antriansvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian/service"
	bookingrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	configrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	configsvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	instansirepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	loketrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	loketopsrepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/repository"
	loketopssvc "github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display/service"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
	loket1ID   = "a5000000-0000-0000-0000-000000000001"
	petugasID  = "a9000000-0000-0000-0000-000000000001"
)

var jakarta = mustJakarta()

func mustJakarta() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

type harness struct {
	display *service.DisplayService
	antrian *antriansvc.AntrianService
	ops     *loketopssvc.LoketOpsService
}

func setup(t *testing.T) *harness {
	t.Helper()

	pool := testutil.RequireDB(t)
	rdb := testutil.RequireRedis(t)
	testutil.TruncateMPP(t, pool)

	cfg := configsvc.NewConfigService(configrepo.NewConfigRepository(pool), jakarta)
	instRepo := instansirepo.NewInstansiRepository(pool, companyID)
	lokRepo := loketrepo.NewLoketRepository(pool, companyID)
	antRepo := antrianrepo.NewAntrianRepository(pool, rdb)

	antrian := antriansvc.NewAntrianService(
		antRepo, instRepo, lokRepo,
		bookingrepo.NewBookingRepository(pool, companyID), cfg, companyID, jakarta)

	key := "mpp:counter:" + instansiID + ":" + antrian.OperatingDay().Format("20060102")
	if err := rdb.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("flush counter: %v", err)
	}

	return &harness{
		display: service.NewDisplayService(pool, repository.NewDisplayRepository(pool), instRepo, antrian, cfg),
		antrian: antrian,
		ops: loketopssvc.NewLoketOpsService(
			loketopsrepo.NewLoketOpsRepository(pool), lokRepo, antRepo, instRepo, antrian, cfg, nil, companyID),
	}
}

func (h *harness) enqueue(t *testing.T, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		if _, err := h.antrian.WalkIn(context.Background(), &antriandto.WalkInRequest{
			InstansiID: instansiID,
			LayananID:  layananID,
			Pemohon: antriandto.PemohonRequest{
				Name:  fmt.Sprintf("Warga %d", i),
				Phone: fmt.Sprintf("62816000%04d", i),
			},
		}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
}

func TestSnapshotShowsCurrentCallAndNextUp(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 4)
	if _, err := h.ops.Session(ctx, loket1ID, petugasID, "open"); err != nil {
		t.Fatalf("open session: %v", err)
	}

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}

	snapshot, err := h.display.Snapshot(ctx, instansiID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snapshot.Instansi.Prefix != "A" {
		t.Errorf("prefix = %q, want A", snapshot.Instansi.Prefix)
	}
	if len(snapshot.Current) != 1 {
		t.Fatalf("current calls = %d, want 1", len(snapshot.Current))
	}
	if snapshot.Current[0].Nomor != called.Nomor || snapshot.Current[0].Loket != "Loket 1" {
		t.Fatalf("current = %+v, want %s at Loket 1", snapshot.Current[0], called.Nomor)
	}
	if snapshot.Current[0].TTSText == "" {
		t.Error("current call has no tts_text — the TV would have nothing to speak")
	}

	if len(snapshot.Next) != 3 {
		t.Fatalf("next-up = %d, want 3", len(snapshot.Next))
	}
	if snapshot.Next[0].Nomor != "A-002" {
		t.Fatalf("next[0] = %s, want A-002", snapshot.Next[0].Nomor)
	}
}

// After start, the counter must still show the same number: SERVING is
// "being helped at Loket 1", not "nothing to display".
func TestSnapshotKeepsShowingAServingNumber(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 1)
	if _, err := h.ops.Session(ctx, loket1ID, petugasID, "open"); err != nil {
		t.Fatalf("open session: %v", err)
	}

	called, err := h.ops.CallNext(ctx, loket1ID, petugasID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if _, err := h.ops.Start(ctx, called.AntrianID, petugasID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	snapshot, err := h.display.Snapshot(ctx, instansiID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Current) != 1 || snapshot.Current[0].Status != "SERVING" {
		t.Fatalf("current = %+v, want one SERVING entry", snapshot.Current)
	}

	// Finishing clears the counter.
	if _, err := h.ops.Done(ctx, called.AntrianID, petugasID); err != nil {
		t.Fatalf("Done: %v", err)
	}

	after, err := h.display.Snapshot(ctx, instansiID)
	if err != nil {
		t.Fatalf("Snapshot after done: %v", err)
	}
	if len(after.Current) != 0 {
		t.Fatalf("current after done = %+v, want empty", after.Current)
	}
}

func TestSnapshotUnknownInstansi(t *testing.T) {
	h := setup(t)

	_, err := h.display.Snapshot(context.Background(), "a2000000-0000-0000-0000-0000000009ff")
	if !errors.Is(err, service.ErrInstansiNotFound) {
		t.Fatalf("Snapshot = %v, want ErrInstansiNotFound", err)
	}
}

// The WebSocket snapshot must match the REST one — a reconnecting TV
// re-syncs from the socket, not from a second HTTP call.
func TestSnapshotForChannel(t *testing.T) {
	h := setup(t)
	ctx := context.Background()

	h.enqueue(t, 1)

	data, err := h.display.SnapshotForChannel(ctx, "display:"+instansiID)
	if err != nil {
		t.Fatalf("SnapshotForChannel: %v", err)
	}
	if data == nil || data["next"] == nil {
		t.Fatalf("snapshot payload = %+v, want instansi/current/next", data)
	}

	// Channels that carry deltas only get an empty snapshot, not an error.
	other, err := h.display.SnapshotForChannel(ctx, "layanan:"+layananID)
	if err != nil {
		t.Fatalf("SnapshotForChannel(layanan): %v", err)
	}
	if other != nil {
		t.Fatalf("delta channel snapshot = %+v, want nil", other)
	}
}
