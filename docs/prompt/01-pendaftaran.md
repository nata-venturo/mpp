# Slice 01 — Pendaftaran (booking + atomic quota)

> **Goal:** citizen picks agency + service, sees remaining quota, submits a booking.
> **State:** `[*] → BOOKED`.
> **KOMPLIT inti:** `201` + status `BOOKED`; quota full → `409` with **no overbooking** under
> concurrency (NFR-DATA-02).

Read [`README.md`](./README.md) (shared conventions) and
[`00-prerequisites.md`](./00-prerequisites.md) first.

## Depends on
- P1 router group, P2 master modules (read), P4 demo seed (agency `A`, a service, quota rows),
  P7 `(citizen)` route group.
- QR token generation is **slice 02** — in this slice `booking.qr_token` stays `NULL`.

## Contract

### `GET /mpp/v1/availability` — public
Query: `instansi_id` (uuid, req), `layanan_id` (uuid, opt), `date` (YYYY-MM-DD, req).
```json
200 { "data": { "date": "2026-08-06", "kuota": 20, "terpakai": 7, "remaining": 13 }, "message": "…" }
```
No quota row for that date → `remaining: 0` (or `404`? use `200` with `remaining:0` + `kuota:0`).

### `POST /mpp/v1/booking` — public (rate-limited)
```json
// request
{ "instansi_id": "…", "layanan_id": "…", "tanggal": "2026-08-06",
  "pemohon": { "name": "Ibu Sari", "phone": "628…", "email": null, "nik": null } }
// 201
{ "data": { "id": "…", "status": "BOOKED", "instansi_id": "…", "layanan_id": "…",
            "tanggal": "2026-08-06", "channel": "WEB", "created_at": "…Z" },
  "message": "Booking created" }
```
Errors: `400` validation; `404` instansi/layanan not found or inactive; **`409` quota full**;
`429` rate limited.

## Backend — `apps/api/internal/modules/mpp/booking` (+ `mpp/kuota` for availability)

Mirror `internal/modules/core/company/` layer-by-layer.

**Files:**
- `domain/booking.go`, `domain/pemohon.go` — DB-shaped structs.
- `dto/booking.dto.go` — `CreateBookingRequest` (binding tags: `instansi_id` `required,uuid`;
  `tanggal` `required,datetime=2006-01-02`; nested `pemohon.name` `required,min=2`;
  `pemohon.phone` `required`), `BookingResponse`, `AvailabilityQuery`, `AvailabilityResponse`.
- `repository/booking.repository.go` — `UpsertPemohon(ctx, tx, p)` (dedupe by `phone`),
  `CreateBooking(ctx, tx, b)`, `GetByID(ctx, id)`.
- `repository/kuota.repository.go` (or in `mpp/kuota`) — `GetAvailability(ctx, instansiID,
  layananID *string, date)`, and the **atomic consume** below.
- `service/booking.service.go` — `Create(...)`, `Availability(...)`; sentinel errors
  `ErrQuotaFull`, `ErrInstansiInactive`.
- `handler/booking.handler.go` — bind → service → `response.*`.
- `main.booking.go` — `Initialize(db)` + `SetupRoutes(mppV1)`.

**Atomic quota consume (the crux — no ORM):** run inside the same `pgx.Tx` as the insert.
Prefer a per-service quota row; fall back to agency-wide (`jenis_layanan_id IS NULL`):

```sql
UPDATE mpp.kuota_booking
SET terpakai = terpakai + 1, updated_at = NOW()
WHERE instansi_id = $1
  AND tanggal = $2
  AND ( jenis_layanan_id = $3 OR ($3::uuid IS NULL AND jenis_layanan_id IS NULL) )
  AND terpakai < kuota
RETURNING id;
```
- 1 row → quota reserved. **0 rows → `ErrQuotaFull` → rollback → `409`.** The
  `terpakai < kuota` guard + row lock makes concurrent bookings safe (no overbook).
- Precedence: try `$3 = layanan_id` first; if that row doesn't exist at all, retry with
  `$3 = NULL` (agency-wide). Document which the demo seed uses.

**Service.Create flow:** validate instansi+layanan active (read) → `tx := db.Begin` →
atomic consume (409 on 0 rows) → `UpsertPemohon` → `CreateBooking` (status `BOOKED`, channel
`WEB`, `qr_token NULL`) → `tx.Commit` → map to `BookingResponse`.

**Routes (`main.booking.go`):** both **public** (no `JWTAuth`) — quota/state are the
authority, not RBAC:
```go
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
    rg.GET("/availability", m.Handler.Availability)
    rg.POST("/booking", /* rateLimit(), */ m.Handler.Create)
    rg.GET("/booking/:id", m.Handler.GetByID) // used by slice 02
}
```
Wire in `router.go`: `booking.Initialize(db).SetupRoutes(mppV1)`.

**Rate limiting (NFR-SEC-06):** add a lightweight per-IP+phone limiter (Redis `INCR` with
TTL) on `POST /booking`. If a limiter helper doesn't exist yet, a minimal Redis-backed one
is fine; **may be deferred** to a hardening pass — mark with a `// ponytail:` note if so.

## Frontend — `(citizen)` flow

api-layer trio in `apps/web/src/lib/api/` (copy `articles.ts`):
- `endpoints.ts`: `mpp.instansi.list`, `mpp.instansi.layanan(id)`, `mpp.availability`,
  `mpp.booking.create`, `mpp.booking.detail(id)`.
- `booking.ts`: zod schemas (`InstansiSchema`, `LayananSchema` incl. `syarat_dokumen[]` +
  `estimasi_durasi_menit`, `AvailabilitySchema`, `BookingSchema`), fetchers, query keys.
- `use-booking.ts`: `useInstansiQuery`, `useLayananQuery(instansiId)`,
  `useAvailabilityQuery(params)`, `useCreateBookingMutation()`.

Pages (page → view → section under `src/app/(citizen)/`):
- `daftar/page.tsx` → `sections/citizen/view/booking-view.tsx` composing:
  pick-instansi → pick-layanan (shows `syarat_dokumen` + duration) → pick-date (calendar +
  `remaining` from availability, disable full dates) → pemohon form.
- Form: `Field.Text` name/phone/email, optional NIK; `Field.DatePicker` tanggal; `Form` +
  `zodResolver`. On submit → `useCreateBookingMutation` → on success route to the confirm
  screen (built in slice 02). On `409` show "kuota tanggal ini penuh".
- Add paths to `src/routes/paths.ts`.

## Tests

Backend (`internal/modules/mpp/booking/service/booking_service_test.go`):
- Table-driven `Create`: cases `available→BOOKED`, `full→ErrQuotaFull`, `inactive layanan→
  error`. Use a test DB or a repo fake; for the atomic path a real-DB test is strongest.
- **Concurrency test:** seed `kuota=1`; fire N goroutines calling `Create`; assert exactly 1
  success and N-1 `ErrQuotaFull`, and final `terpakai == 1` (proves no overbook).
- httptest: `POST /mpp/v1/booking` on a full date → `409` + envelope has `message`.

Smoke (`curl`, after `make api-dev` + demo seed):
```bash
curl "http://localhost:8080/mpp/v1/availability?instansi_id=$IID&layanan_id=$LID&date=2026-08-06"
curl -X POST http://localhost:8080/mpp/v1/booking -H 'Content-Type: application/json' \
  -d '{"instansi_id":"'$IID'","layanan_id":"'$LID'","tanggal":"2026-08-06","pemohon":{"name":"Sari","phone":"628123"}}'
# repeat until quota exhausted → expect 409
```

FE gate: `yarn tsc:check` + `yarn lint`; manual: pick agency→service→date→submit → booking
created; full date shows quota-full message.

## Out of scope (later slices)
- QR token + confirm/QR screen (slice 02).
- Check-in, number allocation, queue (slices 03–04).
- Booking cancel/quota refund (BR-07) — add when cancel is needed.

## Definition of Done
- [ ] `POST /booking` → `201` `BOOKED`; concurrent-full test proves no overbook; `409` on full.
- [ ] `GET /availability` returns correct `remaining`.
- [ ] Module built to pattern; routes under `mppV1`; envelope + UTC honored.
- [ ] `go test ./internal/modules/mpp/booking/...` green; curl smoke pasted.
- [ ] FE booking flow works; `tsc:check` + `lint` green.
