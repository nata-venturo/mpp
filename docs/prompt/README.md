# MPP Build Prompts — 6-Slice Walking Skeleton

Executable build-prompt briefs for an AI coding agent. Each doc is a self-contained work
order for one **vertical slice** of the happy path (registration → done + TV display),
grounded in the **real** skeleton patterns (see `apps/api/CLAUDE.md`, `apps/web/CLAUDE.md`,
and the requirement docs in `../`).

These are **implementation prompts**, not a PRD — the PRD/requirements already live in
`../01-requirements/`. A slice doc tells the agent exactly what to build, where, and how to
prove it works.

## The 6 slices (scoreboard)

| # | Slice | Core | Main endpoint | Done when |
|---|-------|------|---------------|-----------|
| [01](./01-pendaftaran.md) | Pendaftaran | booking + atomic quota | `POST /mpp/v1/booking` (full→409) | `201` + `BOOKED`; full→`409` no overbook |
| [02](./02-terbitkan-qr.md) | Terbitkan QR | single-use token + expiry | `qr_token` in booking resp · `GET /booking/{id}` | unique token + expiry; QR screen + download |
| [03](./03-checkin.md) | Check-in | device `X-API-Key` | `POST /mpp/v1/checkin` | valid→`CHECKED_IN`; reuse/expired→rejected |
| [04](./04-nomor-antrian.md) | Nomor A-014 | Redis `INCR` | alloc inside `/checkin` · `GET /queue` | atomic seq, no dup; →`WAITING` |
| [05](./05-panggil.md) | Panggil | idle-longest, max 3× | `/queue/next` · `/antrian/{id}/{recall,start,skip}` | `CALLED→SERVING`; 4th recall→`409` |
| [06](./06-selesai-display.md) | Selesai + Display | close + TV | `/antrian/{id}/done` · `GET /display` | →`DONE` records duration; TV shows nomor+loket |

Read [`00-prerequisites.md`](./00-prerequisites.md) **first** — the slices assume master
data, MPP roles, an FE auth layer, the `/mpp/v1` router group, and (from slice 5) a
WebSocket hub already exist.

**Already built and want to run it?** [`WALKTHROUGH.md`](./WALKTHROUGH.md) takes you from
`make bootstrap` to a manual end-to-end test of all six slices (browser and `curl`), plus
the seeded credentials and a troubleshooting table.

Build order = 01 → 06 (each depends on the prior). Slices map across delivery-plan phases
(see [`../08-roadmap/delivery-plan.md`](../08-roadmap/delivery-plan.md)); they are a thin
end-to-end skeleton, not one phase each.

---

## Shared conventions (apply to every slice)

### Response envelope

All API responses use the skeleton envelope
(`apps/api/internal/shared/response/response.go`). The FE `ky` client unwraps it
(`apps/web/src/lib/api/client.ts`).

```json
{ "data": {}, "message": "human-readable", "meta": { "pagination": {} }, "errors": { "field": ["msg"] } }
```

Helpers (exact signatures):
- `response.Success(c, http.StatusOK, "message", data)`
- `response.SuccessWithPagination(c, http.StatusOK, "message", data, page, limit, total)`
- `response.Error(c, http.StatusBadRequest, "message", "detail")`
- `response.ValidationError(c, http.StatusBadRequest, "message", map[string][]string{...})`

### HTTP status codes

| Code | Use |
|------|-----|
| 200 / 201 | success / created |
| 400 | validation error (`errors[]`) |
| 401 | missing/invalid credentials |
| 403 | authenticated but lacks permission |
| 404 | not found / out of tenant scope |
| 409 | conflict — quota full, illegal state transition, duplicate number, reused token |
| 410 | gone — expired QR token (optional; 409 acceptable) |
| 429 | rate limited (public registration) |

### Time & tenancy

- **All timestamps UTC** in requests/responses (`TZ=UTC` enforced backend-side). Clients
  localize to WIB/WITA/WIT.
- Tenancy: **one MPP building = one company**; `instansi` (agency) is an entity inside it.
  Public FE sends `X-Company-Slug` (already wired in `client.ts`).

### Backend module pattern (mirror `core/company`)

Every new module lives at `apps/api/internal/modules/mpp/<name>/` with:

```
<name>/
├── domain/            # entities (plain structs, DB-shaped)
├── dto/               # request/response structs with `binding:` tags
├── repository/        # pgxpool hand-written SQL (NO ORM)
├── service/           # business logic, validation, orchestration
├── handler/           # Gin handlers: bind DTO → call service → response.*
└── main.<name>.go     # Initialize(db) *Module + SetupRoutes(rg *gin.RouterGroup)
```

- `Initialize(db *pgxpool.Pool) *Module` builds repo → service → handler.
- `SetupRoutes(rg *gin.RouterGroup)` registers routes; guard mutations with
  `middleware.RequirePermission("mpp.<res>:<verb>")` (verb from the CRUD vocabulary — see
  [`../06-security/rbac-matrix.md`](../06-security/rbac-matrix.md)).
- Wire in `apps/api/internal/router/router.go` under the `mppV1` group (added in slice 0).
- Repository always filters tenant/soft-delete: `WHERE … AND deleted_at IS NULL`; scope by
  `company_id`/`instansi_id` as applicable.
- Redis via `apps/api/internal/shared/redis` (go-redis v9): `Get/Set/Del/Incr/Ping`.

### Frontend api-layer pattern (mirror `articles.ts`)

For each resource the FE talks to, create the trio in `apps/web/src/lib/api/`:
1. `endpoints.ts` entry (URL map).
2. `<resource>.ts` — **zod schema validated at the fetch boundary** (no `as` casts) + a
   fetcher through the shared `api` (ky) instance + a query-key factory.
3. `use-<resource>.ts` — `'use client'` TanStack Query hooks (NOT re-exported from a barrel;
   server code must not import client hooks).

UI follows **page → view → section** (`apps/web/src/app/<route>/page.tsx` →
`src/sections/<vertical>/view/*-view.tsx` → section components). Routes centralized in
`src/routes/paths.ts`. Forms use `Field.*` + `Form` from `src/components/hook-form/` with
`zodResolver`. Env only via `CONFIG` (`src/global-config.ts`), never `process.env`.

### Test & verification approach (TDD where it pays)

- **Backend has no test harness today** (only a couple of `pkg/*_test.go`). Each slice
  scaffolds:
  - **Table-driven unit tests** (stdlib `testing`) for the service's core logic (atomic
    quota, token validation, state transitions, recall cap) — write the failing test first.
  - **One `httptest` integration test** per new endpoint asserting status + envelope shape.
  - A **`curl` smoke script** (in the slice doc) to hit the live server after `make api-dev`.
  - Run: `cd apps/api && go test ./internal/modules/mpp/...`
- **Frontend has no test runner** — the gate is `yarn tsc:check` + `yarn lint` + a **manual
  e2e checklist** in the slice doc. Do not add a test framework unless asked.
- **Never claim done without running** the tests/curl and pasting output (see
  `superpowers:verification-before-completion`).

### Definition of Done (template — every slice restates concretely)

- [ ] Migrations already applied (`make db-setup`); no schema drift.
- [ ] Backend module built to pattern; routes wired under `mppV1`; permissions from the RBAC map.
- [ ] `go test ./internal/modules/mpp/...` green; `curl` smoke script passes (output pasted).
- [ ] FE trio + route + view built; `yarn tsc:check` + `yarn lint` green.
- [ ] Manual e2e checklist passes; the slice's `KOMPLIT inti` acceptance criteria met.
- [ ] Contract honored exactly (path · status · field · envelope · UTC).
