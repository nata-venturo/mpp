package repository_test

import (
	"context"
	"testing"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	loket3ID   = "a5000000-0000-0000-0000-000000000003"
	layananID  = "a3000000-0000-0000-0000-000000000002"
)

func TestFindByInstansiReturnsSeededLokets(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewLoketRepository(pool, companyID)

	list, err := repo.FindByInstansi(context.Background(), instansiID)
	if err != nil {
		t.Fatalf("FindByInstansi: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 seeded lokets, got %d", len(list))
	}
	if list[2].DisplayName() != "Loket 3" {
		t.Errorf("third loket = %q, want 'Loket 3'", list[2].DisplayName())
	}
}

func TestServedLayananIDsCoversAgencyServices(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewLoketRepository(pool, companyID)

	ids, err := repo.ServedLayananIDs(context.Background(), loket3ID)
	if err != nil {
		t.Fatalf("ServedLayananIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("want 3 mapped services, got %d (%v)", len(ids), ids)
	}
}

func TestCountOpenForLayanan(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewLoketRepository(pool, companyID)

	n, err := repo.CountOpenForLayanan(context.Background(), layananID)
	if err != nil {
		t.Fatalf("CountOpenForLayanan: %v", err)
	}
	if n != 3 {
		t.Fatalf("want 3 OPEN lokets for the service, got %d", n)
	}
}
