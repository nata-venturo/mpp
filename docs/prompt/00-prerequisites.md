# Slice 0 — Prerequisites (groundwork)

The 6 slices assume this groundwork exists. Build it before slice 01 (or in parallel where
noted). This is **not** one of the 6 scoreboard slices — it's the platform they stand on.

Maps to delivery-plan **Fase 1** (master data + FE auth) plus the router/WS scaffolding the
later slices need.

---

## P1 — Router group `/mpp/v1`

Add to `apps/api/internal/router/router.go`, alongside the existing `coreV1 :=
router.Group("/core/v1")` block:

```go
mppV1 := router.Group("/mpp/v1")
{
    // MPP modules register here as slices land, e.g.:
    // bookingModule := booking.Initialize(db); bookingModule.SetupRoutes(mppV1)
}
```

Public MPP reads (catalog, availability, public queue status) live under `mppV1` with **no**
`JWTAuth()`; staff/device routes add `JWTAuth()` + `RequirePermission(...)`.

**Auth entry point:** `middleware.JWTAuth()` is unified — it checks the `X-API-Key` header
first (device flow, scoped permissions) and falls back to the `Authorization: Bearer` JWT
(`apps/api/internal/middleware/auth.go`). There is **no** separate API-key middleware.
Device endpoints (kiosk/TV) therefore use `JWTAuth()` too; the device's API-key must carry
the scoped permission the route requires.

## P2 — Master data modules (`mpp/instansi`, `mpp/layanan`, `mpp/loket`, `mpp/kuota`)

Minimal CRUD, mirroring `apps/api/internal/modules/core/company/`. Tables already exist
(`internal/database/migrations/mpp/000002_master.up.sql`,
`000003_registration.up.sql`). Each module: `domain/dto/repository/service/handler` +
`main.<name>.go`. Enough to create an agency, its services (+ `syarat_dokumen`), its lokets
(+ `loket_layanan` map), and quota rows. Guard writes with `mpp.<res>:create|update|delete`.

The slices only strictly need **read** access to this data plus seeded demo rows (P4), so a
thin read-first implementation is acceptable if full admin CRUD is deferred — but the tables
and at least `GET` endpoints must work.

## P3 — MPP RBAC roles (seeder)

Create `apps/api/internal/database/seeders/mpp/001_roles.sql` mirroring
`seeders/core/001_roles.sql`. Seed the four roles as `{"resource":"level"}` JSONB maps taken
**verbatim** from [`../06-security/rbac-matrix.md`](../06-security/rbac-matrix.md). Keep
`is_system = TRUE`. Example (`petugas_loket`):

```sql
INSERT INTO core.roles (id, code, name, permissions, is_system, is_active) VALUES
('20000000-0000-0000-0000-000000000004', 'petugas_loket', 'Petugas Loket',
 '{"mpp.antrian":"editor","mpp.queue":"editor","mpp.booking":"viewer",
   "mpp.layanan":"viewer","mpp.loket":"viewer","mpp.fo":"viewer"}'::jsonb,
 TRUE, TRUE)
ON CONFLICT DO NOTHING;
```

Remember: levels expand to CRUD verbs at login (`viewer→read`, `editor→create,read,update,
delete`, `admin→+export,import,restore,approve`). Queue actions (call/skip/start/done) are
guarded as `mpp.antrian:update` → need `editor`. **Do not invent verbs** — see the RBAC doc.

Wire `seeders/mpp/` into `make seed-mpp` (already scaffolded in `apps/api/Makefile`).

## P4 — Demo seed data (`seeders/mpp/002_demo.sql`)

One tenant company (reuse a `core.companies` row from core seed), one agency
(`instansi` prefix `A`, `queue_mode='FIFO'`), ≥1 service
(`jenis_layanan`, e.g. "Perpanjang KTP", `estimasi_durasi_menit=10`,
`requires_fo_verification=false`) with a couple `syarat_dokumen`, ≥2 lokets
(`loket` status `OPEN`, mapped via `loket_layanan`), and quota rows
(`kuota_booking`) for today + the next few days. Enough for the whole happy path.

## P5 — Device API-keys (kiosk, tv)

Ensure the `core` api_key module can mint scoped keys (it exists:
`internal/modules/core/api_key/`). Seed or document creating:
- **kiosk key** — scoped permissions `["mpp.checkin:create","mpp.booking:create",
  "mpp.layanan:read","mpp.instansi:read"]`.
- **tv key** — scoped permissions `["mpp.display:read","mpp.queue:read"]`.

These are passed as `X-API-Key` by the kiosk/TV frontends (P7).

**Implemented.** `seeders/mpp/009_device_keys.sql` mints both keys against dedicated
device users (`kiosk@mpp.device`, `tv@mpp.device`) carrying the narrow roles
`mpp_device_kiosk` / `mpp_device_tv` — an API key's effective permissions are the
*intersection* of its scope and its **owning user's** permissions, so a key owned by a
user without MPP permissions would resolve to nothing.

| Device | Key (test env — rotate before real deployment) |
|--------|-----------------------------------------------|
| kiosk | `wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90` |
| tv | `wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978` |

Staff logins (`seeders/mpp/008_staff_users.sql`), all with password `Petugas2026*`:
`petugas@mpp.test`, `fo@mpp.test`, `supervisor@mpp.test`, `adminmpp@mpp.test`.

## P6 — Frontend auth layer

The skeleton FE ships **unauthenticated**. Add, at the `ky` seam in
`apps/web/src/lib/api/client.ts` (the `beforeRequest`/`afterResponse` comment block):
- a **token store** (in-memory + `localStorage`) for the staff JWT (loket/FO/admin);
- `beforeRequest` hook attaching `Authorization: Bearer <token>` when present;
- `afterResponse` hook handling `401` (redirect to signin / refresh);
- a signin page hitting `POST /core/v1/auth/signin` (body `{login, password}`).

Kiosk and TV do **not** use the user token — they attach a build-configured `X-API-Key`
header via a separate `ky` instance (or per-request header), gated by API-key context, not
user login.

## P7 — Route groups & layouts (FE)

Create route groups under `apps/web/src/app/`:
`(citizen)/` (public booking + status), `(kiosk)/` (touch, API-key), `(loket)/` (staff JWT),
`(tv)/` (mini-PC display, API-key). Add their paths to `src/routes/paths.ts`. Kiosk/TV get
device-appropriate full-screen layouts; the `/components` gallery gate pattern in
`src/middleware.ts` is the reference for route gating.

## P8 — WebSocket hub (backend) — needed from slice 05

Scaffold a hub before slice 05:
- `GET /mpp/v1/ws` upgrade endpoint (`JWTAuth()` — staff JWT or device key).
- A hub that fans out server→client events, subscribing to **Redis pub/sub** channels so any
  API instance can serve any socket (`internal/shared/redis` provides the client; add a
  pub/sub wrapper).
- Channels + event shapes per [`../04-api/websocket-events.md`](../04-api/websocket-events.md)
  (`instansi:<prefix>`, `layanan:<id>`, `loket:<id>`, `display:<instansi>`, `monitoring`).
- On subscribe/reconnect, reply with a `snapshot` (rebuilt from Postgres/Redis).

Place under `apps/api/internal/modules/mpp/ws/` (or `internal/shared/ws/` if you prefer it
cross-module). Slices 05–06 publish to it; slice 05 also adds the FE client `src/lib/ws.ts`.

---

## Prerequisite Definition of Done

- [ ] `mppV1` group live; `GET /mpp/v1/instansi` returns seeded agency (200 + envelope).
- [ ] `make db-setup` seeds MPP roles + demo data without error; `make seed-mpp` idempotent.
- [ ] Signin works from FE; staff token attaches on protected calls; kiosk/TV `X-API-Key`
      instance reaches a device-scoped endpoint.
- [ ] WS `GET /mpp/v1/ws` accepts a connection and returns a `snapshot` frame (can be empty).
- [ ] `go build ./...` + `yarn tsc:check` green.
