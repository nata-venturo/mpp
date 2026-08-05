# MPP Walking Skeleton (Slices 0–6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the end-to-end MPP happy path — citizen books a slot (atomic quota) → gets a single-use QR → checks in at a kiosk → receives an atomic queue number `A-014` → an operator calls/recalls/starts/finishes them → the TV shows and speaks the number offline.

**Architecture:** Backend adds MPP modules under `apps/api/internal/modules/mpp/<name>/` (domain/dto/repository/service/handler + `main.<name>.go`), all wired into a new `/mpp/v1` Gin router group. Postgres (schema `mpp`, migrations already applied) is the source of truth; Redis is the queue-number counter and the WebSocket pub/sub backbone. Frontend adds `(citizen)`, `(kiosk)`, `(loket)`, `(tv)` route groups following the strict page → view → section pattern with the ky+zod+TanStack API layer.

**Tech Stack:** Go 1.26 · Gin 1.11 · pgx v5 (no ORM) · go-redis v9 · gorilla/websocket · Next.js 16 (App Router) · React 19 · MUI v9 · TanStack Query v5 · ky · react-hook-form + zod v4 · qrcode.react.

---

## Global Constraints

Every task's requirements implicitly include this section.

- **Go module path:** `github.com/ndollem/mpp/apps/api`. Frontend imports are rooted at `src/…` (no `@/`).
- **Response envelope, exact helpers** (`internal/shared/response/response.go`):
  - `response.Success(c, http.StatusOK, "message", data)`
  - `response.SuccessWithPagination(c, http.StatusOK, "message", data, page, limit, total)`
  - `response.Error(c, http.StatusBadRequest, "message", "detail")`
  - `response.ValidationError(c, http.StatusBadRequest, "message", map[string][]string{...})`
- **HTTP status vocabulary:** 200/201 ok · 400 validation · 401 no/bad credentials · 403 authenticated-but-forbidden · 404 not found/out of scope · **409 conflict** (quota full, illegal transition, reused token, duplicate number) · **410 gone** (expired QR) · 429 rate limited.
- **All timestamps UTC** in requests/responses. Local-day arithmetic (booking day, queue day, QR expiry) uses `cfg.MPP.Location` (`MPP_LOCAL_TZ`, default `Asia/Jakarta`).
- **Tenancy:** one MPP building = one company. Public MPP reads are scoped by `cfg.MPP.CompanyID`.
- **No ORM.** Hand-written SQL through `pgxpool`/`pgx.Tx`. Every read filters `deleted_at IS NULL` where the table has that column.
- **RBAC verbs are fixed** (`viewer→read`, `editor→create,read,update,delete`, `admin→+export,import,restore,approve`). Never invent verbs. Guards used in this plan, verbatim:
  `mpp.instansi:read` · `mpp.layanan:read` · `mpp.loket:read` · `mpp.booking:create` · `mpp.checkin:create` · `mpp.queue:read` · `mpp.queue:update` · `mpp.antrian:update` · `mpp.display:read`.
- **Auth entry point:** `middleware.JWTAuth()` is unified — it checks `X-API-Key` first, then `Authorization: Bearer`. There is no separate API-key middleware.
- **Module shape:** `Initialize(...) *Module` builds repo → service → handler; `SetupRoutes(rg *gin.RouterGroup)` registers routes; wire in `internal/router/router.go` inside the `mppV1` group.
- **Frontend api-layer trio per resource:** `endpoints.ts` entry → `<resource>.ts` (zod parsed at the fetch boundary, no `as` casts, + query-key factory) → `use-<resource>.ts` (`'use client'` hooks, never re-exported from `src/lib/api/index.ts`).
- **Frontend UI:** page (`src/app/...`) → view (`src/sections/<vertical>/view/*-view.tsx`, `'use client'`) → section components. Routes only via `src/routes/paths.ts`. Env only via `CONFIG` (`src/global-config.ts`). Forms only via `Field.*` + `Form` from `src/components/hook-form/` with `zodResolver`.
- **Verification gates:** backend `cd apps/api && go test ./internal/modules/mpp/... && go build ./...`; frontend `cd apps/web && yarn tsc:check && yarn lint`. Never claim done without pasting the command output.
- **Commit style:** conventional commits (`feat:`, `test:`, `chore:`), one commit per task step that says "Commit".

### Decisions locked before Task 1 (do not re-litigate)

1. **Public tenant resolution.** `core.companies` has **no `slug` column** and no backend code reads `X-Company-Slug`, so the header cannot resolve the tenant today. Public MPP reads use `MPP_COMPANY_ID` (default = seeded MPP company `a1000000-0000-0000-0000-000000000001`). The FE keeps sending `X-Company-Slug` (harmless). Marked with a `// ponytail:` note pointing at a `companies.slug` column as the upgrade path when multi-building lands.
2. **QR token stored raw** (not hashed) so check-in is a single indexed lookup; the column is written in exactly one place (`bookingRepo.Create`) and read in one place (`FindBookingByToken`) so a hash swap is a two-line change. Marked `// ponytail:`.
3. **Route groups need a URL segment.** `docs/prompt/00-prerequisites.md` P7 suggests `src/app/(kiosk)/page.tsx`, which would collide with the existing `src/app/(home)/page.tsx` at `/`. Kiosk/loket/TV therefore live at `/kiosk`, `/loket`, `/display/[instansi]`.
4. **Master-data admin CRUD is deferred.** `docs/prompt/00-prerequisites.md` P2 explicitly allows a read-first implementation. Tasks 2–3 ship the `GET` endpoints the slices need; write endpoints for instansi/layanan/loket/kuota stay in delivery-plan Fase 1 follow-up work (listed in "Deferred" at the end of this plan).
5. **Existing seeders are authoritative.** `seeders/mpp/001_roles.sql` … `007_demo_antrian.sql` already satisfy P3/P4 with role codes `mpp_admin`, `mpp_supervisor`, `mpp_front_office`, `mpp_petugas_loket` (ids `a0000000-…-0001..0004`). Do not renumber them.
6. **Device API keys need their own owner users.** `ApiKey.GetEffectivePermissions` intersects the key's scope with the **owning user's** permissions, and the seeded `super_admin` role has `'{}'` permissions — a key owned by `owner@gmail.com` would resolve to zero permissions. Task 4 seeds dedicated device roles + device users.

### Seeded identifiers used throughout

| Thing | UUID / value |
|---|---|
| MPP company (tenant) | `a1000000-0000-0000-0000-000000000001` |
| Instansi Dukcapil (prefix `A`, FIFO) | `a2000000-0000-0000-0000-000000000001` |
| Instansi Imigrasi (prefix `B`, BOOKING_PRIORITY) | `a2000000-0000-0000-0000-000000000002` |
| Layanan "Pencetakan / Perpanjangan KTP-el" (10 mnt) | `a3000000-0000-0000-0000-000000000002` |
| Loket 1/2/3 Dukcapil | `a5000000-0000-0000-0000-00000000000{1,2,3}` |
| Roles admin/supervisor/FO/petugas | `a0000000-0000-0000-0000-00000000000{1,2,3,4}` |
| Agency-wide quota rows | `kuota=30` for today..today+6 (`jenis_layanan_id IS NULL`) |

---

## File Structure

### Backend — `apps/api`

| File | Responsibility |
|---|---|
| `internal/config/config.go` *(modify)* | add `MPPConfig{CompanyID, LocalTZ, Location}` |
| `internal/router/router.go` *(modify)* | `mppV1` group + module wiring |
| `internal/modules/mpp/testutil/db.go` | `RequireDB(t)`, `RequireRedis(t)`, `TruncateMPP(t)` |
| `internal/modules/mpp/config/{repository,service}` | `system_config` reader (number format, TTS template, check-in window) |
| `internal/modules/mpp/instansi/**` | agency + service catalog reads |
| `internal/modules/mpp/loket/**` | loket reads (staff) |
| `internal/modules/mpp/kuota/**` | availability + **atomic quota consume** |
| `internal/modules/mpp/booking/**` | booking create/detail, QR token issuance |
| `internal/modules/mpp/checkin/**` | QR validation, `BOOKED → CHECKED_IN`, enqueue seam |
| `internal/modules/mpp/antrian/**` | Redis `INCR` numbering, enqueue, walk-in, queue read, ETA |
| `internal/modules/mpp/loket_ops/**` | session, call next, recall, start, skip, done + TTS text |
| `internal/modules/mpp/display/**` | TV snapshot |
| `internal/modules/mpp/ws/**` | WebSocket hub + Redis pub/sub fan-out |
| `internal/database/seeders/mpp/001_roles.sql` *(modify)* | + device roles |
| `internal/database/seeders/mpp/008_staff_users.sql` | MPP staff users + memberships |
| `internal/database/seeders/mpp/009_device_keys.sql` | device users + scoped API keys |

### Frontend — `apps/web/src`

| File | Responsibility |
|---|---|
| `lib/env.ts`, `global-config.ts` *(modify)* | kiosk/TV API keys, WS URL |
| `lib/api/client.ts` *(modify)* | token store hooks (`beforeRequest`/`afterResponse`) |
| `lib/api/token-store.ts` | in-memory + `localStorage` staff token |
| `lib/api/device-client.ts` | `X-API-Key` ky instance + `deviceFetch` |
| `lib/api/{endpoints,auth,booking,checkin,antrian,loket-ops,display}.ts` + `use-*.ts` | api-layer trios |
| `lib/ws.ts` | WebSocket singleton + `useQueueSocket` |
| `lib/tv/audio-queue.ts` | leader election + offline TTS FIFO |
| `routes/paths.ts` *(modify)* | all new routes |
| `app/(citizen)/**`, `app/(kiosk)/kiosk/**`, `app/(loket)/loket/**`, `app/(tv)/display/[instansi]/**`, `app/signin/**` | thin pages |
| `sections/{citizen,kiosk,loket,tv,auth}/**` | views + sections |

---

## Phase 0 — Prerequisites (`docs/prompt/00-prerequisites.md`)

### Task 1: `/mpp/v1` router group + MPP config

**Files:**
- Modify: `apps/api/internal/config/config.go`
- Modify: `apps/api/internal/router/router.go`
- Modify: `apps/api/.env.example`
- Create: `apps/api/internal/modules/mpp/testutil/db.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `config.MPPConfig{CompanyID string; LocalTZ string; Location *time.Location}` reachable as `cfg.MPP`; the `mppV1 *gin.RouterGroup` in `router.Setup`; `testutil.RequireDB(t) *pgxpool.Pool`, `testutil.RequireRedis(t) *redis.Client`, `testutil.TruncateMPP(t, pool)`.

- [ ] **Step 1: Add the MPP config block**

In `internal/config/config.go`, add the struct, the field on `Config`, and the loader entry:

```go
// MPPConfig holds MPP-domain settings. One MPP building = one company,
// so public (unauthenticated) MPP reads resolve their tenant from
// CompanyID rather than from a request header.
//
// ponytail: core.companies has no slug column and nothing reads
// X-Company-Slug server-side yet. Add companies.slug + a lookup
// middleware when a second building actually exists.
type MPPConfig struct {
	CompanyID string
	LocalTZ   string
	Location  *time.Location
}
```

Add `MPP MPPConfig` to `type Config struct`, and inside `Load()`:

```go
	mppTZ := getEnv("MPP_LOCAL_TZ", "Asia/Jakarta")
	loc, err := time.LoadLocation(mppTZ)
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
```

then, in the returned `&Config{...}` literal:

```go
		MPP: MPPConfig{
			CompanyID: getEnv("MPP_COMPANY_ID", "a1000000-0000-0000-0000-000000000001"),
			LocalTZ:   mppTZ,
			Location:  loc,
		},
```

- [ ] **Step 2: Document the new env vars**

Append to `apps/api/.env.example`:

```bash
# MPP Configuration
# Tenant company for the MPP building (public MPP reads are scoped to it).
MPP_COMPANY_ID=a1000000-0000-0000-0000-000000000001
# Local presentation/operating timezone. Storage stays UTC (TZ=UTC above).
MPP_LOCAL_TZ=Asia/Jakarta
```

- [ ] **Step 3: Add the `mppV1` group to the router**

In `internal/router/router.go`, after the `coreV1` block and before the shared audit module, add:

```go
	// MPP v1 routes — queue domain. Public catalog/registration reads carry
	// no JWTAuth(); staff and device routes add JWTAuth() + RequirePermission.
	mppV1 := router.Group("/mpp/v1")
	{
		// MPP modules register here as slices land.
		_ = mppV1
	}
```

- [ ] **Step 4: Build and smoke the group**

Run: `cd apps/api && go build ./... && go vet ./internal/config/... ./internal/router/...`
Expected: no output (success).

- [ ] **Step 5: Add the MPP test helpers**

Create `apps/api/internal/modules/mpp/testutil/db.go`:

```go
// Package testutil wires MPP integration tests to a real Postgres and
// Redis. There is no test harness in the skeleton, so tests SKIP (never
// fail) when the env is absent — CI without infra stays green while a
// developer with `make up` running gets the real coverage.
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// RequireDB returns a pool against TEST_DATABASE_URL, or skips the test.
func RequireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping MPP integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// RequireRedis returns a client against TEST_REDIS_ADDR (default
// localhost:6379, DB 15), or skips the test. DB 15 is reserved for tests
// so a run never clobbers the dev permission cache (DB 10).
func RequireRedis(t *testing.T) *goredis.Client {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := goredis.NewClient(&goredis.Options{Addr: addr, DB: 15})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis unavailable at %s — skipping: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return client
}

// TruncateMPP clears transactional MPP tables (master data and seeds
// survive) so each test starts from a known state.
func TruncateMPP(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		TRUNCATE mpp.fo_verification, mpp.serving_session, mpp.antrian,
		         mpp.booking, mpp.pemohon, mpp.loket_session RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate mpp tables: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE mpp.kuota_booking SET terpakai = 0`); err != nil {
		t.Fatalf("reset kuota: %v", err)
	}
}
```

- [ ] **Step 6: Verify the helpers compile and skip cleanly**

Run: `cd apps/api && go test ./internal/modules/mpp/...`
Expected: `no test files` for testutil (it has none yet) and a successful build — no compile errors.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/config/config.go apps/api/internal/router/router.go \
        apps/api/.env.example apps/api/internal/modules/mpp/testutil/db.go
git commit -m "feat(mpp): add /mpp/v1 router group, MPP tenant config and test helpers"
```

---

### Task 2: `mpp/instansi` — agency + service catalog reads

**Files:**
- Create: `apps/api/internal/modules/mpp/instansi/domain/instansi.go`
- Create: `apps/api/internal/modules/mpp/instansi/dto/instansi.dto.go`
- Create: `apps/api/internal/modules/mpp/instansi/repository/instansi.repository.go`
- Create: `apps/api/internal/modules/mpp/instansi/service/instansi.service.go`
- Create: `apps/api/internal/modules/mpp/instansi/handler/instansi.handler.go`
- Create: `apps/api/internal/modules/mpp/instansi/main.instansi.go`
- Test: `apps/api/internal/modules/mpp/instansi/repository/instansi_repository_test.go`
- Modify: `apps/api/internal/router/router.go`

**Interfaces:**
- Consumes: `cfg.MPP.CompanyID` (Task 1).
- Produces:
  - `instansi.Initialize(db *pgxpool.Pool, companyID string) *Module` with fields `Handler`, `Service`, `Repository`.
  - `repository.InstansiRepository` methods used by later tasks:
    `FindAll(ctx) ([]domain.Instansi, error)`,
    `FindByID(ctx, id string) (*domain.Instansi, error)`,
    `FindLayananByInstansi(ctx, instansiID string) ([]domain.Layanan, error)`,
    `FindActiveLayanan(ctx, instansiID, layananID string) (*domain.Layanan, *domain.Instansi, error)` — returns `(nil, nil, nil)` when either row is missing/inactive.
  - Routes `GET /mpp/v1/instansi`, `GET /mpp/v1/instansi/:id`, `GET /mpp/v1/instansi/:id/layanan` (all public).

- [ ] **Step 1: Write the failing repository test**

Create `apps/api/internal/modules/mpp/instansi/repository/instansi_repository_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/instansi/...`
Expected: FAIL — `package repository is not in std` / undefined `repository.NewInstansiRepository`.

- [ ] **Step 3: Write the domain structs**

Create `apps/api/internal/modules/mpp/instansi/domain/instansi.go`:

```go
package domain

import "time"

// Instansi is an agency operating inside the MPP building.
type Instansi struct {
	ID             string
	CompanyID      string
	Name           string
	Slug           string
	Prefix         string
	Description    *string
	LogoURL        *string
	OperatingHours []byte // raw JSONB, passed through to the client
	QueueMode      string // FIFO | BOOKING_PRIORITY
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Layanan is a service type under an agency.
type Layanan struct {
	ID                     string
	InstansiID             string
	Name                   string
	Description            *string
	EstimasiDurasiMenit    int
	RequiresFOVerification bool
	IsActive               bool
	Syarat                 []SyaratDokumen
}

// SyaratDokumen is one document requirement attached to a service.
type SyaratDokumen struct {
	ID             string
	JenisLayananID string
	Name           string
	IsRequired     bool
	Notes          *string
	Sort           int
}
```

- [ ] **Step 4: Write the repository**

Create `apps/api/internal/modules/mpp/instansi/repository/instansi.repository.go`:

```go
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/domain"
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

// FindByID returns one active agency, or (nil, nil) when it does not
// exist inside this tenant.
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
		list[idx].Syarat = syarat[list[idx].ID]
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
	err := r.db.QueryRow(ctx, query, layananID, instansiID, r.companyID).Scan(
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
```

- [ ] **Step 5: Run the repository test**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/instansi/... -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Write the DTOs**

Create `apps/api/internal/modules/mpp/instansi/dto/instansi.dto.go`:

```go
package dto

import "encoding/json"

// InstansiResponse is the public agency payload.
type InstansiResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Prefix         string          `json:"prefix"`
	Description    *string         `json:"description"`
	LogoURL        *string         `json:"logo_url"`
	OperatingHours json.RawMessage `json:"operating_hours"`
	QueueMode      string          `json:"queue_mode"`
	IsActive       bool            `json:"is_active"`
}

// SyaratDokumenResponse is one document requirement.
type SyaratDokumenResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	IsRequired bool    `json:"is_required"`
	Notes      *string `json:"notes"`
	Sort       int     `json:"sort"`
}

// LayananResponse is a service plus its requirements.
type LayananResponse struct {
	ID                     string                  `json:"id"`
	InstansiID             string                  `json:"instansi_id"`
	Name                   string                  `json:"name"`
	Description            *string                 `json:"description"`
	EstimasiDurasiMenit    int                     `json:"estimasi_durasi_menit"`
	RequiresFOVerification bool                    `json:"requires_fo_verification"`
	SyaratDokumen          []SyaratDokumenResponse `json:"syarat_dokumen"`
}
```

- [ ] **Step 7: Write the service**

Create `apps/api/internal/modules/mpp/instansi/service/instansi.service.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
)

var ErrInstansiNotFound = errors.New("instansi not found")

type InstansiService struct {
	repo *repository.InstansiRepository
}

func NewInstansiService(repo *repository.InstansiRepository) *InstansiService {
	return &InstansiService{repo: repo}
}

func (s *InstansiService) List(ctx context.Context) ([]dto.InstansiResponse, error) {
	list, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]dto.InstansiResponse, 0, len(list))
	for i := range list {
		out = append(out, toInstansiResponse(&list[i]))
	}

	return out, nil
}

func (s *InstansiService) GetByID(ctx context.Context, id string) (*dto.InstansiResponse, error) {
	i, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, ErrInstansiNotFound
	}

	resp := toInstansiResponse(i)
	return &resp, nil
}

// Layanan lists an agency's services. A missing agency is a 404, not an
// empty list — the citizen picked something that does not exist.
func (s *InstansiService) Layanan(ctx context.Context, instansiID string) ([]dto.LayananResponse, error) {
	i, err := s.repo.FindByID(ctx, instansiID)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, ErrInstansiNotFound
	}

	list, err := s.repo.FindLayananByInstansi(ctx, instansiID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.LayananResponse, 0, len(list))
	for idx := range list {
		out = append(out, toLayananResponse(&list[idx]))
	}

	return out, nil
}

func toInstansiResponse(i *domain.Instansi) dto.InstansiResponse {
	hours := json.RawMessage(i.OperatingHours)
	if len(hours) == 0 {
		hours = json.RawMessage(`{}`)
	}

	return dto.InstansiResponse{
		ID:             i.ID,
		Name:           i.Name,
		Slug:           i.Slug,
		Prefix:         i.Prefix,
		Description:    i.Description,
		LogoURL:        i.LogoURL,
		OperatingHours: hours,
		QueueMode:      i.QueueMode,
		IsActive:       i.IsActive,
	}
}

func toLayananResponse(l *domain.Layanan) dto.LayananResponse {
	syarat := make([]dto.SyaratDokumenResponse, 0, len(l.Syarat))
	for _, s := range l.Syarat {
		syarat = append(syarat, dto.SyaratDokumenResponse{
			ID:         s.ID,
			Name:       s.Name,
			IsRequired: s.IsRequired,
			Notes:      s.Notes,
			Sort:       s.Sort,
		})
	}

	return dto.LayananResponse{
		ID:                     l.ID,
		InstansiID:             l.InstansiID,
		Name:                   l.Name,
		Description:            l.Description,
		EstimasiDurasiMenit:    l.EstimasiDurasiMenit,
		RequiresFOVerification: l.RequiresFOVerification,
		SyaratDokumen:          syarat,
	}
}
```

- [ ] **Step 8: Write the handler**

Create `apps/api/internal/modules/mpp/instansi/handler/instansi.handler.go`:

```go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type InstansiHandler struct {
	instansiService *service.InstansiService
}

func NewInstansiHandler(s *service.InstansiService) *InstansiHandler {
	return &InstansiHandler{instansiService: s}
}

func (h *InstansiHandler) List(c *gin.Context) {
	result, err := h.instansiService.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list instansi", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Instansi retrieved successfully", result)
}

func (h *InstansiHandler) GetByID(c *gin.Context) {
	result, err := h.instansiService.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrInstansiNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get instansi", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Instansi retrieved successfully", result)
}

func (h *InstansiHandler) Layanan(c *gin.Context) {
	result, err := h.instansiService.Layanan(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrInstansiNotFound) {
			response.Error(c, http.StatusNotFound, "Instansi not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get layanan", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Layanan retrieved successfully", result)
}
```

- [ ] **Step 9: Write `main.instansi.go`**

Create `apps/api/internal/modules/mpp/instansi/main.instansi.go`:

```go
package instansi

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/service"
)

type Module struct {
	Handler    *handler.InstansiHandler
	Service    *service.InstansiService
	Repository *repository.InstansiRepository
}

// Initialize builds repo → service → handler for the catalog module.
func Initialize(db *pgxpool.Pool, companyID string) *Module {
	repo := repository.NewInstansiRepository(db, companyID)
	svc := service.NewInstansiService(repo)

	return &Module{
		Handler:    handler.NewInstansiHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the public catalog reads. Catalog data is public
// by design (citizens browse it before any auth), so no JWTAuth here.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/instansi", m.Handler.List)
	rg.GET("/instansi/:id", m.Handler.GetByID)
	rg.GET("/instansi/:id/layanan", m.Handler.Layanan)
}
```

- [ ] **Step 10: Wire it into the router**

In `internal/router/router.go`, add the import `"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi"` and replace the `_ = mppV1` placeholder inside the `mppV1` block with:

```go
		instansiModule := instansi.Initialize(db, cfg.MPP.CompanyID)
		instansiModule.SetupRoutes(mppV1)
```

- [ ] **Step 11: Smoke the endpoints against a running server**

Run (with `make up && make db-setup` done and `make api-dev` running):

```bash
curl -s http://localhost:8080/mpp/v1/instansi | head -c 400
IID=a2000000-0000-0000-0000-000000000001
curl -s http://localhost:8080/mpp/v1/instansi/$IID/layanan | head -c 600
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/mpp/v1/instansi/a2000000-0000-0000-0000-000000000999
```

Expected: envelope with `"data":[…]` listing 3 agencies; layanan list containing `syarat_dokumen`; `404` for the unknown id. Paste the output into the task report.

- [ ] **Step 12: Commit**

```bash
git add apps/api/internal/modules/mpp/instansi apps/api/internal/router/router.go
git commit -m "feat(mpp): add instansi catalog module with public reads"
```

---

### Task 3: `mpp/loket` — counter reads for staff

**Files:**
- Create: `apps/api/internal/modules/mpp/loket/domain/loket.go`
- Create: `apps/api/internal/modules/mpp/loket/dto/loket.dto.go`
- Create: `apps/api/internal/modules/mpp/loket/repository/loket.repository.go`
- Create: `apps/api/internal/modules/mpp/loket/service/loket.service.go`
- Create: `apps/api/internal/modules/mpp/loket/handler/loket.handler.go`
- Create: `apps/api/internal/modules/mpp/loket/main.loket.go`
- Test: `apps/api/internal/modules/mpp/loket/repository/loket_repository_test.go`
- Modify: `apps/api/internal/router/router.go`

**Interfaces:**
- Consumes: `cfg.MPP.CompanyID`.
- Produces:
  - `loket.Initialize(db *pgxpool.Pool, companyID string) *Module` with `Handler`, `Service`, `Repository`.
  - `repository.LoketRepository` methods reused by `loket_ops` (Task 17) and `antrian` (Task 13):
    `FindByInstansi(ctx, instansiID string) ([]domain.Loket, error)`,
    `FindByID(ctx, id string) (*domain.Loket, error)`,
    `ServedLayananIDs(ctx, loketID string) ([]string, error)`,
    `CountOpenForLayanan(ctx, layananID string) (int, error)`,
    `TouchIdle(ctx context.Context, tx pgx.Tx, loketID string) error`.
  - Route `GET /mpp/v1/loket?instansi_id=…` guarded by `JWTAuth()` + `RequirePermission("mpp.loket:read")`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/modules/mpp/loket/repository/loket_repository_test.go`:

```go
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

func TestFindByInstansiReturnsOpenLokets(t *testing.T) {
	pool := testutil.RequireDB(t)
	repo := repository.NewLoketRepository(pool, companyID)

	list, err := repo.FindByInstansi(context.Background(), instansiID)
	if err != nil {
		t.Fatalf("FindByInstansi: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 seeded lokets, got %d", len(list))
	}
	if list[2].Name == nil || *list[2].Name != "Loket 3" {
		t.Errorf("third loket = %+v, want name 'Loket 3'", list[2])
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
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/loket/...`
Expected: FAIL — undefined `repository.NewLoketRepository`.

- [ ] **Step 3: Write the domain struct**

Create `apps/api/internal/modules/mpp/loket/domain/loket.go`:

```go
package domain

import "time"

// Loket is a physical service counter belonging to an agency.
type Loket struct {
	ID         string
	InstansiID string
	Code       string
	Name       *string
	Status     string // OPEN | CLOSED | BREAK
	LastIdleAt *time.Time
	IsActive   bool
}

// DisplayName prefers the human name and falls back to the code, so the
// TV and TTS never announce an empty counter.
func (l *Loket) DisplayName() string {
	if l.Name != nil && *l.Name != "" {
		return *l.Name
	}
	return l.Code
}
```

- [ ] **Step 4: Write the repository**

Create `apps/api/internal/modules/mpp/loket/repository/loket.repository.go`:

```go
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/domain"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

type LoketRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewLoketRepository(db *pgxpool.Pool, companyID string) *LoketRepository {
	return &LoketRepository{db: db, companyID: companyID}
}

// FindByInstansi lists an agency's active lokets ordered by code.
func (r *LoketRepository) FindByInstansi(ctx context.Context, instansiID string) ([]domain.Loket, error) {
	query := `
		SELECT l.id, l.instansi_id, l.code, l.name, l.status, l.last_idle_at, l.is_active
		FROM mpp.loket l
		JOIN mpp.instansi i ON i.id = l.instansi_id AND i.deleted_at IS NULL
		WHERE l.instansi_id = $1 AND i.company_id = $2
		  AND l.deleted_at IS NULL AND l.is_active = TRUE
		ORDER BY l.code ASC`

	rows, err := r.db.Query(ctx, query, instansiID, r.companyID)
	if err != nil {
		logger.Error("Failed to list loket", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Loket, 0)
	for rows.Next() {
		var l domain.Loket
		if err := rows.Scan(&l.ID, &l.InstansiID, &l.Code, &l.Name, &l.Status, &l.LastIdleAt, &l.IsActive); err != nil {
			return nil, err
		}
		list = append(list, l)
	}

	return list, rows.Err()
}

// FindByID returns one loket inside the tenant, or (nil, nil).
func (r *LoketRepository) FindByID(ctx context.Context, id string) (*domain.Loket, error) {
	query := `
		SELECT l.id, l.instansi_id, l.code, l.name, l.status, l.last_idle_at, l.is_active
		FROM mpp.loket l
		JOIN mpp.instansi i ON i.id = l.instansi_id AND i.deleted_at IS NULL
		WHERE l.id = $1 AND i.company_id = $2 AND l.deleted_at IS NULL`

	var l domain.Loket
	err := r.db.QueryRow(ctx, query, id, r.companyID).Scan(
		&l.ID, &l.InstansiID, &l.Code, &l.Name, &l.Status, &l.LastIdleAt, &l.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find loket", logger.Err(err))
		return nil, err
	}

	return &l, nil
}

// ServedLayananIDs returns the services this loket may call from.
func (r *LoketRepository) ServedLayananIDs(ctx context.Context, loketID string) ([]string, error) {
	query := `
		SELECT ll.jenis_layanan_id
		FROM mpp.loket_layanan ll
		JOIN mpp.jenis_layanan l ON l.id = ll.jenis_layanan_id
		WHERE ll.loket_id = $1 AND l.deleted_at IS NULL AND l.is_active = TRUE`

	rows, err := r.db.Query(ctx, query, loketID)
	if err != nil {
		logger.Error("Failed to list loket services", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// CountOpenForLayanan counts OPEN lokets eligible for a service — the
// n_loket term of the ETA formula (BR-29).
func (r *LoketRepository) CountOpenForLayanan(ctx context.Context, layananID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM mpp.loket l
		JOIN mpp.loket_layanan ll ON ll.loket_id = l.id
		WHERE ll.jenis_layanan_id = $1
		  AND l.status = 'OPEN' AND l.is_active = TRUE AND l.deleted_at IS NULL`

	var n int
	if err := r.db.QueryRow(ctx, query, layananID).Scan(&n); err != nil {
		logger.Error("Failed to count open lokets", logger.Err(err))
		return 0, err
	}

	return n, nil
}

// TouchIdle refreshes last_idle_at so the idle-longest ordering (BR-12)
// stays fair. Called inside the transaction that frees the loket.
func (r *LoketRepository) TouchIdle(ctx context.Context, tx pgx.Tx, loketID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE mpp.loket SET last_idle_at = NOW(), updated_at = NOW() WHERE id = $1`, loketID)
	if err != nil {
		logger.Error("Failed to refresh loket idle", logger.Err(err))
	}
	return err
}
```

- [ ] **Step 5: Run the test to green**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/loket/... -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Write DTO, service, handler and module**

Create `apps/api/internal/modules/mpp/loket/dto/loket.dto.go`:

```go
package dto

// LoketQuery is the filter for GET /loket.
type LoketQuery struct {
	InstansiID string `form:"instansi_id" binding:"required,uuid"`
}

// LoketResponse is the staff-facing counter payload.
type LoketResponse struct {
	ID         string `json:"id"`
	InstansiID string `json:"instansi_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	IsActive   bool   `json:"is_active"`
}
```

Create `apps/api/internal/modules/mpp/loket/service/loket.service.go`:

```go
package service

import (
	"context"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
)

type LoketService struct {
	repo *repository.LoketRepository
}

func NewLoketService(repo *repository.LoketRepository) *LoketService {
	return &LoketService{repo: repo}
}

func (s *LoketService) ListByInstansi(ctx context.Context, instansiID string) ([]dto.LoketResponse, error) {
	list, err := s.repo.FindByInstansi(ctx, instansiID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.LoketResponse, 0, len(list))
	for i := range list {
		out = append(out, dto.LoketResponse{
			ID:         list[i].ID,
			InstansiID: list[i].InstansiID,
			Code:       list[i].Code,
			Name:       list[i].DisplayName(),
			Status:     list[i].Status,
			IsActive:   list[i].IsActive,
		})
	}

	return out, nil
}
```

Create `apps/api/internal/modules/mpp/loket/handler/loket.handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type LoketHandler struct {
	loketService *service.LoketService
}

func NewLoketHandler(s *service.LoketService) *LoketHandler {
	return &LoketHandler{loketService: s}
}

func (h *LoketHandler) List(c *gin.Context) {
	var query dto.LoketQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	result, err := h.loketService.ListByInstansi(c.Request.Context(), query.InstansiID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list loket", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Loket retrieved successfully", result)
}
```

Create `apps/api/internal/modules/mpp/loket/main.loket.go`:

```go
package loket

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket/service"
)

type Module struct {
	Handler    *handler.LoketHandler
	Service    *service.LoketService
	Repository *repository.LoketRepository
}

func Initialize(db *pgxpool.Pool, companyID string) *Module {
	repo := repository.NewLoketRepository(db, companyID)
	svc := service.NewLoketService(repo)

	return &Module{
		Handler:    handler.NewLoketHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers staff-only loket reads.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/loket", middleware.JWTAuth(), middleware.RequirePermission("mpp.loket:read"), m.Handler.List)
}
```

- [ ] **Step 7: Wire into the router**

In `internal/router/router.go`, import `"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket"` and add inside the `mppV1` block, after the instansi module:

```go
		loketModule := loket.Initialize(db, cfg.MPP.CompanyID)
		loketModule.SetupRoutes(mppV1)
```

- [ ] **Step 8: Verify build + auth guard**

Run:

```bash
cd apps/api && go build ./...
curl -s -o /dev/null -w '%{http_code}\n' "http://localhost:8080/mpp/v1/loket?instansi_id=a2000000-0000-0000-0000-000000000001"
```

Expected: build succeeds; the unauthenticated curl prints `401`.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/modules/mpp/loket apps/api/internal/router/router.go
git commit -m "feat(mpp): add loket read module guarded by mpp.loket:read"
```

---

### Task 4: MPP staff users, device roles and scoped device API keys (P3 + P5)

**Files:**
- Modify: `apps/api/internal/database/seeders/mpp/001_roles.sql`
- Create: `apps/api/internal/database/seeders/mpp/008_staff_users.sql`
- Create: `apps/api/internal/database/seeders/mpp/009_device_keys.sql`
- Modify: `docs/prompt/00-prerequisites.md` (record the minted demo keys)

**Interfaces:**
- Consumes: seeded MPP company/instansi/loket ids; core `roles`, `users`, `company_users`, `user_roles`, `api_keys` tables.
- Produces (fixed demo credentials used by every later smoke test):
  - staff logins `petugas@mpp.test` / `Petugas2026*`, `fo@mpp.test` / `Petugas2026*`, `supervisor@mpp.test` / `Petugas2026*`, `adminmpp@mpp.test` / `Petugas2026*` — all members of the MPP company with `is_primary = TRUE`.
  - kiosk key `wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90`
  - tv key `wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978`

- [ ] **Step 1: Append the two device roles**

Append to `apps/api/internal/database/seeders/mpp/001_roles.sql` (levels taken verbatim from the `kiosk` and `tv` columns of `docs/06-security/rbac-matrix.md`):

```sql
-- =====================================================
-- DEVICE ROLES (API-key owners)
-- =====================================================
-- API keys resolve their effective permissions as the INTERSECTION of the
-- key's scoped_permissions with the OWNING USER's permissions
-- (ApiKey.GetEffectivePermissions). Devices therefore need their own
-- users holding these roles — the super_admin role stores '{}' and would
-- intersect to nothing.
INSERT INTO core.roles (id, code, name, description, permissions, is_system, is_active) VALUES
(
    'a0000000-0000-0000-0000-000000000005',
    'mpp_device_kiosk',
    'MPP Device — Kiosk',
    'Self-service kiosk: QR check-in, walk-in registration, catalog reads.',
    '{
        "mpp.instansi": "viewer",
        "mpp.layanan": "viewer",
        "mpp.booking": "editor",
        "mpp.checkin": "editor"
    }'::jsonb,
    TRUE,
    TRUE
),
(
    'a0000000-0000-0000-0000-000000000006',
    'mpp_device_tv',
    'MPP Device — TV Display',
    'TV display: read the queue snapshot and subscribe to display events.',
    '{
        "mpp.queue": "viewer",
        "mpp.antrian": "viewer",
        "mpp.display": "viewer"
    }'::jsonb,
    TRUE,
    TRUE
)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Seed the staff users**

Create `apps/api/internal/database/seeders/mpp/008_staff_users.sql`:

```sql
-- =====================================================
-- MPP SEEDER - STAFF USERS
-- =====================================================
-- Seeder 008: one demo user per MPP role, all members of the MPP tenant
-- company with is_primary = TRUE so signin puts company_id in the JWT
-- (auth service resolves it via companyUserRepo.GetPrimaryCompany).
--
-- Password for every account below: Petugas2026*
-- =====================================================

INSERT INTO core.users (id, email, username, password_hash, full_name, phone, is_active, is_email_verified, email_verified_at) VALUES
('a9000000-0000-0000-0000-000000000001', 'adminmpp@mpp.test',   'adminmpp',   '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'Admin MPP',        '+6281234500001', TRUE, TRUE, NOW()),
('a9000000-0000-0000-0000-000000000002', 'supervisor@mpp.test', 'supervisor', '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'Supervisor MPP',   '+6281234500002', TRUE, TRUE, NOW()),
('a9000000-0000-0000-0000-000000000003', 'fo@mpp.test',         'frontoffice','$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'Front Office MPP', '+6281234500003', TRUE, TRUE, NOW()),
('a9000000-0000-0000-0000-000000000004', 'petugas@mpp.test',    'petugas',    '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'Pak Budi',         '+6281234500004', TRUE, TRUE, NOW())
ON CONFLICT DO NOTHING;

-- Membership in the MPP building (role_id also feeds GetUserRoles, so the
-- MPP role applies inside this company).
INSERT INTO core.company_users (id, company_id, user_id, role_id, is_primary, is_active, joined_at) VALUES
('a9100000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', TRUE, TRUE, NOW()),
('a9100000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000002', TRUE, TRUE, NOW()),
('a9100000-0000-0000-0000-000000000003', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000003', TRUE, TRUE, NOW()),
('a9100000-0000-0000-0000-000000000004', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000004', TRUE, TRUE, NOW())
ON CONFLICT DO NOTHING;

-- Explicit company-scoped role assignment (belt and braces: permission
-- lookups accept either core.user_roles or core.company_users.role_id).
INSERT INTO core.user_roles (user_id, role_id, company_id) VALUES
('a9000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001'),
('a9000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000001'),
('a9000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000003', 'a1000000-0000-0000-0000-000000000001'),
('a9000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000004', 'a1000000-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Seed the device users and API keys**

Create `apps/api/internal/database/seeders/mpp/009_device_keys.sql`:

```sql
-- =====================================================
-- MPP SEEDER - DEVICE USERS & API KEYS
-- =====================================================
-- Seeder 009: kiosk and TV devices authenticate with scoped API keys
-- (X-API-Key), never a user JWT. The middleware parses the key as
-- wiz_<env>_<key_id>_<secret> and compares SHA256(secret) to secret_hash.
--
-- DEMO KEYS (development only — rotate before any real deployment):
--   kiosk: wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90
--   tv:    wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978
-- =====================================================

INSERT INTO core.users (id, email, username, password_hash, full_name, is_active, is_email_verified, email_verified_at) VALUES
('a9000000-0000-0000-0000-000000000011', 'kiosk@mpp.device', 'mpp-kiosk', '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'MPP Kiosk Device', TRUE, TRUE, NOW()),
('a9000000-0000-0000-0000-000000000012', 'tv@mpp.device',    'mpp-tv',    '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', 'MPP TV Device',    TRUE, TRUE, NOW())
ON CONFLICT DO NOTHING;

INSERT INTO core.company_users (id, company_id, user_id, role_id, is_primary, is_active, joined_at) VALUES
('a9100000-0000-0000-0000-000000000011', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000005', TRUE, TRUE, NOW()),
('a9100000-0000-0000-0000-000000000012', 'a1000000-0000-0000-0000-000000000001', 'a9000000-0000-0000-0000-000000000012', 'a0000000-0000-0000-0000-000000000006', TRUE, TRUE, NOW())
ON CONFLICT DO NOTHING;

INSERT INTO core.user_roles (user_id, role_id, company_id) VALUES
('a9000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000005', 'a1000000-0000-0000-0000-000000000001'),
('a9000000-0000-0000-0000-000000000012', 'a0000000-0000-0000-0000-000000000006', 'a1000000-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;

-- secret_hash is SHA256(secret) in lowercase hex — exactly what
-- api_key.service.hashSecret produces. sha256() is a Postgres builtin
-- (PG 11+), so no pgcrypto extension is required.
INSERT INTO core.api_keys (
    id, user_id, company_id, key_id, secret_hash, key_prefix,
    name, description, environment, scoped_permissions, rate_limit, rate_limit_window
) VALUES
(
    'a9200000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000011',
    'a1000000-0000-0000-0000-000000000001',
    'kiosk001',
    encode(sha256('a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90'::bytea), 'hex'),
    'wiz_test_kiosk001_a1b2c3d4...',
    'MPP Kiosk',
    'Self-service kiosk: check-in + walk-in + catalog reads.',
    'test',
    '["mpp.checkin:create","mpp.booking:create","mpp.layanan:read","mpp.instansi:read"]'::jsonb,
    10000,
    3600
),
(
    'a9200000-0000-0000-0000-000000000002',
    'a9000000-0000-0000-0000-000000000012',
    'a1000000-0000-0000-0000-000000000001',
    'tvdsp001',
    encode(sha256('f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978'::bytea), 'hex'),
    'wiz_test_tvdsp001_f0e1d2c3...',
    'MPP TV Display',
    'TV display snapshot + WebSocket subscribe.',
    'test',
    '["mpp.display:read","mpp.queue:read"]'::jsonb,
    10000,
    3600
)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 4: Run the seeders**

Run: `cd apps/api && make seed-mpp && make seed-mpp`
Expected: both runs print `✅ MPP seeders completed!` with no error — proving idempotency (`ON CONFLICT DO NOTHING`).

- [ ] **Step 5: Prove the staff login and the device key both work**

Run:

```bash
STAFF=$(curl -s -X POST http://localhost:8080/core/v1/auth/signin \
  -H 'Content-Type: application/json' \
  -d '{"login":"petugas@mpp.test","password":"Petugas2026*"}' | jq -r .data.access_token)
echo "token len: ${#STAFF}"

KIOSK_KEY=wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90
curl -s -o /dev/null -w 'kiosk on loket-read (expect 403): %{http_code}\n' \
  "http://localhost:8080/mpp/v1/loket?instansi_id=a2000000-0000-0000-0000-000000000001" \
  -H "X-API-Key: $KIOSK_KEY"
curl -s -o /dev/null -w 'staff on loket-read (expect 200): %{http_code}\n' \
  "http://localhost:8080/mpp/v1/loket?instansi_id=a2000000-0000-0000-0000-000000000001" \
  -H "Authorization: Bearer $STAFF"
```

Expected: a non-empty token; `403` for the kiosk key (it has no `mpp.loket:read`); `200` for the petugas token. Paste the output.

- [ ] **Step 6: Record the demo credentials in the prerequisites doc**

Append to `docs/prompt/00-prerequisites.md`, at the end of the **P5** section:

```markdown
**Seeded demo keys** (`seeders/mpp/009_device_keys.sql`, development only — rotate before deployment):

| Device | Key |
|---|---|
| kiosk | `wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90` |
| tv | `wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978` |

Device keys are owned by dedicated users (`kiosk@mpp.device`, `tv@mpp.device`) holding the
`mpp_device_kiosk` / `mpp_device_tv` roles, because a key's effective permissions are the
intersection of its scope with the owning user's permissions.

**Seeded staff logins** (`seeders/mpp/008_staff_users.sql`, password `Petugas2026*`):
`adminmpp@mpp.test`, `supervisor@mpp.test`, `fo@mpp.test`, `petugas@mpp.test`.
```

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/database/seeders/mpp docs/prompt/00-prerequisites.md
git commit -m "feat(mpp): seed MPP staff users, device roles and scoped kiosk/TV API keys"
```

---

### Task 5: Frontend auth layer, device client and route groups (P6 + P7)

**Files:**
- Create: `apps/web/src/lib/api/token-store.ts`
- Create: `apps/web/src/lib/api/device-client.ts`
- Create: `apps/web/src/lib/api/auth.ts`
- Create: `apps/web/src/lib/api/use-auth.ts`
- Modify: `apps/web/src/lib/api/client.ts`
- Modify: `apps/web/src/lib/api/endpoints.ts`
- Modify: `apps/web/src/lib/api/index.ts`
- Modify: `apps/web/src/lib/env.ts`, `apps/web/src/global-config.ts`, `apps/web/.env.example`
- Modify: `apps/web/src/routes/paths.ts`
- Create: `apps/web/src/app/signin/page.tsx`, `apps/web/src/app/signin/layout.tsx`
- Create: `apps/web/src/sections/auth/view/signin-view.tsx`
- Create: `apps/web/src/app/(citizen)/layout.tsx`, `apps/web/src/app/(kiosk)/layout.tsx`, `apps/web/src/app/(loket)/layout.tsx`, `apps/web/src/app/(tv)/layout.tsx`

**Interfaces:**
- Consumes: existing `api`/`apiFetch` from `src/lib/api/client.ts`, `CONFIG`.
- Produces:
  - `tokenStore` — `{ get(): string | null; set(token: string, refresh?: string): void; clear(): void; getRefresh(): string | null }`.
  - `deviceFetch<T>(path, options, key: 'kiosk' | 'tv'): Promise<ApiEnvelope<T>>` (same option shape as `apiFetch`).
  - `signIn(login: string, password: string): Promise<SignInResult>` and `useSignInMutation()`.
  - `paths.auth.signin`, `paths.citizen.*`, `paths.kiosk.*`, `paths.loket.*`, `paths.tv.display(instansi)`.

- [ ] **Step 1: Add the device/WS env vars**

In `apps/web/src/lib/env.ts`, add to `rawEnv`:

```ts
  NEXT_PUBLIC_WS_URL: emptyToUndefined(process.env.NEXT_PUBLIC_WS_URL),
  NEXT_PUBLIC_KIOSK_API_KEY: emptyToUndefined(process.env.NEXT_PUBLIC_KIOSK_API_KEY),
  NEXT_PUBLIC_TV_API_KEY: emptyToUndefined(process.env.NEXT_PUBLIC_TV_API_KEY),
```

and to `schema`:

```ts
  // Optional: falls back to NEXT_PUBLIC_API_URL with http→ws in global-config.
  NEXT_PUBLIC_WS_URL: z.string().default(''),
  // Device keys are baked into the bundle at build time. That is acceptable
  // ONLY because kiosk/TV builds are deployed to locked-down on-site devices
  // and the keys are narrowly scoped; never reuse them for a public build.
  NEXT_PUBLIC_KIOSK_API_KEY: z.string().default(''),
  NEXT_PUBLIC_TV_API_KEY: z.string().default(''),
```

In `apps/web/src/global-config.ts`, add to the `CONFIG` object:

```ts
  /** WebSocket origin for the MPP queue feed. Derived from apiUrl when unset. */
  wsUrl: env.NEXT_PUBLIC_WS_URL || env.NEXT_PUBLIC_API_URL.replace(/^http/, 'ws'),
  /** Scoped device API keys (kiosk / TV builds only). */
  kioskApiKey: env.NEXT_PUBLIC_KIOSK_API_KEY,
  tvApiKey: env.NEXT_PUBLIC_TV_API_KEY,
```

Append to `apps/web/.env.example`:

```bash
# ── MPP ─────────────────────────────────────────────────────────────────────
# WebSocket origin for the queue feed. Leave empty to derive it from
# NEXT_PUBLIC_API_URL (http→ws).
NEXT_PUBLIC_WS_URL=

# Scoped device API keys — set ONLY in kiosk / TV device builds.
# Demo values from seeders/mpp/009_device_keys.sql:
NEXT_PUBLIC_KIOSK_API_KEY=wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90
NEXT_PUBLIC_TV_API_KEY=wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978
```

- [ ] **Step 2: Write the token store**

Create `apps/web/src/lib/api/token-store.ts`:

```ts
// ----------------------------------------------------------------------
// Staff token store (loket / FO / admin). In-memory is the source of
// truth so SSR and the first client render agree; localStorage only
// rehydrates it after a reload. Kiosk and TV never touch this — they
// carry a scoped X-API-Key instead (see device-client.ts).

const ACCESS_KEY = 'mpp.access_token';
const REFRESH_KEY = 'mpp.refresh_token';

let accessToken: string | null = null;
let refreshToken: string | null = null;
let hydrated = false;

function hydrate() {
  if (hydrated || typeof window === 'undefined') return;

  accessToken = window.localStorage.getItem(ACCESS_KEY);
  refreshToken = window.localStorage.getItem(REFRESH_KEY);
  hydrated = true;
}

export const tokenStore = {
  get(): string | null {
    hydrate();
    return accessToken;
  },
  getRefresh(): string | null {
    hydrate();
    return refreshToken;
  },
  set(access: string, refresh?: string) {
    accessToken = access;
    hydrated = true;

    if (typeof window !== 'undefined') {
      window.localStorage.setItem(ACCESS_KEY, access);
      if (refresh) window.localStorage.setItem(REFRESH_KEY, refresh);
    }
    if (refresh) refreshToken = refresh;
  },
  clear() {
    accessToken = null;
    refreshToken = null;
    hydrated = true;

    if (typeof window !== 'undefined') {
      window.localStorage.removeItem(ACCESS_KEY);
      window.localStorage.removeItem(REFRESH_KEY);
    }
  },
};
```

- [ ] **Step 3: Attach the token at the ky seam**

In `apps/web/src/lib/api/client.ts`, add the import `import { tokenStore } from './token-store';` and replace the `hooks`-less `ky.create({...})` call with the same options plus:

```ts
  hooks: {
    beforeRequest: [
      (request) => {
        // Server-side renders have no staff session; only the browser
        // ever carries a token.
        const token = typeof window === 'undefined' ? null : tokenStore.get();
        if (token && !request.headers.has('Authorization')) {
          request.headers.set('Authorization', `Bearer ${token}`);
        }
      },
    ],
    afterResponse: [
      (_request, _options, response) => {
        // A 401 means the staff session is gone. Drop the token and let
        // the caller's ApiError surface; route guards send the user to
        // /signin rather than this low-level hook doing navigation.
        if (response.status === 401 && typeof window !== 'undefined') {
          tokenStore.clear();
        }
        return response;
      },
    ],
  },
```

- [ ] **Step 4: Write the device client**

Create `apps/web/src/lib/api/device-client.ts`:

```ts
import type { ApiEnvelope, ApiFetchOptions } from './client';

import ky, { HTTPError, TimeoutError } from 'ky';

import { CONFIG } from 'src/global-config';

import { ApiError } from './client';

// ----------------------------------------------------------------------
// Kiosk and TV authenticate with a scoped API key, never the staff token,
// so they get their own ky instance — mixing them on one instance would
// leak a staff Authorization header onto device calls.

export type DeviceKind = 'kiosk' | 'tv';

function deviceKey(kind: DeviceKind) {
  return kind === 'kiosk' ? CONFIG.kioskApiKey : CONFIG.tvApiKey;
}

function baseUrl() {
  const raw = CONFIG.apiUrl;
  return raw ? `${raw.replace(/\/+$/, '')}/` : '';
}

const deviceApi = ky.create({
  baseUrl: baseUrl(),
  timeout: 10_000,
  retry: { limit: 2, methods: ['get'] },
});

function cleanParams(params: ApiFetchOptions['params']) {
  const search: Record<string, string> = {};

  for (const [key, value] of Object.entries(params ?? {})) {
    if (value !== undefined && value !== null && value !== '') {
      search[key] = String(value);
    }
  }

  return search;
}

/** Same contract as apiFetch, but authenticated with the device API key. */
export async function deviceFetch<T>(
  path: string,
  { params, method = 'get', body, ...options }: ApiFetchOptions = {},
  kind: DeviceKind = 'kiosk'
): Promise<ApiEnvelope<T>> {
  const key = deviceKey(kind);

  if (!key) {
    throw new ApiError(
      0,
      `Device API key is not configured — set NEXT_PUBLIC_${kind === 'kiosk' ? 'KIOSK' : 'TV'}_API_KEY in .env`
    );
  }

  try {
    return await deviceApi<ApiEnvelope<T>>(path, {
      method,
      headers: { 'X-API-Key': key },
      searchParams: cleanParams(params),
      ...(body !== undefined && { json: body }),
      ...options,
    }).json();
  } catch (error) {
    if (error instanceof HTTPError) {
      const payload = (await error.response.json().catch(() => null)) as ApiEnvelope<null> | null;
      throw new ApiError(
        error.response.status,
        payload?.message || error.message,
        payload?.errors ?? null
      );
    }
    if (error instanceof TimeoutError) {
      throw new ApiError(0, `Request timeout: ${path}`);
    }
    if (error instanceof TypeError) {
      throw new ApiError(0, `Network error: ${error.message}`);
    }
    throw error;
  }
}
```

- [ ] **Step 5: Add endpoints and the auth api-layer trio**

In `apps/web/src/lib/api/endpoints.ts`, add to the exported object:

```ts
  core: {
    auth: {
      signin: 'core/v1/auth/signin',
      me: 'core/v1/auth/me',
    },
  },
  mpp: {
    instansi: {
      list: 'mpp/v1/instansi',
      detail: (id: string) => `mpp/v1/instansi/${encodeURIComponent(id)}`,
      layanan: (id: string) => `mpp/v1/instansi/${encodeURIComponent(id)}/layanan`,
    },
    loket: 'mpp/v1/loket',
  },
```

Create `apps/web/src/lib/api/auth.ts`:

```ts
import { z } from 'zod';

import { apiFetch } from './client';
import { endpoints } from './endpoints';
import { tokenStore } from './token-store';

// ----------------------------------------------------------------------
// Mirrors core/auth SignInResponse (apps/api/internal/modules/core/auth/dto).

export const signInResultSchema = z.object({
  access_token: z.string(),
  refresh_token: z.string(),
  token_type: z.string(),
  expires_in: z.number(),
  user: z.object({
    id: z.string(),
    email: z.string(),
    username: z.string(),
    full_name: z.string().nullish(),
  }),
  company: z.object({ id: z.string(), name: z.string() }).nullish(),
  roles: z
    .array(z.string())
    .nullish()
    .transform((v) => v ?? []),
});

export type SignInResult = z.infer<typeof signInResultSchema>;

export const authKeys = {
  me: ['auth', 'me'] as const,
};

export async function signIn(login: string, password: string): Promise<SignInResult> {
  const { data } = await apiFetch<unknown>(endpoints.core.auth.signin, {
    method: 'post',
    body: { login, password },
  });

  const result = signInResultSchema.parse(data);
  tokenStore.set(result.access_token, result.refresh_token);

  return result;
}

export function signOut() {
  tokenStore.clear();
}
```

Create `apps/web/src/lib/api/use-auth.ts`:

```ts
'use client';

import { useMutation } from '@tanstack/react-query';

import { signIn } from './auth';

// ----------------------------------------------------------------------

export function useSignInMutation() {
  return useMutation({
    mutationFn: ({ login, password }: { login: string; password: string }) =>
      signIn(login, password),
  });
}
```

In `apps/web/src/lib/api/index.ts`, add `export * from './auth';`, `export * from './token-store';` and `export * from './device-client';` (hooks modules stay out of the barrel).

- [ ] **Step 6: Add every new route to `paths.ts`**

In `apps/web/src/routes/paths.ts`, add inside the `paths` object:

```ts
  /**
   * Auth (staff: loket / FO / admin)
   */
  auth: {
    signin: '/signin',
  },
  /**
   * MPP — citizen
   */
  citizen: {
    daftar: '/daftar',
    status: '/status',
    booking: {
      detail: (id: string) => `/booking/${id}`,
    },
  },
  /**
   * MPP — kiosk (device, API-key)
   */
  kiosk: {
    root: '/kiosk',
    checkin: '/kiosk/checkin',
    walkin: '/kiosk/walkin',
  },
  /**
   * MPP — loket (staff JWT)
   */
  loket: {
    root: '/loket',
  },
  /**
   * MPP — TV display (device, API-key)
   */
  tv: {
    display: (instansi: string) => `/display/${instansi}`,
  },
```

- [ ] **Step 7: Create the four route-group layouts**

Create `apps/web/src/app/(citizen)/layout.tsx`:

```tsx
import { MainLayout } from 'src/layouts/main';

export default function CitizenLayout({ children }: { children: React.ReactNode }) {
  return <MainLayout>{children}</MainLayout>;
}
```

Create `apps/web/src/app/(kiosk)/layout.tsx` (full-screen, touch-first, high contrast — no site chrome):

```tsx
import Box from '@mui/material/Box';

export default function KioskLayout({ children }: { children: React.ReactNode }) {
  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        bgcolor: 'background.default',
        px: 4,
        py: 6,
        // Touch targets and type scale for a standing kiosk.
        '& .MuiButton-root': { minHeight: 72, fontSize: 24 },
      }}
    >
      {children}
    </Box>
  );
}
```

Create `apps/web/src/app/(loket)/layout.tsx`:

```tsx
import Box from '@mui/material/Box';

export default function LoketLayout({ children }: { children: React.ReactNode }) {
  return <Box sx={{ minHeight: '100vh', p: 3, bgcolor: 'background.default' }}>{children}</Box>;
}
```

Create `apps/web/src/app/(tv)/layout.tsx` (legible from a distance, NFR-UX-02):

```tsx
import Box from '@mui/material/Box';

export default function TvLayout({ children }: { children: React.ReactNode }) {
  return (
    <Box
      sx={{
        minHeight: '100vh',
        bgcolor: 'common.black',
        color: 'common.white',
        overflow: 'hidden',
      }}
    >
      {children}
    </Box>
  );
}
```

- [ ] **Step 8: Build the signin page (page → view)**

Create `apps/web/src/sections/auth/view/signin-view.tsx`:

```tsx
'use client';

import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { useRouter } from 'src/routes/hooks';

import { useSignInMutation } from 'src/lib/api/use-auth';

import { Form, Field } from 'src/components/hook-form';

// ----------------------------------------------------------------------

const SignInSchema = z.object({
  login: z.string().min(1, { message: 'Email atau username wajib diisi' }),
  password: z.string().min(8, { message: 'Kata sandi minimal 8 karakter' }),
});

type SignInValues = z.infer<typeof SignInSchema>;

export function SignInView() {
  const router = useRouter();
  const signInMutation = useSignInMutation();

  const methods = useForm<SignInValues>({
    resolver: zodResolver(SignInSchema),
    defaultValues: { login: '', password: '' },
  });

  const onSubmit = methods.handleSubmit(async (values) => {
    await signInMutation.mutateAsync(values);
    router.push(paths.loket.root);
  });

  return (
    <Box sx={{ maxWidth: 420, mx: 'auto', py: 8 }}>
      <Card sx={{ p: 4 }}>
        <Typography variant="h4" sx={{ mb: 3 }}>
          Masuk Petugas
        </Typography>

        {signInMutation.isError && (
          <Alert severity="error" sx={{ mb: 3 }}>
            {(signInMutation.error as Error).message}
          </Alert>
        )}

        <Form methods={methods} onSubmit={onSubmit}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
            <Field.Text name="login" label="Email atau username" />
            <Field.Text name="password" label="Kata sandi" type="password" />
            <Button type="submit" variant="contained" size="large" disabled={signInMutation.isPending}>
              {signInMutation.isPending ? 'Memproses…' : 'Masuk'}
            </Button>
          </Box>
        </Form>
      </Card>
    </Box>
  );
}
```

Create `apps/web/src/app/signin/page.tsx`:

```tsx
import type { Metadata } from 'next';

import { SignInView } from 'src/sections/auth/view/signin-view';

export const metadata: Metadata = { title: 'Masuk Petugas' };

export default function Page() {
  return <SignInView />;
}
```

Create `apps/web/src/app/signin/layout.tsx`:

```tsx
import { SimpleLayout } from 'src/layouts/simple';

export default function SignInLayout({ children }: { children: React.ReactNode }) {
  return <SimpleLayout>{children}</SimpleLayout>;
}
```

- [ ] **Step 9: Run the frontend gate**

Run: `cd apps/web && yarn tsc:check && yarn lint`
Expected: both clean. If `SimpleLayout`/`MainLayout` import paths differ, fix them by checking `src/layouts/` barrels — do not silence the type error.

- [ ] **Step 10: Manual check**

With `yarn dev` running and the API up: open `http://localhost:8002/signin`, sign in as `petugas@mpp.test` / `Petugas2026*`. Expected: navigation to `/loket` (404 page for now — the route lands in Task 18) and `mpp.access_token` present in `localStorage`.

- [ ] **Step 11: Commit**

```bash
git add apps/web/src/lib/api apps/web/src/routes/paths.ts apps/web/src/lib/env.ts \
        apps/web/src/global-config.ts apps/web/.env.example apps/web/src/app apps/web/src/sections/auth
git commit -m "feat(web): add staff auth layer, device API-key client and MPP route groups"
```

---

## Phase 1 — Slice 01: Pendaftaran (`docs/prompt/01-pendaftaran.md`)

### Task 6: `mpp/kuota` — availability + atomic quota consume

**Files:**
- Create: `apps/api/internal/modules/mpp/kuota/{domain/kuota.go,dto/kuota.dto.go,repository/kuota.repository.go,service/kuota.service.go,handler/kuota.handler.go,main.kuota.go}`
- Test: `apps/api/internal/modules/mpp/kuota/repository/kuota_repository_test.go`
- Modify: `apps/api/internal/router/router.go`

**Interfaces:**
- Consumes: `cfg.MPP.CompanyID`, `cfg.MPP.Location`.
- Produces:
  - `kuota.Initialize(db *pgxpool.Pool, companyID string, loc *time.Location) *Module` with `Handler`, `Service`, `Repository`.
  - `repository.ErrQuotaFull` (sentinel, re-exported by the booking service).
  - `repository.KuotaRepository`:
    `FindSlot(ctx, instansiID string, layananID *string, date time.Time) (*domain.Slot, error)` — per-service row first, agency-wide fallback, `(nil, nil)` when neither exists;
    `Consume(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error` — `ErrQuotaFull` when nothing could be reserved;
    `Release(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error` — the exact inverse, for cancel (BR-07).
  - Route `GET /mpp/v1/availability?instansi_id&layanan_id&date` (public).

- [ ] **Step 1: Write the failing repository tests (incl. the concurrency proof)**

Create `apps/api/internal/modules/mpp/kuota/repository/kuota_repository_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
)

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func TestFindSlotFallsBackToAgencyWideRow(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	repo := repository.NewKuotaRepository(pool, companyID)
	layananID := "a3000000-0000-0000-0000-000000000002" // no per-service quota row seeded

	slot, err := repo.FindSlot(context.Background(), instansiID, &layananID, today())
	if err != nil {
		t.Fatalf("FindSlot: %v", err)
	}
	if slot == nil {
		t.Fatal("want the agency-wide slot, got nil")
	}
	if slot.Kuota != 30 || slot.Terpakai != 0 {
		t.Fatalf("slot = %+v, want kuota 30 / terpakai 0", slot)
	}
	if slot.JenisLayananID != nil {
		t.Errorf("want an agency-wide slot (NULL service), got %v", *slot.JenisLayananID)
	}
}

func TestConsumeIncrementsThenRejectsWhenFull(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	ctx := context.Background()
	day := today()

	if _, err := pool.Exec(ctx, `
		UPDATE mpp.kuota_booking SET kuota = 1, terpakai = 0
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, day); err != nil {
		t.Fatalf("shrink quota: %v", err)
	}

	repo := repository.NewKuotaRepository(pool, companyID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := repo.Consume(ctx, tx, instansiID, nil, day); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	if err := repo.Consume(ctx, tx, instansiID, nil, day); !errors.Is(err, repository.ErrQuotaFull) {
		t.Fatalf("second Consume err = %v, want ErrQuotaFull", err)
	}
	_ = tx.Rollback(ctx)
}

// TestConsumeIsRaceFree is the NFR-DATA-02 proof: N concurrent consumers
// against a 1-seat quota must produce exactly one winner and terpakai == 1.
func TestConsumeIsRaceFree(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	ctx := context.Background()
	day := today()

	if _, err := pool.Exec(ctx, `
		UPDATE mpp.kuota_booking SET kuota = 1, terpakai = 0
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, day); err != nil {
		t.Fatalf("shrink quota: %v", err)
	}

	repo := repository.NewKuotaRepository(pool, companyID)

	const n = 20
	var wg sync.WaitGroup
	results := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			tx, err := pool.Begin(ctx)
			if err != nil {
				results <- err
				return
			}
			if err := repo.Consume(ctx, tx, instansiID, nil, day); err != nil {
				_ = tx.Rollback(ctx)
				results <- err
				return
			}
			results <- tx.Commit(ctx)
		}()
	}

	wg.Wait()
	close(results)

	var ok, full int
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, repository.ErrQuotaFull):
			full++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if ok != 1 || full != n-1 {
		t.Fatalf("winners = %d, quota-full = %d; want 1 and %d", ok, full, n-1)
	}

	var terpakai int
	if err := pool.QueryRow(ctx, `
		SELECT terpakai FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2`,
		instansiID, day).Scan(&terpakai); err != nil {
		t.Fatalf("read terpakai: %v", err)
	}
	if terpakai != 1 {
		t.Fatalf("terpakai = %d, want 1 (overbooking!)", terpakai)
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/kuota/...`
Expected: FAIL — undefined `repository.NewKuotaRepository` / `repository.ErrQuotaFull`.

- [ ] **Step 3: Write the domain struct**

Create `apps/api/internal/modules/mpp/kuota/domain/kuota.go`:

```go
package domain

import "time"

// Slot is one quota row: a date, for an agency, optionally narrowed to a
// service. JenisLayananID nil means the agency-wide row.
type Slot struct {
	ID             string
	InstansiID     string
	JenisLayananID *string
	Tanggal        time.Time
	Kuota          int
	Terpakai       int
}

// Remaining never reports a negative number.
func (s *Slot) Remaining() int {
	if s.Terpakai >= s.Kuota {
		return 0
	}
	return s.Kuota - s.Terpakai
}
```

- [ ] **Step 4: Write the repository — the atomic consume is the crux**

Create `apps/api/internal/modules/mpp/kuota/repository/kuota.repository.go`:

```go
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

// ErrQuotaFull means no seat could be reserved for that agency/service/date
// — either the row is exhausted or no quota was ever configured. Both are
// a 409 to the caller (BR-05).
var ErrQuotaFull = errors.New("quota full")

type KuotaRepository struct {
	db        *pgxpool.Pool
	companyID string
}

func NewKuotaRepository(db *pgxpool.Pool, companyID string) *KuotaRepository {
	return &KuotaRepository{db: db, companyID: companyID}
}

const slotColumns = `id, instansi_id, jenis_layanan_id, tanggal, kuota, terpakai`

// FindSlot resolves the quota row that governs a booking: the per-service
// row wins when it exists, otherwise the agency-wide row. Returns
// (nil, nil) when neither exists — the caller reports remaining 0.
func (r *KuotaRepository) FindSlot(ctx context.Context, instansiID string, layananID *string, date time.Time) (*domain.Slot, error) {
	if layananID != nil {
		slot, err := r.findOne(ctx, instansiID, layananID, date)
		if err != nil {
			return nil, err
		}
		if slot != nil {
			return slot, nil
		}
	}

	return r.findOne(ctx, instansiID, nil, date)
}

func (r *KuotaRepository) findOne(ctx context.Context, instansiID string, layananID *string, date time.Time) (*domain.Slot, error) {
	query := `
		SELECT ` + slotColumns + `
		FROM mpp.kuota_booking k
		WHERE k.instansi_id = $1
		  AND k.tanggal = $2
		  AND (($3::uuid IS NOT NULL AND k.jenis_layanan_id = $3)
		    OR ($3::uuid IS NULL AND k.jenis_layanan_id IS NULL))
		  AND EXISTS (
		      SELECT 1 FROM mpp.instansi i
		      WHERE i.id = k.instansi_id AND i.company_id = $4 AND i.deleted_at IS NULL
		  )`

	var s domain.Slot
	err := r.db.QueryRow(ctx, query, instansiID, date, layananID, r.companyID).Scan(
		&s.ID, &s.InstansiID, &s.JenisLayananID, &s.Tanggal, &s.Kuota, &s.Terpakai,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find kuota slot", logger.Err(err))
		return nil, err
	}

	return &s, nil
}

// Consume reserves one seat atomically inside the caller's transaction.
//
// The `terpakai < kuota` guard plus the row lock the UPDATE takes is what
// makes concurrent bookings safe: a losing writer blocks on the lock, then
// re-evaluates the guard against the committed value and matches 0 rows
// (BR-06 / NFR-DATA-02). Precedence: per-service row first; fall back to
// the agency-wide row only when no per-service row exists at all.
func (r *KuotaRepository) Consume(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error {
	if layananID != nil {
		reserved, err := r.consumeRow(ctx, tx, instansiID, layananID, date)
		if err != nil {
			return err
		}
		if reserved {
			return nil
		}

		exists, err := r.rowExists(ctx, tx, instansiID, layananID, date)
		if err != nil {
			return err
		}
		if exists {
			// A per-service row exists but is exhausted — do NOT silently
			// spend the agency-wide allowance behind the admin's back.
			return ErrQuotaFull
		}
	}

	reserved, err := r.consumeRow(ctx, tx, instansiID, nil, date)
	if err != nil {
		return err
	}
	if !reserved {
		return ErrQuotaFull
	}

	return nil
}

func (r *KuotaRepository) consumeRow(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) (bool, error) {
	query := `
		UPDATE mpp.kuota_booking
		SET terpakai = terpakai + 1, updated_at = NOW()
		WHERE instansi_id = $1
		  AND tanggal = $2
		  AND (($3::uuid IS NOT NULL AND jenis_layanan_id = $3)
		    OR ($3::uuid IS NULL AND jenis_layanan_id IS NULL))
		  AND terpakai < kuota
		RETURNING id`

	var id string
	err := tx.QueryRow(ctx, query, instansiID, date, layananID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		logger.Error("Failed to consume kuota", logger.Err(err))
		return false, err
	}

	return true, nil
}

func (r *KuotaRepository) rowExists(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM mpp.kuota_booking
			WHERE instansi_id = $1 AND tanggal = $2
			  AND (($3::uuid IS NOT NULL AND jenis_layanan_id = $3)
			    OR ($3::uuid IS NULL AND jenis_layanan_id IS NULL))
		)`

	var exists bool
	if err := tx.QueryRow(ctx, query, instansiID, date, layananID).Scan(&exists); err != nil {
		logger.Error("Failed to probe kuota row", logger.Err(err))
		return false, err
	}

	return exists, nil
}

// Release gives a seat back (BR-07, booking cancel before the cutoff).
// Clamped at zero so a double-cancel can never drive terpakai negative.
func (r *KuotaRepository) Release(ctx context.Context, tx pgx.Tx, instansiID string, layananID *string, date time.Time) error {
	query := `
		UPDATE mpp.kuota_booking
		SET terpakai = GREATEST(terpakai - 1, 0), updated_at = NOW()
		WHERE instansi_id = $1
		  AND tanggal = $2
		  AND (($3::uuid IS NOT NULL AND jenis_layanan_id = $3)
		    OR ($3::uuid IS NULL AND jenis_layanan_id IS NULL))`

	_, err := tx.Exec(ctx, query, instansiID, date, layananID)
	if err != nil {
		logger.Error("Failed to release kuota", logger.Err(err))
	}

	return err
}
```

- [ ] **Step 5: Run the tests to green**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/kuota/... -v -race`
Expected: PASS (3 tests), including `TestConsumeIsRaceFree`. Paste the output — this is the NFR-DATA-02 evidence.

- [ ] **Step 6: Write the DTO**

Create `apps/api/internal/modules/mpp/kuota/dto/kuota.dto.go`:

```go
package dto

// AvailabilityQuery is the public availability filter. `date` is a local
// calendar date (the citizen picks a day, not an instant).
type AvailabilityQuery struct {
	InstansiID string `form:"instansi_id" binding:"required,uuid"`
	LayananID  string `form:"layanan_id" binding:"omitempty,uuid"`
	Date       string `form:"date" binding:"required,datetime=2006-01-02"`
}

// AvailabilityResponse mirrors docs/prompt/01-pendaftaran.md exactly.
type AvailabilityResponse struct {
	Date      string `json:"date"`
	Kuota     int    `json:"kuota"`
	Terpakai  int    `json:"terpakai"`
	Remaining int    `json:"remaining"`
}
```

- [ ] **Step 7: Write the service**

Create `apps/api/internal/modules/mpp/kuota/service/kuota.service.go`:

```go
package service

import (
	"context"
	"time"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
)

type KuotaService struct {
	repo *repository.KuotaRepository
	loc  *time.Location
}

func NewKuotaService(repo *repository.KuotaRepository, loc *time.Location) *KuotaService {
	return &KuotaService{repo: repo, loc: loc}
}

// Availability answers "how many seats are left on this date". A date with
// no configured quota is not an error — it is simply full (remaining 0),
// which is exactly what the calendar renders.
func (s *KuotaService) Availability(ctx context.Context, q *dto.AvailabilityQuery) (*dto.AvailabilityResponse, error) {
	date, err := time.ParseInLocation("2006-01-02", q.Date, s.loc)
	if err != nil {
		return nil, err
	}

	var layananID *string
	if q.LayananID != "" {
		layananID = &q.LayananID
	}

	slot, err := s.repo.FindSlot(ctx, q.InstansiID, layananID, date)
	if err != nil {
		return nil, err
	}
	if slot == nil {
		return &dto.AvailabilityResponse{Date: q.Date, Kuota: 0, Terpakai: 0, Remaining: 0}, nil
	}

	return &dto.AvailabilityResponse{
		Date:      q.Date,
		Kuota:     slot.Kuota,
		Terpakai:  slot.Terpakai,
		Remaining: slot.Remaining(),
	}, nil
}
```

- [ ] **Step 8: Write the handler and module**

Create `apps/api/internal/modules/mpp/kuota/handler/kuota.handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type KuotaHandler struct {
	kuotaService *service.KuotaService
}

func NewKuotaHandler(s *service.KuotaService) *KuotaHandler {
	return &KuotaHandler{kuotaService: s}
}

func (h *KuotaHandler) Availability(c *gin.Context) {
	var query dto.AvailabilityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	result, err := h.kuotaService.Availability(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to read availability", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Availability retrieved successfully", result)
}
```

Create `apps/api/internal/modules/mpp/kuota/main.kuota.go`:

```go
package kuota

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/service"
)

type Module struct {
	Handler    *handler.KuotaHandler
	Service    *service.KuotaService
	Repository *repository.KuotaRepository
}

func Initialize(db *pgxpool.Pool, companyID string, loc *time.Location) *Module {
	repo := repository.NewKuotaRepository(db, companyID)
	svc := service.NewKuotaService(repo, loc)

	return &Module{
		Handler:    handler.NewKuotaHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the public availability read. Quota state — not
// RBAC — is the authority on whether a citizen may book.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/availability", m.Handler.Availability)
}
```

- [ ] **Step 9: Wire into the router**

In `internal/router/router.go`, import `"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota"` and add inside the `mppV1` block:

```go
		kuotaModule := kuota.Initialize(db, cfg.MPP.CompanyID, cfg.MPP.Location)
		kuotaModule.SetupRoutes(mppV1)
```

- [ ] **Step 10: Smoke it**

Run:

```bash
IID=a2000000-0000-0000-0000-000000000001
LID=a3000000-0000-0000-0000-000000000002
DATE=$(date -d '+1 day' +%F)
curl -s "http://localhost:8080/mpp/v1/availability?instansi_id=$IID&layanan_id=$LID&date=$DATE"
curl -s "http://localhost:8080/mpp/v1/availability?instansi_id=$IID&date=2030-01-01"
```

Expected: first call `"kuota":30,"terpakai":0,"remaining":30`; second (no quota row) `"kuota":0,"remaining":0` with HTTP 200.

- [ ] **Step 11: Commit**

```bash
git add apps/api/internal/modules/mpp/kuota apps/api/internal/router/router.go
git commit -m "feat(mpp): add kuota module with availability read and atomic quota consume"
```

---

### Task 7: `mpp/booking` — create booking (409 on full) + detail

**Files:**
- Create: `apps/api/internal/modules/mpp/booking/{domain/booking.go,dto/booking.dto.go,repository/booking.repository.go,service/booking.service.go,handler/booking.handler.go,main.booking.go}`
- Test: `apps/api/internal/modules/mpp/booking/service/booking_service_test.go`
- Modify: `apps/api/internal/router/router.go`

**Interfaces:**
- Consumes: `kuotaRepository.KuotaRepository.Consume` (Task 6), `instansiRepository.InstansiRepository.FindActiveLayanan` (Task 2), `cfg.MPP.Location`.
- Produces:
  - `booking.Initialize(db *pgxpool.Pool, kuota *kuotaRepository.KuotaRepository, catalog *instansiRepository.InstansiRepository, loc *time.Location) *Module` with `Handler`, `Service`, `Repository`.
  - `service.ErrQuotaFull`, `service.ErrLayananNotFound`, `service.ErrBookingNotFound`.
  - `repository.BookingRepository`:
    `UpsertPemohon(ctx, tx pgx.Tx, p *domain.Pemohon) (string, error)`,
    `Create(ctx, tx pgx.Tx, b *domain.Booking) error`,
    `FindDetailByID(ctx, id string) (*domain.BookingDetail, error)`,
    `FindByToken(ctx, token string) (*domain.BookingDetail, error)` *(consumed by Task 11)*,
    `MarkCheckedIn(ctx, tx pgx.Tx, bookingID string) (bool, error)` *(consumed by Task 11)*.
  - `domain.Booking`, `domain.BookingDetail`, `domain.Pemohon` (shapes below; Task 11 and Task 13 read them).
  - Routes `POST /mpp/v1/booking` (public, rate-limited), `GET /mpp/v1/booking/:id` (public).
  - `qr_token` / `qr_expires_at` are written as `NULL` here — populated in Task 9, exactly as slice 01 specifies.

- [ ] **Step 1: Write the failing service test**

Create `apps/api/internal/modules/mpp/booking/service/booking_service_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	bookingRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	instansiRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotaRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const (
	companyID  = "a1000000-0000-0000-0000-000000000001"
	instansiID = "a2000000-0000-0000-0000-000000000001"
	layananID  = "a3000000-0000-0000-0000-000000000002"
)

func newService(t *testing.T, pool *pgxpool.Pool) *service.BookingService {
	t.Helper()

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}

	return service.NewBookingService(
		pool,
		bookingRepo.NewBookingRepository(pool),
		kuotaRepo.NewKuotaRepository(pool, companyID),
		instansiRepo.NewInstansiRepository(pool, companyID),
		loc,
	)
}

func tomorrow() string {
	return time.Now().AddDate(0, 0, 1).Format("2006-01-02")
}

func setQuota(t *testing.T, pool *pgxpool.Pool, date string, kuota int) {
	t.Helper()

	if _, err := pool.Exec(context.Background(), `
		UPDATE mpp.kuota_booking SET kuota = $3, terpakai = 0
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2::date`,
		instansiID, date, kuota); err != nil {
		t.Fatalf("set quota: %v", err)
	}
}

func req(date string) *dto.CreateBookingRequest {
	return &dto.CreateBookingRequest{
		InstansiID: instansiID,
		LayananID:  layananID,
		Tanggal:    date,
		Pemohon:    dto.PemohonRequest{Name: "Ibu Sari", Phone: "628123456789"},
	}
}

func TestCreateReturnsBookedAndConsumesQuota(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	date := tomorrow()
	setQuota(t, pool, date, 5)

	resp, err := newService(t, pool).Create(context.Background(), req(date))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.Status != "BOOKED" {
		t.Errorf("status = %q, want BOOKED", resp.Status)
	}
	if resp.Channel != "WEB" {
		t.Errorf("channel = %q, want WEB", resp.Channel)
	}
	if resp.ID == "" {
		t.Error("want a booking id")
	}

	var terpakai int
	if err := pool.QueryRow(context.Background(), `
		SELECT terpakai FROM mpp.kuota_booking
		WHERE instansi_id = $1 AND jenis_layanan_id IS NULL AND tanggal = $2::date`,
		instansiID, date).Scan(&terpakai); err != nil {
		t.Fatalf("read terpakai: %v", err)
	}
	if terpakai != 1 {
		t.Fatalf("terpakai = %d, want 1", terpakai)
	}
}

func TestCreateRejectsUnknownLayanan(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	date := tomorrow()
	setQuota(t, pool, date, 5)

	body := req(date)
	body.LayananID = "a3000000-0000-0000-0000-000000000099"

	if _, err := newService(t, pool).Create(context.Background(), body); !errors.Is(err, service.ErrLayananNotFound) {
		t.Fatalf("err = %v, want ErrLayananNotFound", err)
	}
}

func TestCreateRejectsWhenQuotaFull(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	date := tomorrow()
	setQuota(t, pool, date, 0)

	if _, err := newService(t, pool).Create(context.Background(), req(date)); !errors.Is(err, service.ErrQuotaFull) {
		t.Fatalf("err = %v, want ErrQuotaFull", err)
	}
}

// TestCreateNeverOverbooks exercises the whole service path (validate →
// tx → consume → insert) under concurrency — the end-to-end NFR-DATA-02
// proof the slice's Definition of Done asks for.
func TestCreateNeverOverbooks(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	date := tomorrow()
	setQuota(t, pool, date, 1)

	svc := newService(t, pool)

	const n = 15
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Create(context.Background(), req(date))
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	var ok, full int
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, service.ErrQuotaFull):
			full++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || full != n-1 {
		t.Fatalf("created = %d, quota-full = %d; want 1 and %d", ok, full, n-1)
	}

	var bookings int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM mpp.booking WHERE tanggal = $1::date`, date).Scan(&bookings); err != nil {
		t.Fatalf("count bookings: %v", err)
	}
	if bookings != 1 {
		t.Fatalf("bookings = %d, want 1 (overbooking!)", bookings)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/booking/...`
Expected: FAIL — undefined `service.NewBookingService`.

- [ ] **Step 3: Write the domain structs**

Create `apps/api/internal/modules/mpp/booking/domain/booking.go`:

```go
package domain

import "time"

// Pemohon is the applicant. PII is minimized: NIK is only ever stored
// hashed, and only when the service demands it (security-privacy.md).
type Pemohon struct {
	ID      string
	Name    string
	Phone   *string
	Email   *string
	NIKHash *string
}

// Booking is a scheduled registration, before on-site check-in.
type Booking struct {
	ID             string
	PemohonID      string
	InstansiID     string
	JenisLayananID string
	Tanggal        time.Time
	Channel        string // WEB | WHATSAPP
	QRToken        *string
	QRExpiresAt    *time.Time
	Status         string // BOOKED | CHECKED_IN | EXPIRED | CANCELLED
	CheckedInAt    *time.Time
	CreatedAt      time.Time
}

// BookingDetail is a booking joined with the catalog rows the confirm
// screen, the check-in kiosk and the printed ticket all need.
type BookingDetail struct {
	Booking

	InstansiName        string
	InstansiPrefix      string
	InstansiQueueMode   string
	LayananName         string
	EstimasiDurasiMenit int
	PemohonName         string
}
```

- [ ] **Step 4: Write the repository**

Create `apps/api/internal/modules/mpp/booking/repository/booking.repository.go`:

```go
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/domain"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

// UpsertPemohon dedupes applicants by phone and refreshes their contact
// details. Returns the pemohon id.
//
// ponytail: mpp.pemohon has a non-unique index on phone, so two
// simultaneous first-time bookings from one number can create two rows —
// harmless duplication, no data loss. Add a unique partial index if the
// dedupe ever has to be exact.
func (r *BookingRepository) UpsertPemohon(ctx context.Context, tx pgx.Tx, p *domain.Pemohon) (string, error) {
	if p.Phone != nil && *p.Phone != "" {
		var id string
		err := tx.QueryRow(ctx,
			`SELECT id FROM mpp.pemohon WHERE phone = $1 ORDER BY created_at ASC LIMIT 1`,
			*p.Phone).Scan(&id)

		switch {
		case err == nil:
			_, err = tx.Exec(ctx, `
				UPDATE mpp.pemohon
				SET name = $2,
				    email = COALESCE($3, email),
				    nik_hash = COALESCE($4, nik_hash),
				    updated_at = NOW()
				WHERE id = $1`, id, p.Name, p.Email, p.NIKHash)
			if err != nil {
				logger.Error("Failed to refresh pemohon", logger.Err(err))
				return "", err
			}
			return id, nil
		case !errors.Is(err, pgx.ErrNoRows):
			logger.Error("Failed to look up pemohon", logger.Err(err))
			return "", err
		}
	}

	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO mpp.pemohon (name, phone, email, nik_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, p.Name, p.Phone, p.Email, p.NIKHash).Scan(&id)
	if err != nil {
		logger.Error("Failed to create pemohon", logger.Err(err))
		return "", err
	}

	return id, nil
}

// Create inserts the booking row. This is the ONLY place qr_token is
// written — keeping the write in one place is what makes swapping the raw
// token for a hash a one-line change (see service.issueToken, Task 9).
func (r *BookingRepository) Create(ctx context.Context, tx pgx.Tx, b *domain.Booking) error {
	query := `
		INSERT INTO mpp.booking (
			pemohon_id, instansi_id, jenis_layanan_id, tanggal, channel,
			qr_token, qr_expires_at, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	err := tx.QueryRow(ctx, query,
		b.PemohonID, b.InstansiID, b.JenisLayananID, b.Tanggal, b.Channel,
		b.QRToken, b.QRExpiresAt, b.Status,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		logger.Error("Failed to create booking", logger.Err(err))
		return err
	}

	return nil
}

const detailSelect = `
	SELECT b.id, b.pemohon_id, b.instansi_id, b.jenis_layanan_id, b.tanggal, b.channel,
	       b.qr_token, b.qr_expires_at, b.status, b.checked_in_at, b.created_at,
	       i.name, i.prefix, i.queue_mode,
	       l.name, l.estimasi_durasi_menit,
	       p.name
	FROM mpp.booking b
	JOIN mpp.instansi i ON i.id = b.instansi_id
	JOIN mpp.jenis_layanan l ON l.id = b.jenis_layanan_id
	JOIN mpp.pemohon p ON p.id = b.pemohon_id`

func scanDetail(row pgx.Row) (*domain.BookingDetail, error) {
	var d domain.BookingDetail
	err := row.Scan(
		&d.ID, &d.PemohonID, &d.InstansiID, &d.JenisLayananID, &d.Tanggal, &d.Channel,
		&d.QRToken, &d.QRExpiresAt, &d.Status, &d.CheckedInAt, &d.CreatedAt,
		&d.InstansiName, &d.InstansiPrefix, &d.InstansiQueueMode,
		&d.LayananName, &d.EstimasiDurasiMenit,
		&d.PemohonName,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// FindDetailByID returns the booking with catalog context, or (nil, nil).
func (r *BookingRepository) FindDetailByID(ctx context.Context, id string) (*domain.BookingDetail, error) {
	d, err := scanDetail(r.db.QueryRow(ctx, detailSelect+` WHERE b.id = $1 AND b.deleted_at IS NULL`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find booking", logger.Err(err))
		return nil, err
	}

	return d, nil
}

// FindByToken resolves a QR token to its booking, or (nil, nil).
// Counterpart of the single token write in Create.
func (r *BookingRepository) FindByToken(ctx context.Context, token string) (*domain.BookingDetail, error) {
	d, err := scanDetail(r.db.QueryRow(ctx, detailSelect+` WHERE b.qr_token = $1 AND b.deleted_at IS NULL`, token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find booking by token", logger.Err(err))
		return nil, err
	}

	return d, nil
}

// MarkCheckedIn flips BOOKED → CHECKED_IN. The status guard turns a
// replayed or concurrent scan into a 0-row result (false) instead of a
// second check-in — the race-safe half of "single-use token" (BR-09).
func (r *BookingRepository) MarkCheckedIn(ctx context.Context, tx pgx.Tx, bookingID string) (bool, error) {
	var id string
	err := tx.QueryRow(ctx, `
		UPDATE mpp.booking
		SET status = 'CHECKED_IN', checked_in_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'BOOKED'
		RETURNING id`, bookingID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		logger.Error("Failed to mark booking checked-in", logger.Err(err))
		return false, err
	}

	return true, nil
}
```

- [ ] **Step 5: Write the DTOs**

Create `apps/api/internal/modules/mpp/booking/dto/booking.dto.go`:

```go
package dto

// PemohonRequest carries the minimum PII a booking needs. NIK stays
// optional and is hashed before storage.
type PemohonRequest struct {
	Name  string  `json:"name" binding:"required,min=2,max=255"`
	Phone string  `json:"phone" binding:"required,min=8,max=20"`
	Email *string `json:"email" binding:"omitempty,email"`
	NIK   *string `json:"nik" binding:"omitempty,len=16,numeric"`
}

// CreateBookingRequest is the public booking payload.
type CreateBookingRequest struct {
	InstansiID string         `json:"instansi_id" binding:"required,uuid"`
	LayananID  string         `json:"layanan_id" binding:"required,uuid"`
	Tanggal    string         `json:"tanggal" binding:"required,datetime=2006-01-02"`
	Pemohon    PemohonRequest `json:"pemohon" binding:"required"`
}

// BookingResponse is returned by POST /booking. qr_token / qr_expires_at
// are populated from slice 02 onward.
type BookingResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	InstansiID  string  `json:"instansi_id"`
	LayananID   string  `json:"layanan_id"`
	Tanggal     string  `json:"tanggal"`
	Channel     string  `json:"channel"`
	QRToken     *string `json:"qr_token"`
	QRExpiresAt *string `json:"qr_expires_at"`
	CreatedAt   string  `json:"created_at"`
}

// BookingRef is a nested catalog reference on the detail response.
type BookingRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix,omitempty"`
}

// BookingDetailResponse backs GET /booking/{id} — the confirm screen and
// the "re-open my QR" flow.
type BookingDetailResponse struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Tanggal     string     `json:"tanggal"`
	Channel     string     `json:"channel"`
	QRToken     *string    `json:"qr_token"`
	QRExpiresAt *string    `json:"qr_expires_at"`
	Instansi    BookingRef `json:"instansi"`
	Layanan     BookingRef `json:"layanan"`
	PemohonName string     `json:"pemohon_name"`
	CreatedAt   string     `json:"created_at"`
}
```

- [ ] **Step 6: Write the service**

Create `apps/api/internal/modules/mpp/booking/service/booking.service.go`:

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	instansiRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotaRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

var (
	// ErrQuotaFull → 409. Re-exported from the kuota repository so the
	// handler depends on a single package.
	ErrQuotaFull = kuotaRepo.ErrQuotaFull
	// ErrLayananNotFound → 404 (unknown or inactive agency/service).
	ErrLayananNotFound = errors.New("layanan not found or inactive")
	// ErrBookingNotFound → 404.
	ErrBookingNotFound = errors.New("booking not found")
)

type BookingService struct {
	db      *pgxpool.Pool
	repo    *repository.BookingRepository
	kuota   *kuotaRepo.KuotaRepository
	catalog *instansiRepo.InstansiRepository
	loc     *time.Location
}

func NewBookingService(
	db *pgxpool.Pool,
	repo *repository.BookingRepository,
	kuota *kuotaRepo.KuotaRepository,
	catalog *instansiRepo.InstansiRepository,
	loc *time.Location,
) *BookingService {
	return &BookingService{db: db, repo: repo, kuota: kuota, catalog: catalog, loc: loc}
}

// Create books a slot. Quota consumption and the booking insert share one
// transaction, so a full date can never leave a phantom booking behind and
// a rolled-back insert always returns the seat.
func (s *BookingService) Create(ctx context.Context, req *dto.CreateBookingRequest) (*dto.BookingResponse, error) {
	tanggal, err := time.ParseInLocation("2006-01-02", req.Tanggal, s.loc)
	if err != nil {
		return nil, err
	}

	layanan, _, err := s.catalog.FindActiveLayanan(ctx, req.InstansiID, req.LayananID)
	if err != nil {
		return nil, err
	}
	if layanan == nil {
		return nil, ErrLayananNotFound
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is a no-op after a successful commit.
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.kuota.Consume(ctx, tx, req.InstansiID, &req.LayananID, tanggal); err != nil {
		return nil, err
	}

	pemohonID, err := s.repo.UpsertPemohon(ctx, tx, &domain.Pemohon{
		Name:    req.Pemohon.Name,
		Phone:   &req.Pemohon.Phone,
		Email:   req.Pemohon.Email,
		NIKHash: hashNIK(req.Pemohon.NIK),
	})
	if err != nil {
		return nil, err
	}

	booking := &domain.Booking{
		PemohonID:      pemohonID,
		InstansiID:     req.InstansiID,
		JenisLayananID: req.LayananID,
		Tanggal:        tanggal,
		Channel:        "WEB",
		Status:         "BOOKED",
	}

	if err := s.repo.Create(ctx, tx, booking); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	logger.Info("Booking created",
		logger.String("booking_id", booking.ID),
		logger.String("instansi_id", booking.InstansiID),
	)

	return toBookingResponse(booking), nil
}

// GetByID backs the public confirm screen.
func (s *BookingService) GetByID(ctx context.Context, id string) (*dto.BookingDetailResponse, error) {
	d, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrBookingNotFound
	}

	return toBookingDetailResponse(d), nil
}

// hashNIK keeps the national ID out of the database in the clear: only a
// SHA-256 digest is stored, and only when the citizen supplied one.
func hashNIK(nik *string) *string {
	if nik == nil || *nik == "" {
		return nil
	}

	sum := sha256.Sum256([]byte(*nik))
	hashed := hex.EncodeToString(sum[:])

	return &hashed
}

func formatUTC(t *time.Time) *string {
	if t == nil {
		return nil
	}
	out := t.UTC().Format(time.RFC3339)
	return &out
}

func toBookingResponse(b *domain.Booking) *dto.BookingResponse {
	return &dto.BookingResponse{
		ID:          b.ID,
		Status:      b.Status,
		InstansiID:  b.InstansiID,
		LayananID:   b.JenisLayananID,
		Tanggal:     b.Tanggal.Format("2006-01-02"),
		Channel:     b.Channel,
		QRToken:     b.QRToken,
		QRExpiresAt: formatUTC(b.QRExpiresAt),
		CreatedAt:   b.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toBookingDetailResponse(d *domain.BookingDetail) *dto.BookingDetailResponse {
	return &dto.BookingDetailResponse{
		ID:          d.ID,
		Status:      d.Status,
		Tanggal:     d.Tanggal.Format("2006-01-02"),
		Channel:     d.Channel,
		QRToken:     d.QRToken,
		QRExpiresAt: formatUTC(d.QRExpiresAt),
		Instansi:    dto.BookingRef{ID: d.InstansiID, Name: d.InstansiName, Prefix: d.InstansiPrefix},
		Layanan:     dto.BookingRef{ID: d.JenisLayananID, Name: d.LayananName},
		PemohonName: d.PemohonName,
		CreatedAt:   d.CreatedAt.UTC().Format(time.RFC3339),
	}
}
```

- [ ] **Step 7: Run the service tests to green**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/booking/... -v -race`
Expected: PASS (4 tests) including `TestCreateNeverOverbooks`. Paste the output.

- [ ] **Step 8: Write the handler**

Create `apps/api/internal/modules/mpp/booking/handler/booking.handler.go`:

```go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/dto"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	"github.com/ndollem/mpp/apps/api/internal/shared/response"
)

type BookingHandler struct {
	bookingService *service.BookingService
}

func NewBookingHandler(s *service.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: s}
}

func (h *BookingHandler) Create(c *gin.Context) {
	var req dto.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	result, err := h.bookingService.Create(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrQuotaFull):
			response.Error(c, http.StatusConflict, "Kuota tanggal ini sudah penuh", "quota_full")
		case errors.Is(err, service.ErrLayananNotFound):
			response.Error(c, http.StatusNotFound, "Instansi atau layanan tidak ditemukan", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create booking", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "Booking created", result)
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	result, err := h.bookingService.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrBookingNotFound) {
			response.Error(c, http.StatusNotFound, "Booking not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get booking", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Booking retrieved successfully", result)
}
```

- [ ] **Step 9: Write `main.booking.go` with the public rate limiter**

Create `apps/api/internal/modules/mpp/booking/main.booking.go`:

```go
package booking

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/handler"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/repository"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking/service"
	instansiRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi/repository"
	kuotaRepo "github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota/repository"
)

type Module struct {
	Handler    *handler.BookingHandler
	Service    *service.BookingService
	Repository *repository.BookingRepository
}

func Initialize(
	db *pgxpool.Pool,
	kuota *kuotaRepo.KuotaRepository,
	catalog *instansiRepo.InstansiRepository,
	loc *time.Location,
) *Module {
	repo := repository.NewBookingRepository(db)
	svc := service.NewBookingService(db, repo, kuota, catalog, loc)

	return &Module{
		Handler:    handler.NewBookingHandler(svc),
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes registers the public registration endpoints. They carry no
// JWTAuth by design — quota and booking state are the authority, not RBAC
// (docs/prompt/01-pendaftaran.md).
//
// Rate limiting uses the skeleton's in-process limiter keyed by IP: 10
// booking attempts per minute, 5-minute lockout (NFR-SEC-06).
//
// ponytail: in-process means per-instance. Swap the limiter for a Redis
// INCR+TTL one when the API runs more than one replica.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	bookingLimiter := middleware.NewRateLimiter(10, time.Minute)

	rg.POST("/booking", middleware.IPBasedRateLimiter(bookingLimiter, 5*time.Minute), m.Handler.Create)
	rg.GET("/booking/:id", m.Handler.GetByID)
}
```

- [ ] **Step 10: Wire into the router**

In `internal/router/router.go`, import `"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking"` and add inside the `mppV1` block, after the kuota module:

```go
		bookingModule := booking.Initialize(db, kuotaModule.Repository, instansiModule.Repository, cfg.MPP.Location)
		bookingModule.SetupRoutes(mppV1)
```

- [ ] **Step 11: Run the curl smoke script**

Run:

```bash
IID=a2000000-0000-0000-0000-000000000001
LID=a3000000-0000-0000-0000-000000000002
DATE=$(date -d '+1 day' +%F)

curl -s "http://localhost:8080/mpp/v1/availability?instansi_id=$IID&layanan_id=$LID&date=$DATE"

BID=$(curl -s -X POST http://localhost:8080/mpp/v1/booking -H 'Content-Type: application/json' \
  -d '{"instansi_id":"'$IID'","layanan_id":"'$LID'","tanggal":"'$DATE'","pemohon":{"name":"Sari","phone":"628123456789"}}' \
  | tee /dev/stderr | jq -r .data.id)

curl -s "http://localhost:8080/mpp/v1/booking/$BID" | jq .data

psql "postgres://postgres:postgres@localhost:5432/mpp" -c \
  "UPDATE mpp.kuota_booking SET kuota = terpakai WHERE instansi_id='$IID' AND jenis_layanan_id IS NULL AND tanggal='$DATE';"

curl -s -o /dev/null -w 'full date (expect 409): %{http_code}\n' -X POST http://localhost:8080/mpp/v1/booking \
  -H 'Content-Type: application/json' \
  -d '{"instansi_id":"'$IID'","layanan_id":"'$LID'","tanggal":"'$DATE'","pemohon":{"name":"Sari","phone":"628123456789"}}'
```

Expected: `201` with `"status":"BOOKED"`, a detail read echoing it, and `409` once the date is full. Paste the output.

- [ ] **Step 12: Commit**

```bash
git add apps/api/internal/modules/mpp/booking apps/api/internal/router/router.go
git commit -m "feat(mpp): add booking module with atomic quota reservation and 409 on full"
```

---

### Task 8: Frontend citizen booking flow

**Files:**
- Modify: `apps/web/src/lib/api/endpoints.ts`, `apps/web/src/lib/api/index.ts`
- Create: `apps/web/src/lib/api/booking.ts`, `apps/web/src/lib/api/use-booking.ts`
- Create: `apps/web/src/app/(citizen)/daftar/page.tsx`
- Create: `apps/web/src/sections/citizen/view/booking-view.tsx`
- Create: `apps/web/src/sections/citizen/booking-instansi-section.tsx`
- Create: `apps/web/src/sections/citizen/booking-layanan-section.tsx`
- Create: `apps/web/src/sections/citizen/booking-form-section.tsx`

**Interfaces:**
- Consumes: `apiFetch`, `endpoints`, `paths`, backend routes from Tasks 2/6/7.
- Produces:
  - `booking.ts`: `instansiSchema`, `syaratDokumenSchema`, `layananSchema`, `availabilitySchema`, `bookingSchema`, `bookingDetailSchema`; fetchers `getInstansiList()`, `getLayananList(instansiId)`, `getAvailability(params)`, `createBooking(payload)`, `getBookingDetail(id)`; key factory `mppKeys`.
  - `use-booking.ts`: `useInstansiQuery()`, `useLayananQuery(instansiId)`, `useAvailabilityQuery(params)`, `useCreateBookingMutation()`, `useBookingDetailQuery(id)` *(the last is consumed by Task 10)*.

- [ ] **Step 1: Add the booking endpoints**

In `apps/web/src/lib/api/endpoints.ts`, extend the `mpp` block added in Task 5:

```ts
    availability: 'mpp/v1/availability',
    booking: {
      create: 'mpp/v1/booking',
      detail: (id: string) => `mpp/v1/booking/${encodeURIComponent(id)}`,
    },
```

- [ ] **Step 2: Write the schemas, fetchers and key factory**

Create `apps/web/src/lib/api/booking.ts`:

```ts
import { z } from 'zod';

import { apiFetch } from './client';
import { endpoints } from './endpoints';

// ----------------------------------------------------------------------
// Schemas mirror the Go DTOs (apps/api/internal/modules/mpp/**/dto).
// Parsed at the fetch boundary so backend drift fails here, loudly, and
// never leaks an `any` into a component.

// Go marshals nil slices as JSON null — normalize to [].
const nullableList = <T extends z.ZodTypeAny>(item: T) =>
  z
    .array(item)
    .nullish()
    .transform((value) => value ?? []);

export const instansiSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  prefix: z.string(),
  description: z.string().nullish(),
  logo_url: z.string().nullish(),
  queue_mode: z.enum(['FIFO', 'BOOKING_PRIORITY']),
  is_active: z.boolean(),
});

export const syaratDokumenSchema = z.object({
  id: z.string(),
  name: z.string(),
  is_required: z.boolean(),
  notes: z.string().nullish(),
  sort: z.number(),
});

export const layananSchema = z.object({
  id: z.string(),
  instansi_id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  estimasi_durasi_menit: z.number(),
  requires_fo_verification: z.boolean(),
  syarat_dokumen: nullableList(syaratDokumenSchema),
});

export const availabilitySchema = z.object({
  date: z.string(),
  kuota: z.number(),
  terpakai: z.number(),
  remaining: z.number(),
});

export const bookingSchema = z.object({
  id: z.string(),
  status: z.string(),
  instansi_id: z.string(),
  layanan_id: z.string(),
  tanggal: z.string(),
  channel: z.string(),
  qr_token: z.string().nullish(),
  qr_expires_at: z.string().nullish(),
  created_at: z.string(),
});

const bookingRefSchema = z.object({
  id: z.string(),
  name: z.string(),
  prefix: z.string().optional().default(''),
});

export const bookingDetailSchema = z.object({
  id: z.string(),
  status: z.string(),
  tanggal: z.string(),
  channel: z.string(),
  qr_token: z.string().nullish(),
  qr_expires_at: z.string().nullish(),
  instansi: bookingRefSchema,
  layanan: bookingRefSchema,
  pemohon_name: z.string(),
  created_at: z.string(),
});

export type Instansi = z.infer<typeof instansiSchema>;
export type Layanan = z.infer<typeof layananSchema>;
export type Availability = z.infer<typeof availabilitySchema>;
export type Booking = z.infer<typeof bookingSchema>;
export type BookingDetail = z.infer<typeof bookingDetailSchema>;

export type AvailabilityParams = {
  instansiId: string;
  layananId?: string;
  /** Local calendar date, YYYY-MM-DD. */
  date: string;
};

export type CreateBookingPayload = {
  instansi_id: string;
  layanan_id: string;
  tanggal: string;
  pemohon: { name: string; phone: string; email?: string | null; nik?: string | null };
};

// ----------------------------------------------------------------------
// Query keys — every filter that reaches the request is serialized, so two
// different result sets never share one cache entry.

export const mppKeys = {
  all: ['mpp'] as const,
  instansi: () => [...mppKeys.all, 'instansi'] as const,
  layanan: (instansiId: string) => [...mppKeys.all, 'layanan', instansiId] as const,
  availability: (params: AvailabilityParams) =>
    [
      ...mppKeys.all,
      'availability',
      { instansiId: params.instansiId, layananId: params.layananId ?? '', date: params.date },
    ] as const,
  bookingDetail: (id: string) => [...mppKeys.all, 'booking', id] as const,
};

// ----------------------------------------------------------------------
// Fetchers

type FetchOptions = { signal?: AbortSignal };

export async function getInstansiList(options: FetchOptions = {}): Promise<Instansi[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.instansi.list, { signal: options.signal });
  return nullableList(instansiSchema).parse(data);
}

export async function getLayananList(
  instansiId: string,
  options: FetchOptions = {}
): Promise<Layanan[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.instansi.layanan(instansiId), {
    signal: options.signal,
  });
  return nullableList(layananSchema).parse(data);
}

export async function getAvailability(
  params: AvailabilityParams,
  options: FetchOptions = {}
): Promise<Availability> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.availability, {
    params: { instansi_id: params.instansiId, layanan_id: params.layananId, date: params.date },
    signal: options.signal,
  });
  return availabilitySchema.parse(data);
}

export async function createBooking(payload: CreateBookingPayload): Promise<Booking> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.booking.create, {
    method: 'post',
    body: payload,
  });
  return bookingSchema.parse(data);
}

export async function getBookingDetail(
  id: string,
  options: FetchOptions = {}
): Promise<BookingDetail> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.booking.detail(id), {
    signal: options.signal,
  });
  return bookingDetailSchema.parse(data);
}
```

- [ ] **Step 3: Write the hooks module**

Create `apps/web/src/lib/api/use-booking.ts`:

```ts
'use client';

import type { AvailabilityParams, CreateBookingPayload } from './booking';

import { useQuery, useMutation } from '@tanstack/react-query';

import {
  mppKeys,
  createBooking,
  getAvailability,
  getLayananList,
  getInstansiList,
  getBookingDetail,
} from './booking';

// ----------------------------------------------------------------------

export function useInstansiQuery() {
  return useQuery({
    queryKey: mppKeys.instansi(),
    queryFn: ({ signal }) => getInstansiList({ signal }),
    staleTime: 5 * 60 * 1000,
  });
}

export function useLayananQuery(instansiId: string) {
  return useQuery({
    queryKey: mppKeys.layanan(instansiId),
    queryFn: ({ signal }) => getLayananList(instansiId, { signal }),
    enabled: Boolean(instansiId),
    staleTime: 5 * 60 * 1000,
  });
}

export function useAvailabilityQuery(params: AvailabilityParams) {
  return useQuery({
    queryKey: mppKeys.availability(params),
    queryFn: ({ signal }) => getAvailability(params, { signal }),
    enabled: Boolean(params.instansiId && params.date),
    // Quota moves while the citizen is choosing — keep it fresh.
    staleTime: 15 * 1000,
  });
}

export function useCreateBookingMutation() {
  return useMutation({
    mutationFn: (payload: CreateBookingPayload) => createBooking(payload),
  });
}

export function useBookingDetailQuery(id: string) {
  return useQuery({
    queryKey: mppKeys.bookingDetail(id),
    queryFn: ({ signal }) => getBookingDetail(id, { signal }),
    enabled: Boolean(id),
  });
}
```

In `apps/web/src/lib/api/index.ts`, add `export * from './booking';`.

- [ ] **Step 4: Write the section components**

Create `apps/web/src/sections/citizen/booking-instansi-section.tsx`:

```tsx
'use client';

import type { Instansi } from 'src/lib/api/booking';

import Card from '@mui/material/Card';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

type Props = {
  items: Instansi[];
  value: string;
  onChange: (id: string) => void;
};

export function BookingInstansiSection({ items, value, onChange }: Props) {
  return (
    <Card sx={{ p: 3 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        1. Pilih instansi
      </Typography>

      <Stack spacing={1.5}>
        {items.map((instansi) => (
          <Button
            key={instansi.id}
            fullWidth
            size="large"
            variant={value === instansi.id ? 'contained' : 'outlined'}
            onClick={() => onChange(instansi.id)}
            sx={{ justifyContent: 'flex-start', textAlign: 'left' }}
          >
            {instansi.name} ({instansi.prefix})
          </Button>
        ))}
      </Stack>
    </Card>
  );
}
```

Create `apps/web/src/sections/citizen/booking-layanan-section.tsx`:

```tsx
'use client';

import type { Layanan } from 'src/lib/api/booking';

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

type Props = {
  items: Layanan[];
  value: string;
  onChange: (id: string) => void;
};

export function BookingLayananSection({ items, value, onChange }: Props) {
  const selected = items.find((item) => item.id === value);

  return (
    <Card sx={{ p: 3 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        2. Pilih jenis layanan
      </Typography>

      <Stack spacing={1.5}>
        {items.map((layanan) => (
          <Button
            key={layanan.id}
            fullWidth
            size="large"
            variant={value === layanan.id ? 'contained' : 'outlined'}
            onClick={() => onChange(layanan.id)}
            sx={{ justifyContent: 'space-between' }}
          >
            <span>{layanan.name}</span>
            <span>~{layanan.estimasi_durasi_menit} mnt</span>
          </Button>
        ))}
      </Stack>

      {selected && selected.syarat_dokumen.length > 0 && (
        <Box sx={{ mt: 3 }}>
          <Typography variant="subtitle2" sx={{ mb: 1 }}>
            Syarat dokumen
          </Typography>
          <Stack direction="row" flexWrap="wrap" gap={1}>
            {selected.syarat_dokumen.map((syarat) => (
              <Chip
                key={syarat.id}
                label={syarat.is_required ? `${syarat.name} (wajib)` : syarat.name}
                color={syarat.is_required ? 'primary' : 'default'}
                variant="outlined"
              />
            ))}
          </Stack>
        </Box>
      )}
    </Card>
  );
}
```

Create `apps/web/src/sections/citizen/booking-form-section.tsx`:

```tsx
'use client';

import type { UseFormReturn } from 'react-hook-form';

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { Form, Field } from 'src/components/hook-form';

type Props = {
  methods: UseFormReturn<any>;
  onSubmit: () => void;
  remaining: number | null;
  submitting: boolean;
  errorMessage: string | null;
};

export function BookingFormSection({
  methods,
  onSubmit,
  remaining,
  submitting,
  errorMessage,
}: Props) {
  const isFull = remaining !== null && remaining <= 0;

  return (
    <Card sx={{ p: 3 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        3. Tanggal & data pemohon
      </Typography>

      <Form methods={methods} onSubmit={onSubmit}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
          <Field.DatePicker name="tanggal" label="Tanggal kunjungan" />

          {remaining !== null &&
            (isFull ? (
              <Alert severity="warning">Kuota tanggal ini penuh. Silakan pilih tanggal lain.</Alert>
            ) : (
              <Alert severity="info">Sisa kuota tanggal ini: {remaining}</Alert>
            ))}

          <Field.Text name="name" label="Nama lengkap" />
          <Field.Text name="phone" label="Nomor WhatsApp" placeholder="628…" />
          <Field.Text name="email" label="Email (opsional)" />
          <Field.Text name="nik" label="NIK (opsional, 16 digit)" />

          {errorMessage && <Alert severity="error">{errorMessage}</Alert>}

          <Button type="submit" size="large" variant="contained" disabled={submitting || isFull}>
            {submitting ? 'Memproses…' : 'Buat booking'}
          </Button>
        </Box>
      </Form>
    </Card>
  );
}
```

- [ ] **Step 5: Write the view**

Create `apps/web/src/sections/citizen/view/booking-view.tsx`:

```tsx
'use client';

import type { Dayjs } from 'dayjs';

import { z } from 'zod';
import { useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Container from '@mui/material/Container';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { useRouter } from 'src/routes/hooks';

import { ApiError } from 'src/lib/api/client';
import {
  useLayananQuery,
  useInstansiQuery,
  useAvailabilityQuery,
  useCreateBookingMutation,
} from 'src/lib/api/use-booking';

import { BookingFormSection } from '../booking-form-section';
import { BookingLayananSection } from '../booking-layanan-section';
import { BookingInstansiSection } from '../booking-instansi-section';

// ----------------------------------------------------------------------

const BookingSchema = z.object({
  tanggal: z.any().refine((value) => Boolean(value), { message: 'Tanggal wajib dipilih' }),
  name: z.string().min(2, { message: 'Nama minimal 2 karakter' }),
  phone: z.string().min(8, { message: 'Nomor WhatsApp tidak valid' }),
  email: z.string().email({ message: 'Email tidak valid' }).or(z.literal('')),
  nik: z
    .string()
    .regex(/^\d{16}$/, { message: 'NIK harus 16 digit' })
    .or(z.literal('')),
});

type BookingValues = z.infer<typeof BookingSchema>;

/** Dayjs → local YYYY-MM-DD (never toISOString, which shifts to UTC). */
function toLocalDate(value: unknown): string {
  const day = value as Dayjs | null;
  return day && typeof day.format === 'function' ? day.format('YYYY-MM-DD') : '';
}

export function BookingView() {
  const router = useRouter();

  const [instansiId, setInstansiId] = useState('');
  const [layananId, setLayananId] = useState('');

  const instansiQuery = useInstansiQuery();
  const layananQuery = useLayananQuery(instansiId);
  const createBooking = useCreateBookingMutation();

  const methods = useForm<BookingValues>({
    resolver: zodResolver(BookingSchema),
    defaultValues: { tanggal: null, name: '', phone: '', email: '', nik: '' },
  });

  const tanggal = toLocalDate(methods.watch('tanggal'));

  const availabilityQuery = useAvailabilityQuery({
    instansiId,
    layananId: layananId || undefined,
    date: tanggal,
  });

  const remaining = useMemo(
    () => (availabilityQuery.data ? availabilityQuery.data.remaining : null),
    [availabilityQuery.data]
  );

  const errorMessage = useMemo(() => {
    const error = createBooking.error;
    if (!error) return null;
    if (error instanceof ApiError && error.status === 409) {
      return 'Kuota tanggal ini penuh. Silakan pilih tanggal lain.';
    }
    return (error as Error).message;
  }, [createBooking.error]);

  const onSubmit = methods.handleSubmit(async (values) => {
    const booking = await createBooking.mutateAsync({
      instansi_id: instansiId,
      layanan_id: layananId,
      tanggal: toLocalDate(values.tanggal),
      pemohon: {
        name: values.name,
        phone: values.phone,
        email: values.email || null,
        nik: values.nik || null,
      },
    });

    router.push(paths.citizen.booking.detail(booking.id));
  });

  return (
    <Container maxWidth="sm" sx={{ py: 5 }}>
      <Typography variant="h3" sx={{ mb: 1 }}>
        Daftar Antrean
      </Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Pilih instansi dan layanan, lalu tentukan tanggal kunjungan Anda.
      </Typography>

      <Stack spacing={3}>
        <BookingInstansiSection
          items={instansiQuery.data ?? []}
          value={instansiId}
          onChange={(id) => {
            setInstansiId(id);
            setLayananId('');
          }}
        />

        {instansiId && (
          <BookingLayananSection
            items={layananQuery.data ?? []}
            value={layananId}
            onChange={setLayananId}
          />
        )}

        {layananId && (
          <BookingFormSection
            methods={methods}
            onSubmit={onSubmit}
            remaining={remaining}
            submitting={createBooking.isPending}
            errorMessage={errorMessage}
          />
        )}
      </Stack>

      <Box sx={{ height: 40 }} />
    </Container>
  );
}
```

- [ ] **Step 6: Write the thin page**

Create `apps/web/src/app/(citizen)/daftar/page.tsx`:

```tsx
import type { Metadata } from 'next';

import { BookingView } from 'src/sections/citizen/view/booking-view';

export const metadata: Metadata = { title: 'Daftar Antrean' };

export default function Page() {
  return <BookingView />;
}
```

- [ ] **Step 7: Run the frontend gate**

Run: `cd apps/web && yarn tsc:check && yarn lint`
Expected: both clean.

- [ ] **Step 8: Manual e2e checklist**

With the API and `yarn dev` running, open `http://localhost:8002/daftar`:
1. Agencies load; picking Dukcapil loads its services with `syarat_dokumen` chips.
2. Picking a date shows "Sisa kuota …".
3. Submitting a valid form navigates to `/booking/<id>` (a 404 page until Task 10 — expected).
4. Set that date's quota to full in psql, resubmit → "Kuota tanggal ini penuh." is shown and no navigation happens.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/lib/api apps/web/src/sections/citizen "apps/web/src/app/(citizen)"
git commit -m "feat(web): add citizen booking flow (agency -> service -> date -> pemohon)"
```

---

## Phase 2 — Slice 02: Terbitkan QR (`docs/prompt/02-terbitkan-qr.md`)

### Task 9: QR token issuance + expiry + `mpp/config` reader

**Files:**
- Create: `apps/api/internal/modules/mpp/config/domain/config.go`
- Create: `apps/api/internal/modules/mpp/config/repository/config.repository.go`
- Create: `apps/api/internal/modules/mpp/config/service/config.service.go`
- Test: `apps/api/internal/modules/mpp/config/service/config_service_test.go`
- Modify: `apps/api/internal/modules/mpp/booking/service/booking.service.go`
- Modify: `apps/api/internal/modules/mpp/booking/main.booking.go`
- Modify: `apps/api/internal/modules/mpp/booking/service/booking_service_test.go`
- Modify: `apps/api/internal/router/router.go`

**Interfaces:**
- Consumes: `pkg/token.GenerateBase64Token`, `repository.BookingRepository.Create` (Task 7).
- Produces:
  - `config.NewService(db *pgxpool.Pool) *service.ConfigService` (module has **no routes** this phase — admin config CRUD is deferred).
  - `ConfigService` typed getters, each falling back to a documented default when no row exists:
    `CheckinWindowMinutes(ctx, instansiID string) (int, error)` — `0` means "valid until end of the booking day";
    `NumberFormat(ctx, instansiID string) (domain.NumberFormat, error)` — default `{Separator: "-", Pad: 3}` *(consumed by Task 13)*;
    `TTSTemplate(ctx, instansiID string) (string, error)` — default `"Nomor antrian %s, silakan menuju %s"` *(consumed by Task 17)*.
  - `service.IssueTokenExpiry(tanggal time.Time, loc *time.Location, windowMinutes int) time.Time` — exported pure helper on the booking service package.
  - `POST /mpp/v1/booking` and `GET /mpp/v1/booking/:id` now return non-null `qr_token` + `qr_expires_at` (UTC RFC3339).

- [ ] **Step 1: Write the failing config-service test**

Create `apps/api/internal/modules/mpp/config/service/config_service_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/testutil"
)

const instansiID = "a2000000-0000-0000-0000-000000000001"

func TestDefaultsWhenNoRowExists(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DELETE FROM mpp.system_config`); err != nil {
		t.Fatalf("clear config: %v", err)
	}

	svc := service.NewConfigService(pool)

	window, err := svc.CheckinWindowMinutes(ctx, instansiID)
	if err != nil {
		t.Fatalf("CheckinWindowMinutes: %v", err)
	}
	if window != 0 {
		t.Errorf("window = %d, want 0 (end of booking day)", window)
	}

	format, err := svc.NumberFormat(ctx, instansiID)
	if err != nil {
		t.Fatalf("NumberFormat: %v", err)
	}
	if format.Separator != "-" || format.Pad != 3 {
		t.Errorf("format = %+v, want {Separator:-, Pad:3}", format)
	}

	tpl, err := svc.TTSTemplate(ctx, instansiID)
	if err != nil {
		t.Fatalf("TTSTemplate: %v", err)
	}
	if tpl != "Nomor antrian %s, silakan menuju %s" {
		t.Errorf("template = %q, want the default phrasing", tpl)
	}
}

func TestPerAgencyOverrideBeatsGlobal(t *testing.T) {
	pool := testutil.RequireDB(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DELETE FROM mpp.system_config`); err != nil {
		t.Fatalf("clear config: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mpp.system_config (instansi_id, config_key, config_value)
		VALUES (NULL, 'number_format', '{"separator":".","pad":4}'::jsonb),
		       ($1, 'number_format', '{"separator":"/","pad":2}'::jsonb)`, instansiID); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	svc := service.NewConfigService(pool)

	format, err := svc.NumberFormat(ctx, instansiID)
	if err != nil {
		t.Fatalf("NumberFormat: %v", err)
	}
	if format.Separator != "/" || format.Pad != 2 {
		t.Fatalf("format = %+v, want the per-agency override", format)
	}

	global, err := svc.NumberFormat(ctx, "a2000000-0000-0000-0000-000000000003")
	if err != nil {
		t.Fatalf("NumberFormat (other agency): %v", err)
	}
	if global.Separator != "." || global.Pad != 4 {
		t.Fatalf("format = %+v, want the global row", global)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/config/...`
Expected: FAIL — undefined `service.NewConfigService`.

- [ ] **Step 3: Write the config domain + repository**

Create `apps/api/internal/modules/mpp/config/domain/config.go`:

```go
package domain

// NumberFormat controls how a queue number is rendered from a prefix and
// a sequence (BR-04): "A" + 14 → "A-014" with the defaults below.
type NumberFormat struct {
	Separator string `json:"separator"`
	Pad       int    `json:"pad"`
}

// Config keys stored in mpp.system_config.
const (
	KeyCheckinWindow = "checkin_window" // {"minutes": 120}
	KeyNumberFormat  = "number_format"  // {"separator":"-","pad":3}
	KeyTTSText       = "tts_text"       // {"template":"Nomor antrian %s, silakan menuju %s"}
)
```

Create `apps/api/internal/modules/mpp/config/repository/config.repository.go`:

```go
package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

type ConfigRepository struct {
	db *pgxpool.Pool
}

func NewConfigRepository(db *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// Get resolves a config key with per-agency precedence: the agency row
// wins, then the global row (instansi_id IS NULL), then (nil, nil) so the
// caller applies its documented default.
func (r *ConfigRepository) Get(ctx context.Context, instansiID, key string) (json.RawMessage, error) {
	query := `
		SELECT config_value
		FROM mpp.system_config
		WHERE config_key = $2
		  AND (instansi_id = $1::uuid OR instansi_id IS NULL)
		ORDER BY (instansi_id IS NULL) ASC
		LIMIT 1`

	var raw json.RawMessage
	err := r.db.QueryRow(ctx, query, instansiID, key).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to read system config", logger.Err(err))
		return nil, err
	}

	return raw, nil
}
```

- [ ] **Step 4: Write the config service**

Create `apps/api/internal/modules/mpp/config/service/config.service.go`:

```go
package service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/domain"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
)

// Documented defaults. They are the contract when an operator has not
// configured anything — never silently change one.
const (
	DefaultCheckinWindowMinutes = 0 // 0 = valid until the end of the booking day
	DefaultSeparator            = "-"
	DefaultPad                  = 3
	DefaultTTSTemplate          = "Nomor antrian %s, silakan menuju %s"
)

type ConfigService struct {
	repo *repository.ConfigRepository
}

func NewConfigService(db *pgxpool.Pool) *ConfigService {
	return &ConfigService{repo: repository.NewConfigRepository(db)}
}

// CheckinWindowMinutes returns how long after the booking day starts a QR
// stays valid. 0 keeps the token valid until 23:59:59 local on the day.
func (s *ConfigService) CheckinWindowMinutes(ctx context.Context, instansiID string) (int, error) {
	raw, err := s.repo.Get(ctx, instansiID, domain.KeyCheckinWindow)
	if err != nil || raw == nil {
		return DefaultCheckinWindowMinutes, err
	}

	var value struct {
		Minutes int `json:"minutes"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || value.Minutes <= 0 {
		return DefaultCheckinWindowMinutes, nil
	}

	return value.Minutes, nil
}

// NumberFormat returns the queue-number rendering rules (BR-04).
func (s *ConfigService) NumberFormat(ctx context.Context, instansiID string) (domain.NumberFormat, error) {
	format := domain.NumberFormat{Separator: DefaultSeparator, Pad: DefaultPad}

	raw, err := s.repo.Get(ctx, instansiID, domain.KeyNumberFormat)
	if err != nil || raw == nil {
		return format, err
	}

	var value domain.NumberFormat
	if err := json.Unmarshal(raw, &value); err != nil {
		return format, nil
	}
	if value.Separator != "" {
		format.Separator = value.Separator
	}
	if value.Pad > 0 {
		format.Pad = value.Pad
	}

	return format, nil
}

// TTSTemplate returns the announcement phrasing (FR-CFG-03). The template
// takes exactly two %s verbs: the spelled-out number, then the loket.
func (s *ConfigService) TTSTemplate(ctx context.Context, instansiID string) (string, error) {
	raw, err := s.repo.Get(ctx, instansiID, domain.KeyTTSText)
	if err != nil || raw == nil {
		return DefaultTTSTemplate, err
	}

	var value struct {
		Template string `json:"template"`
	}
	if err := json.Unmarshal(raw, &value); err != nil || value.Template == "" {
		return DefaultTTSTemplate, nil
	}

	return value.Template, nil
}
```

- [ ] **Step 5: Run the config tests to green**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/config/... -v`
Expected: PASS (2 tests).

- [ ] **Step 6: Write the failing token tests in the booking package**

Append to `apps/api/internal/modules/mpp/booking/service/booking_service_test.go`:

```go
func TestIssueTokenExpiryEndsTheBookingDayInUTC(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skip("Asia/Jakarta tzdata unavailable")
	}

	tanggal := time.Date(2026, 8, 6, 0, 0, 0, 0, loc)

	expiry := service.IssueTokenExpiry(tanggal, loc, 0)

	local := expiry.In(loc)
	if local.Year() != 2026 || local.Month() != time.August || local.Day() != 6 {
		t.Fatalf("expiry local date = %s, want 2026-08-06", local.Format(time.RFC3339))
	}
	if local.Hour() != 23 || local.Minute() != 59 || local.Second() != 59 {
		t.Fatalf("expiry local time = %s, want 23:59:59", local.Format("15:04:05"))
	}
	if expiry.Location() != time.UTC {
		t.Errorf("expiry must be returned in UTC, got %s", expiry.Location())
	}
}

func TestIssueTokenExpiryHonoursConfiguredWindow(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skip("Asia/Jakarta tzdata unavailable")
	}

	tanggal := time.Date(2026, 8, 6, 0, 0, 0, 0, loc)

	// 10 hours after local midnight → 10:00 WIB → 03:00 UTC.
	expiry := service.IssueTokenExpiry(tanggal, loc, 600)

	local := expiry.In(loc)
	if local.Hour() != 10 || local.Minute() != 0 {
		t.Fatalf("expiry local time = %s, want 10:00", local.Format("15:04:05"))
	}
}

func TestCreateIssuesUniqueUrlSafeTokens(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateMPP(t, pool)

	date := tomorrow()
	setQuota(t, pool, date, 5)

	svc := newService(t, pool)

	first, err := svc.Create(context.Background(), req(date))
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second, err := svc.Create(context.Background(), req(date))
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if first.QRToken == nil || *first.QRToken == "" {
		t.Fatal("first booking has no qr_token")
	}
	if second.QRToken == nil || *second.QRToken == "" {
		t.Fatal("second booking has no qr_token")
	}
	if *first.QRToken == *second.QRToken {
		t.Fatal("tokens collide — they must be crypto-random per booking")
	}
	if len(*first.QRToken) < 22 {
		t.Errorf("token %q is shorter than 128 bits of entropy", *first.QRToken)
	}
	if strings.ContainsAny(*first.QRToken, "+/ ") {
		t.Errorf("token %q is not URL-safe", *first.QRToken)
	}
	if first.QRExpiresAt == nil {
		t.Fatal("qr_expires_at must be set")
	}
	if _, err := time.Parse(time.RFC3339, *first.QRExpiresAt); err != nil {
		t.Errorf("qr_expires_at %q is not RFC3339 UTC: %v", *first.QRExpiresAt, err)
	}
}
```

Add `"strings"` to that file's imports.

- [ ] **Step 7: Run and watch the new tests fail**

Run: `cd apps/api && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/booking/... -run 'Token' -v`
Expected: FAIL — undefined `service.IssueTokenExpiry`, and `qr_token` is nil.

- [ ] **Step 8: Issue the token in the booking service**

In `apps/api/internal/modules/mpp/booking/service/booking.service.go`:

1. Add imports `"strings"` and `configService "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"` and `"github.com/ndollem/mpp/apps/api/pkg/token"`.
2. Add the `config` dependency to the struct and constructor:

```go
type BookingService struct {
	db      *pgxpool.Pool
	repo    *repository.BookingRepository
	kuota   *kuotaRepo.KuotaRepository
	catalog *instansiRepo.InstansiRepository
	config  *configService.ConfigService
	loc     *time.Location
}

func NewBookingService(
	db *pgxpool.Pool,
	repo *repository.BookingRepository,
	kuota *kuotaRepo.KuotaRepository,
	catalog *instansiRepo.InstansiRepository,
	cfg *configService.ConfigService,
	loc *time.Location,
) *BookingService {
	return &BookingService{db: db, repo: repo, kuota: kuota, catalog: catalog, config: cfg, loc: loc}
}
```

3. Add the pure helpers at the bottom of the file:

```go
// IssueTokenExpiry computes when a QR stops working, in UTC.
//
// windowMinutes == 0 → valid until 23:59:59 local on the booking day
// (BR-09's "hari-H"). A positive window ends that many minutes after
// local midnight of the booking day, letting an agency say "check in
// before 10:00".
func IssueTokenExpiry(tanggal time.Time, loc *time.Location, windowMinutes int) time.Time {
	y, m, d := tanggal.In(loc).Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, loc)

	if windowMinutes > 0 {
		return midnight.Add(time.Duration(windowMinutes) * time.Minute).UTC()
	}

	return midnight.Add(24*time.Hour - time.Second).UTC()
}

// issueToken mints an opaque, unguessable check-in handle: 32 random
// bytes (256 bits) in URL-safe base64, padding stripped so it survives a
// QR payload and a URL untouched. It carries NO PII — it is a handle to
// the booking, nothing more.
//
// ponytail: the raw token is stored. Everything that reads or writes it
// goes through repository.Create / FindByToken, so hashing later means
// changing those two call sites and nothing else.
func issueToken() (string, error) {
	raw, err := token.GenerateBase64Token(32)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(raw, "="), nil
}
```

4. Inside `Create`, replace the `booking := &domain.Booking{...}` construction with the token-carrying version (placed after `UpsertPemohon`, before `repo.Create`):

```go
	windowMinutes, err := s.config.CheckinWindowMinutes(ctx, req.InstansiID)
	if err != nil {
		return nil, err
	}

	qrToken, err := issueToken()
	if err != nil {
		return nil, err
	}
	expiresAt := IssueTokenExpiry(tanggal, s.loc, windowMinutes)

	booking := &domain.Booking{
		PemohonID:      pemohonID,
		InstansiID:     req.InstansiID,
		JenisLayananID: req.LayananID,
		Tanggal:        tanggal,
		Channel:        "WEB",
		QRToken:        &qrToken,
		QRExpiresAt:    &expiresAt,
		Status:         "BOOKED",
	}

	if err := s.repo.Create(ctx, tx, booking); err != nil {
		// booking_qr_token_key is unique. A collision at 256 bits is
		// astronomically unlikely, but retrying once costs nothing and
		// turns a theoretical 500 into a success.
		if isUniqueViolation(err, "booking_qr_token_key") {
			retryToken, tokenErr := issueToken()
			if tokenErr != nil {
				return nil, tokenErr
			}
			booking.QRToken = &retryToken
			if err := s.repo.Create(ctx, tx, booking); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
```

5. Add the violation helper (imports `"github.com/jackc/pgx/v5/pgconn"`):

```go
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
```

- [ ] **Step 9: Update the module wiring and the test helper**

In `apps/api/internal/modules/mpp/booking/main.booking.go`, add the import
`configService "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"` and extend `Initialize`:

```go
func Initialize(
	db *pgxpool.Pool,
	kuota *kuotaRepo.KuotaRepository,
	catalog *instansiRepo.InstansiRepository,
	cfg *configService.ConfigService,
	loc *time.Location,
) *Module {
	repo := repository.NewBookingRepository(db)
	svc := service.NewBookingService(db, repo, kuota, catalog, cfg, loc)
	...
}
```

In `apps/api/internal/modules/mpp/booking/service/booking_service_test.go`, update `newService` to pass the config service (import `configService "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"`):

```go
	return service.NewBookingService(
		pool,
		bookingRepo.NewBookingRepository(pool),
		kuotaRepo.NewKuotaRepository(pool, companyID),
		instansiRepo.NewInstansiRepository(pool, companyID),
		configService.NewConfigService(pool),
		loc,
	)
```

In `apps/api/internal/router/router.go`, build the config service once (it is shared with Tasks 13 and 17) and pass it in:

```go
		mppConfig := mppConfigService.NewConfigService(db)

		bookingModule := booking.Initialize(db, kuotaModule.Repository, instansiModule.Repository, mppConfig, cfg.MPP.Location)
		bookingModule.SetupRoutes(mppV1)
```

with the import `mppConfigService "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/service"`.

- [ ] **Step 10: Run the whole MPP suite**

Run: `cd apps/api && go build ./... && TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/mpp?sslmode=disable" go test ./internal/modules/mpp/... -race`
Expected: all packages PASS. Paste the output.

- [ ] **Step 11: Smoke the token**

Run:

```bash
IID=a2000000-0000-0000-0000-000000000001
LID=a3000000-0000-0000-0000-000000000002
DATE=$(date -d '+1 day' +%F)

BID=$(curl -s -X POST http://localhost:8080/mpp/v1/booking -H 'Content-Type: application/json' \
  -d '{"instansi_id":"'$IID'","layanan_id":"'$LID'","tanggal":"'$DATE'","pemohon":{"name":"Sari","phone":"628123456789"}}' \
  | jq -r .data.id)

curl -s "http://localhost:8080/mpp/v1/booking/$BID" | jq '.data | {qr_token, qr_expires_at}'
```

Expected: a long URL-safe token and an RFC3339 `Z` timestamp at the end of the booking day. Paste the output.

- [ ] **Step 12: Commit**

```bash
git add apps/api/internal/modules/mpp/config apps/api/internal/modules/mpp/booking apps/api/internal/router/router.go
git commit -m "feat(mpp): issue single-use time-bound QR tokens with configurable check-in window"
```

---

### Task 10: Frontend confirm + QR screen

**Files:**
- Modify: `apps/web/package.json` (add `qrcode.react`)
- Create: `apps/web/src/app/(citizen)/booking/[id]/page.tsx`
- Create: `apps/web/src/sections/citizen/view/booking-confirm-view.tsx`
- Create: `apps/web/src/sections/citizen/booking-qr-section.tsx`

**Interfaces:**
- Consumes: `useBookingDetailQuery(id)` (Task 8), `paths.citizen.*`.
- Produces: the confirm route `/booking/<id>` rendering a scannable QR of `qr_token` plus a working **Unduh QR** download.

- [ ] **Step 1: Add the QR dependency**

Run: `cd apps/web && yarn add qrcode.react@^4.2.0`
Expected: `package.json` gains `"qrcode.react": "^4.2.0"` and `yarn.lock` updates. Do **not** hand-roll QR encoding.

- [ ] **Step 2: Write the QR section**

Create `apps/web/src/sections/citizen/booking-qr-section.tsx`:

```tsx
'use client';

import { useRef } from 'react';
import { QRCodeCanvas } from 'qrcode.react';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

type Props = {
  token: string;
  fileName: string;
};

export function BookingQrSection({ token, fileName }: Props) {
  const wrapperRef = useRef<HTMLDivElement>(null);

  // The QR encodes the opaque token itself — the kiosk posts exactly this
  // string to /mpp/v1/checkin. No PII travels in the code.
  const handleDownload = () => {
    const canvas = wrapperRef.current?.querySelector('canvas');
    if (!canvas) return;

    const link = document.createElement('a');
    link.download = `${fileName}.png`;
    link.href = canvas.toDataURL('image/png');
    link.click();
  };

  return (
    <Stack spacing={2} alignItems="center">
      <Box
        ref={wrapperRef}
        sx={{ p: 2, bgcolor: 'common.white', borderRadius: 1, lineHeight: 0 }}
      >
        <QRCodeCanvas value={token} size={220} level="M" includeMargin />
      </Box>

      <Typography variant="body2" sx={{ color: 'text.secondary', textAlign: 'center' }}>
        Tunjukkan QR ini di kiosk MPP untuk check-in.
      </Typography>

      <Button variant="outlined" size="large" onClick={handleDownload}>
        Unduh QR
      </Button>
    </Stack>
  );
}
```

- [ ] **Step 3: Write the confirm view**

Create `apps/web/src/sections/citizen/view/booking-confirm-view.tsx`:

```tsx
'use client';

import Card from '@mui/material/Card';
import Alert from '@mui/material/Alert';
import Stack from '@mui/material/Stack';
import Divider from '@mui/material/Divider';
import Container from '@mui/material/Container';
import Typography from '@mui/material/Typography';

import { useBookingDetailQuery } from 'src/lib/api/use-booking';

import { BookingQrSection } from '../booking-qr-section';

// ----------------------------------------------------------------------

type Props = { id: string };

/** UTC instant → readable local time (WIB/WITA/WIT per the device). */
function formatLocal(value?: string | null) {
  if (!value) return '-';
  return new Date(value).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' });
}

export function BookingConfirmView({ id }: Props) {
  const { data, isPending, isError, error } = useBookingDetailQuery(id);

  if (isPending) {
    return (
      <Container maxWidth="sm" sx={{ py: 5 }}>
        <Typography>Memuat booking…</Typography>
      </Container>
    );
  }

  if (isError || !data) {
    return (
      <Container maxWidth="sm" sx={{ py: 5 }}>
        <Alert severity="error">
          Booking tidak ditemukan. {(error as Error | null)?.message ?? ''}
        </Alert>
      </Container>
    );
  }

  return (
    <Container maxWidth="sm" sx={{ py: 5 }}>
      <Typography variant="h3" sx={{ mb: 1 }}>
        Booking berhasil
      </Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Simpan QR di bawah ini. Anda membutuhkannya untuk check-in di lokasi.
      </Typography>

      <Card sx={{ p: 3 }}>
        <Stack spacing={1} sx={{ mb: 3 }}>
          <Typography variant="h6">{data.instansi.name}</Typography>
          <Typography variant="body2">{data.layanan.name}</Typography>
          <Typography variant="body2">Tanggal: {data.tanggal}</Typography>
          <Typography variant="body2">Atas nama: {data.pemohon_name}</Typography>
          <Typography variant="body2">Status: {data.status}</Typography>
        </Stack>

        <Divider sx={{ mb: 3 }} />

        {data.qr_token ? (
          <>
            <BookingQrSection token={data.qr_token} fileName={`qr-mpp-${data.id}`} />
            <Typography variant="caption" sx={{ display: 'block', mt: 2, textAlign: 'center' }}>
              Berlaku sampai {formatLocal(data.qr_expires_at)}
            </Typography>
          </>
        ) : (
          <Alert severity="warning">
            QR sudah tidak tersedia untuk booking ini (sudah dipakai atau dibatalkan).
          </Alert>
        )}
      </Card>
    </Container>
  );
}
```

- [ ] **Step 4: Write the thin page**

Create `apps/web/src/app/(citizen)/booking/[id]/page.tsx`:

```tsx
import type { Metadata } from 'next';

import { BookingConfirmView } from 'src/sections/citizen/view/booking-confirm-view';

export const metadata: Metadata = { title: 'Konfirmasi Booking' };

type Props = { params: Promise<{ id: string }> };

export default async function Page({ params }: Props) {
  const { id } = await params;

  return <BookingConfirmView id={id} />;
}
```

- [ ] **Step 5: Run the frontend gate**

Run: `cd apps/web && yarn tsc:check && yarn lint`
Expected: both clean.

- [ ] **Step 6: Manual e2e checklist**

1. `/daftar` → submit → lands on `/booking/<id>` showing agency · service · date and a QR.
2. **Unduh QR** downloads a PNG that a phone QR reader decodes to the same token as
   `curl -s .../mpp/v1/booking/<id> | jq -r .data.qr_token`.
3. Reloading `/booking/<id>` shows the identical QR.

- [ ] **Step 7: Commit**

```bash
git add apps/web/package.json apps/web/yarn.lock apps/web/src/sections/citizen "apps/web/src/app/(citizen)"
git commit -m "feat(web): render downloadable check-in QR on the booking confirm screen"
```

---
