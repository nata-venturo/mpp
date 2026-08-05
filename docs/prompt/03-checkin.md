# Slice 03 — Check-in (device, `X-API-Key`)

> **Goal:** at the kiosk, scan the QR token; validate it; reject reuse/expired/wrong-day.
> **State:** `BOOKED → CHECKED_IN`.
> **KOMPLIT inti:** valid token → `CHECKED_IN`; reused/expired token → rejected (`409`/`410`).

Read [`README.md`](./README.md) first.

## Depends on
- Slices 01–02 (`booking` with `qr_token` + `qr_expires_at`).
- P5 kiosk API-key (scoped `mpp.checkin:create`), P7 `(kiosk)` route group.
- **Seam with slice 04:** this slice makes `/checkin` flip the booking to `CHECKED_IN`.
  Slice 04 extends the *same handler* to allocate a number and create the `antrian`
  (`WAITING`). Keep `service.CheckIn` returning a result struct that slice 04 can enrich.

## Contract

### `POST /mpp/v1/checkin` — device (`X-API-Key`)
```json
// request
{ "token": "<qr_token>" }
// 200 (this slice)
{ "data": { "booking_id": "…", "status": "CHECKED_IN", "checked_in_at": "…Z",
            "instansi": {"name":"Dukcapil","prefix":"A"}, "layanan": {"name":"Perpanjang KTP"} },
  "message": "Checked in" }
```
Errors:
- `400` missing/malformed token.
- **`409`** token already used (`booking.status != BOOKED`) — reuse rejected.
- **`410`** (or `409`) token expired / wrong day (`now > qr_expires_at` or `tanggal != today`).
- `404` token not found.
- `401/403` missing/insufficient API-key scope.

Auth: route uses `middleware.JWTAuth()` (accepts the `X-API-Key`) +
`middleware.RequirePermission("mpp.checkin:create")`. The kiosk key carries that scope (P5).

## Backend — `apps/api/internal/modules/mpp/checkin`

Small module; depends on the booking repository (inject it, like `core/company` injects
`branchRepo`).

- `dto/checkin.dto.go` — `CheckInRequest{Token string `binding:"required"`}`,
  `CheckInResponse` (+ fields slice 04 will add).
- `repository/checkin.repository.go` — or reuse `booking.repository`:
  - `FindBookingByToken(ctx, token)` — select booking + instansi(prefix,name) + layanan(name).
    If tokens are hashed (slice 02 note), hash the input before lookup.
  - `MarkCheckedIn(ctx, tx, bookingID)` — `UPDATE mpp.booking SET status='CHECKED_IN',
    checked_in_at=NOW(), updated_at=NOW() WHERE id=$1 AND status='BOOKED' RETURNING id`
    (the `status='BOOKED'` guard makes reuse a **0-row → 409**, race-safe).
- `service/checkin.service.go` — `CheckIn(ctx, token) (*Result, error)`:
  1. `FindBookingByToken`; nil → `ErrTokenNotFound` (404).
  2. `booking.status != BOOKED` → `ErrTokenUsed` (409).
  3. `now > qr_expires_at` OR `booking.tanggal != today(local)` → `ErrTokenExpired` (410).
  4. `tx`: `MarkCheckedIn` (0 rows → `ErrTokenUsed`, concurrent double-scan) → **[slice 04
     inserts here]** → commit.
  5. Return `Result{BookingID, Instansi, Layanan, CheckedInAt}`.
  - Sentinels map to status in the handler.
- `handler/checkin.handler.go` — bind → `CheckIn` → switch on sentinel → `response.*`.
- `main.checkin.go` — `Initialize(db, bookingRepo)` + `SetupRoutes(mppV1)`:
  ```go
  chk := rg.Group("")
  chk.Use(middleware.JWTAuth())
  chk.POST("/checkin", middleware.RequirePermission("mpp.checkin:create"), m.Handler.CheckIn)
  ```

**Debounce double-scan** (BR/kiosk): the `status='BOOKED'` guard already makes the second
concurrent scan a 409; the kiosk UI should also ignore repeat scans within a short window.

## Frontend — `(kiosk)` check-in

- **API-key ky instance** (P6): a separate client attaching `X-API-Key` (build-config), not
  the user token.
- api-layer: `endpoints.ts` `mpp.checkin`; `checkin.ts` zod (`CheckInResultSchema`);
  `use-checkin.ts` `useCheckInMutation()`.
- **QR scanner** — USB HID scanners emulate a keyboard and end a scan with `Enter`. Capture
  into a focused hidden `<input>`; on `Enter` submit the buffered value. Ladder: this needs
  **no camera library** for HID scanners. If camera-based scanning is required, use a small
  lib (`html5-qrcode`/`jsQR`) — do not hand-roll decoding.
- Pages: `src/app/(kiosk)/page.tsx` (idle: big **Check-in QR** / **Walk-in** buttons) →
  `src/app/(kiosk)/checkin/page.tsx` → `sections/kiosk/view/checkin-view.tsx`:
  - hidden input captures scan → `useCheckInMutation` → success shows confirmation (number +
    ticket come in slice 04); error shows a clear message (used / expired / wrong-day) +
    "minta bantuan petugas".
- Full-screen, touch-first, high-contrast layout; route gated by API-key context, not login.

## Tests

Backend (`.../checkin/service/checkin_service_test.go`):
- Table-driven `CheckIn`: `valid→CHECKED_IN`; `already CHECKED_IN→ErrTokenUsed`;
  `expired (now>expires)→ErrTokenExpired`; `wrong day→ErrTokenExpired`; `unknown token→
  ErrTokenNotFound`.
- **Reuse race:** two concurrent `CheckIn` on the same valid token → exactly one success,
  one `ErrTokenUsed` (proves the `status='BOOKED'` guard).
- httptest: reused token → `409`; expired → `410`.

Smoke:
```bash
TOK=$(curl -s .../mpp/v1/booking/$BID | jq -r .data.qr_token)
curl -X POST .../mpp/v1/checkin -H "X-API-Key: $KIOSK_KEY" -H 'Content-Type: application/json' -d '{"token":"'$TOK'"}'
# second call → 409 (reuse)
```

FE gate: `tsc:check` + `lint`; manual: scan (or type token + Enter) valid → CHECKED_IN;
re-scan → clear "sudah dipakai" message.

## Out of scope
- Number allocation + `WAITING` + ticket print → **slice 04** (same `/checkin` handler seam).
- Walk-in registration (kiosk without booking) — add alongside slice 04 (`POST /walkin`).
- Booking-expiry sweep (`BOOKED → EXPIRED` worker) — later.

## Definition of Done
- [ ] Valid token → `CHECKED_IN` + `checked_in_at` (UTC); booking guarded so reuse → `409`.
- [ ] Expired / wrong-day → `410`/`409` with clear message.
- [ ] Route behind `JWTAuth` + `mpp.checkin:create`; kiosk key works, others rejected.
- [ ] `go test ./internal/modules/mpp/checkin/...` green (incl. reuse race); curl smoke pasted.
- [ ] Kiosk check-in screen works with HID scan; `tsc:check` + `lint` green.
