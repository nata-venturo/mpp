# Slice 02 — Terbitkan QR (single-use token + expiry)

> **Goal:** every booking gets a single-use, time-bound QR token; the citizen sees/downloads it.
> **State/field:** `qr_token` (+ `qr_expires_at`) set on the `booking` row.
> **KOMPLIT inti:** token unique + time-bound; confirm screen renders the QR with a download.

Read [`README.md`](./README.md) first.

## Depends on
- Slice 01 (`mpp/booking` module + `POST /booking` + `GET /booking/{id}`).
- `pkg/crypto` (skeleton) for crypto-random token generation.

## Contract

Extend slice-01 `POST /mpp/v1/booking` response to include the token + expiry:
```json
201 { "data": { "id": "…", "status": "BOOKED", "qr_token": "<opaque>",
                "qr_expires_at": "2026-08-06T10:00:00Z", … },
      "message": "Booking created" }
```

`GET /mpp/v1/booking/{id}` — public (owner/token). Returns booking + token/expiry so the
confirm screen and a "re-open my QR" flow work:
```json
200 { "data": { "id":"…","status":"BOOKED","qr_token":"…","qr_expires_at":"…Z",
                "instansi":{…},"layanan":{…},"tanggal":"2026-08-06" }, "message":"…" }
```

## Token rules (security)
- **Crypto-random**, unguessable (≥128 bits, base64url). Generate with `pkg/crypto`.
- **Single-use** — invalidated on check-in / cancel / expiry (enforced in slice 03).
- **Time-bound** — `qr_expires_at` = end of the booking day (or a config window
  `system_config` key `checkin_window`, default: valid on `tanggal` until 23:59:59 local →
  stored UTC).
- **Uniqueness** — DB already enforces `booking_qr_token_key` unique partial index. Store the
  raw token, or a hash of it, in `booking.qr_token`. Security doc prefers hashed-where-feasible;
  for slice simplicity you MAY store raw, but note the trade-off with a `// ponytail:` comment
  and keep the column write in one place so hashing can be added later.
- **Do not** put PII in the token; it's an opaque handle to the booking.

## Backend — extend `mpp/booking`

- `service/booking.service.go`:
  - In `Create`, after the booking row is built and before insert, generate
    `qr_token` + compute `qr_expires_at`; persist both (single writer for the token so a hash
    swap is one-line later).
  - Add `IssueTokenExpiry(tanggal, cfg)` helper (pure, unit-testable).
- `repository/booking.repository.go`: `GetByID` selects `qr_token`, `qr_expires_at`,
  and joins instansi/layanan for the detail response. Handle the unique-violation on
  `qr_token` (astronomically rare) by regenerating once.
- `dto`: add `QRToken`, `QRExpiresAt` to `BookingResponse`; a richer `BookingDetailResponse`
  for `GET /booking/{id}`.

No new routes (both already declared in slice 01).

## Frontend — confirm + QR screen

- api-layer: add `useBookingDetailQuery(id)` to `use-booking.ts`; extend `BookingSchema`
  with `qr_token`, `qr_expires_at` (zod).
- **QR rendering:** add a QR component. Ladder: use a tiny lib — `qrcode.react` (SVG/canvas,
  ~1 dep) — encoding either the `qr_token` or a check-in URL wrapping it. Do **not** hand-roll
  QR encoding.
- Page: `src/app/(citizen)/booking/[id]/page.tsx` → `sections/citizen/view/booking-confirm-
  view.tsx`: shows agency·service·date, the QR, and **Download QR** (canvas → `toDataURL` →
  anchor download) + optional **Send to email** (calls a notification endpoint — email is a
  later concern; the button may be stubbed/hidden this slice).
- Slice-01 submit success now routes here (`paths.citizen.booking.detail(id)`).

## Tests

Backend (`.../booking/service/booking_service_test.go`, extend):
- `IssueTokenExpiry`: expiry falls on the booking day / respects a config window; UTC.
- Token generation: two bookings → distinct tokens; token is non-empty, URL-safe.
- httptest: `POST /booking` → response includes `qr_token` + `qr_expires_at`;
  `GET /booking/{id}` → returns them.

Smoke:
```bash
BID=$(curl -s -X POST .../mpp/v1/booking -d '…' | jq -r .data.id)
curl -s .../mpp/v1/booking/$BID | jq '.data | {qr_token, qr_expires_at}'
```

FE gate: `tsc:check` + `lint`; manual: submit booking → confirm screen renders QR →
Download saves an image; reopening `/booking/{id}` shows the same QR.

## Out of scope
- Validating/consuming the token — that's **slice 03** (check-in).
- Email/WhatsApp delivery of the QR (notifications, later phase).

## Definition of Done
- [ ] Booking create + detail return unique `qr_token` + `qr_expires_at` (UTC).
- [ ] Confirm screen renders scannable QR + working Download.
- [ ] `go test ./internal/modules/mpp/booking/...` green; curl smoke pasted.
- [ ] `tsc:check` + `lint` green.
