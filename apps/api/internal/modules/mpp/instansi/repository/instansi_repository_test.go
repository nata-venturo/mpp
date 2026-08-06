package repository_test

import (
	"context"
	"testing"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	seededCompanyID  = "a1000000-0000-0000-0000-000000000001"
	seededInstansiID = "a2000000-0000-0000-0000-000000000001"
	seededLayananID  = "a3000000-0000-0000-0000-000000000002"
)

func TestFindAllReturnsSeededAgencies(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewInstansiRepository(pool, seededCompanyID)

	list, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(list) < 3 {
		t.Fatalf("want >= 3 seeded agencies, got %d", len(list))
	}

	var found bool
	for _, i := range list {
		if i.ID == seededInstansiID {
			found = true
			if i.Prefix != "A" {
				t.Errorf("prefix = %q, want A", i.Prefix)
			}
			if i.QueueMode != "FIFO" {
				t.Errorf("queue_mode = %q, want FIFO", i.QueueMode)
			}
		}
	}
	if !found {
		t.Fatalf("seeded Dukcapil %s missing from FindAll", seededInstansiID)
	}
}

func TestFindLayananByInstansiIncludesSyarat(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewInstansiRepository(pool, seededCompanyID)

	list, err := repo.FindLayananByInstansi(context.Background(), seededInstansiID)
	if err != nil {
		t.Fatalf("FindLayananByInstansi: %v", err)
	}

	for _, l := range list {
		if l.ID != seededLayananID {
			continue
		}
		if l.EstimasiDurasiMenit != 10 {
			t.Errorf("estimasi = %d, want 10", l.EstimasiDurasiMenit)
		}
		if len(l.Syarat) != 2 {
			t.Fatalf("syarat count = %d, want 2", len(l.Syarat))
		}
		if l.Syarat[0].Sort > l.Syarat[1].Sort {
			t.Errorf("syarat not sorted by sort ASC: %+v", l.Syarat)
		}
		return
	}
	t.Fatalf("seeded layanan %s not returned", seededLayananID)
}

func TestFindActiveLayananRejectsForeignAgency(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewInstansiRepository(pool, seededCompanyID)

	// Layanan belongs to Dukcapil; ask for it under Imigrasi.
	l, i, err := repo.FindActiveLayanan(context.Background(),
		"a2000000-0000-0000-0000-000000000002", seededLayananID)
	if err != nil {
		t.Fatalf("FindActiveLayanan: %v", err)
	}
	if l != nil || i != nil {
		t.Fatalf("want (nil, nil) for cross-agency lookup, got (%v, %v)", l, i)
	}
}
