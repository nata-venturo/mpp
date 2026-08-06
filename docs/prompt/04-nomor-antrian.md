# Slice 04 — Nomor antrian (Redis `INCR` → `A-014`)

> **Goal:** on check-in (and walk-in), allocate a queue number atomically and enter the stream.
> **State:** `CHECKED_IN → WAITING` (walk-in: `[*] → WAITING`).
> **KOMPLIT inti:** sequential, atomic, **no duplicates**; item enters `WAITING`.

Read [`README.md`](./README.md) first.

## Depends on
- Slice 03 (`/checkin` handler seam). This slice extends that handler and adds `POST /walkin`.
- Redis (`internal/shared/redis`, go-redis v9 — first use of `INCR`).
- Numbering is **per-instansi/day** (shared agency prefix). Migration
  `000004_antrian.up.sql` already enforces `UNIQUE (instansi_id, queue_date, nomor_seq)` and
  the counter is keyed per instansi — do not re-key per service.

## Contract

- `POST /mpp/v1/checkin` (extended): the `200` now also returns the allocated number:
  ```json
  { "data": { "booking_id":"…","status":"CHECKED_IN","antrian_id":"…","nomor":"A-014",
              "nomor_seq":14,"queue_status":"WAITING","eta_menit":25 }, "message":"Checked in" }
  ```
- `POST /mpp/v1/walkin` — device (`X-API-Key`, scope `mpp.booking:create`):
  ```json
  // request
  { "instansi_id":"…","layanan_id":"…","pemohon":{"name":"…","phone":"…"} }
  // 201
  { "data": { "antrian_id":"…","nomor":"A-015","nomor_seq":15,"queue_status":"WAITING",
              "eta_menit":30 }, "message":"Registered" }
  ```
- `GET /mpp/v1/queue?layanan_id=…` — staff (`mpp.queue:read`): current `WAITING` stream for a
  service, ordered by mode. `SuccessWithPagination` or a plain list + `meta`.
  ```json
  { "data": [ {"antrian_id":"…","nomor":"A-014","status":"WAITING","queued_at":"…Z"} ],
    "meta": {"pagination":{…}}, "message":"…" }
  ```

## Number allocation (the crux)

Redis counter is authoritative; DB unique index is the backstop.

```
key   = mpp:counter:<instansi_id>:<yyyymmdd>   (yyyymmdd in the operating-day local TZ)
seq   = redis.Incr(ctx, key)                    // atomic, returns the new value
nomor = format(prefix, seq)                     // e.g. "A-" + zero-pad(seq,3) → "A-014"
```
- **First call of the day** on a fresh key returns `1`. Set a TTL on the key (e.g. 36h) so
  stale counters self-expire; the daily-reset worker (later) also clears them.
- **Cold-start / Redis-flush safety:** if the key is missing but antrian rows already exist
  for today, seed the counter from `SELECT COALESCE(MAX(nomor_seq),0) FROM mpp.antrian WHERE
  instansi_id=$1 AND queue_date=CURRENT_DATE` before/after the first `INCR` (use `SETNX` +
  `INCR`, or `INCRBY` from the max). Document the chosen approach.
- **DB backstop:** insert `antrian` with `(instansi_id, queue_date, nomor_seq)`; the unique
  index makes any accidental duplicate a hard error (rollback + retry `INCR`).
- **Number format** is configurable (`system_config` key `number_format`, BR-04); default
  `<prefix>-<seq:03d>`. Keep formatting in one helper.

## Backend — `apps/api/internal/modules/mpp/antrian`

- `domain/antrian.go` — matches `000004` columns.
- `dto/antrian.dto.go` — `WalkInRequest`, `AntrianResponse`, `QueueQuery`, queue list item.
- `repository/antrian.repository.go`:
  - `NextSeq(ctx, instansiID, day)` — the Redis `INCR` (+ cold-start seeding).
  - `CreateAntrian(ctx, tx, a)` — insert `WAITING`, `source`, `queue_date`, `queued_at`;
    handle unique violation → signal caller to re-`INCR`.
  - `ListWaiting(ctx, layananID, page, limit)` — order per mode (see slice 05 for booking
    priority; here default `queued_at ASC`), index `idx_antrian_service_status_seq`.
- `service/antrian.service.go`:
  - `Enqueue(ctx, tx, EnqueueInput) (*Antrian, error)` — the shared allocation+insert used by
    **both** check-in and walk-in. Computes `nomor` via the format helper; sets `WAITING`.
  - `WalkIn(ctx, req)` — own `tx`: upsert pemohon → `Enqueue(source=WALK_IN)`.
  - `Queue(ctx, layananID, …)` — read stream.
  - `eta_menit` via BR-29: `ceil(position / max(open_eligible_lokets,1)) *
    estimasi_durasi_menit`. Position = count of `WAITING`/`CALLED` ahead in the service.
- `handler/antrian.handler.go`, `main.antrian.go` (`SetupRoutes(mppV1)`):
  ```go
  rg.GET("/queue", middleware.JWTAuth(), middleware.RequirePermission("mpp.queue:read"), m.Handler.Queue)
  wi := rg.Group(""); wi.Use(middleware.JWTAuth())
  wi.POST("/walkin", middleware.RequirePermission("mpp.booking:create"), m.Handler.WalkIn)
  ```

**Wire into slice-03 check-in:** inject `antrian.Service` into `checkin.Service`; inside the
same `tx` (after `MarkCheckedIn`) call `Enqueue(source=BOOKING, booking_id, pemohon_id,
instansi_id, layanan_id)`; return the number in the check-in response. One transaction ⇒
check-in and enqueue are all-or-nothing.

## Frontend

- **Kiosk ticket print** (check-in + walk-in): after success, render a print-CSS ticket
  (`nomor`, agency+service, date/time, `eta_menit`, "watch the TV / listen for your number")
  and call `window.print()` — no PII on paper. A `@media print` stylesheet scoped to the
  ticket. (Thermal via browser print; a raw ESC/POS local agent is a later hardening option.)
- **Walk-in flow** (`(kiosk)/walkin`): pick agency → service (+ syarat) → confirm → number +
  print. Reuse the citizen catalog api-layer (instansi/layanan) with the kiosk key instance.
- **Public queue status** (`(citizen)/status`): `useQueueQuery(layananId)` showing the
  current waiting stream / now-serving per agency (read-only, polling or WS later).
- api-layer: add `mpp.walkin`, `mpp.queue` to `endpoints.ts`; `antrian.ts` zod +
  `use-antrian.ts` hooks.

## Tests

Backend (`.../antrian/...`):
- **Concurrency:** N goroutines `Enqueue` for one instansi/day → numbers `1..N` with **no
  duplicate/gap** (assert the set); DB unique index never trips under the Redis counter.
- Cold-start: pre-seed antrian rows with `MAX(nomor_seq)=5`, flush the Redis key, next
  `Enqueue` → `6` (not `1`).
- Format: `format("A",14)=="A-014"`; config override respected.
- httptest: `/walkin` → `201` with `nomor`; extended `/checkin` returns `nomor` + `WAITING`;
  `/queue` lists the item.

Smoke:
```bash
curl -X POST .../mpp/v1/checkin -H "X-API-Key: $KIOSK_KEY" -d '{"token":"'$TOK'"}' | jq .data.nomor
curl -X POST .../mpp/v1/walkin  -H "X-API-Key: $KIOSK_KEY" -d '{"instansi_id":"'$IID'","layanan_id":"'$LID'","pemohon":{"name":"Budi","phone":"628"}}'
curl "http://localhost:8080/mpp/v1/queue?layanan_id=$LID" -H "Authorization: Bearer $STAFF"
```

FE gate: `tsc:check` + `lint`; manual: check-in → ticket prints with a number; walk-in →
number + ticket; public status shows the waiting item.

## Out of scope
- Calling / serving (slice 05); daily-reset worker + booking-expiry sweep (later).
- Real-time queue updates over WebSocket (slice 05 introduces WS; here `/queue` is polled).

## Definition of Done
- [ ] Check-in & walk-in allocate atomic, gap-checked numbers → `WAITING`; no duplicates
      under concurrency; cold-start reseeds from `MAX(nomor_seq)`.
- [ ] `nomor` format `A-014` (configurable); numbering per-instansi/day.
- [ ] `GET /queue` returns the waiting stream (RBAC `mpp.queue:read`).
- [ ] `go test ./internal/modules/mpp/antrian/...` green (concurrency + cold-start); smoke pasted.
- [ ] Kiosk prints a ticket; `tsc:check` + `lint` green.
