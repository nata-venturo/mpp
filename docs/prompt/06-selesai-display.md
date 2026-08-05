# Slice 06 — Selesai + Display (close service + TV shows number & loket)

> **Goal:** operator finishes serving (records duration); the TV shows the called number +
> loket and **speaks it** (offline TTS).
> **State:** `SERVING → DONE`.
> **KOMPLIT inti:** `→ DONE` records service duration; the TV displays nomor + loket (and
> announces it).

Read [`README.md`](./README.md) first. This slice closes the walking skeleton.

## Depends on
- Slice 05 (`loket_ops`, WS hub, `tts_text` on call events), slice 04 (`antrian`).
- P5 tv API-key (scope `mpp.display:read`, `mpp.queue:read`), P7 `(tv)` route group.

## Contract

- `POST /mpp/v1/antrian/{id}/done` — staff JWT (`mpp.antrian:update`):
  ```json
  200 { "data": { "antrian_id":"…","status":"DONE","done_at":"…Z","durasi_detik":312 },
        "message":"Done" }
  ```
  Effect: `SERVING → DONE`; close `serving_session` (`outcome=DONE`, `ended_at=NOW()`);
  `antrian.done_at=NOW()`; loket `last_idle_at=NOW()`. `409` if not `SERVING`.

- `GET /mpp/v1/display?instansi_id=…` — device (`X-API-Key`, `mpp.display:read`): TV snapshot.
  ```json
  200 { "data": { "instansi":{"name":"Dukcapil","prefix":"A"},
                  "current": {"nomor":"A-014","loket":"Loket 3","tts_text":"…"},
                  "next": ["A-015","A-016","A-017"] }, "message":"…" }
  ```

## Backend — extend `mpp/loket_ops` + new `mpp/display`

- `loket_ops`: add `Done(ctx, tx, id)` — `UPDATE … SET status='DONE', done_at=NOW() WHERE
  id=$1 AND status='SERVING' RETURNING …` (0 rows → `409`); close `serving_session`; refresh
  loket `last_idle_at`; compute `durasi_detik = ended_at - started_at`. Publish WS
  `serving.ended {antrian_id, outcome:"DONE"}` + `queue.updated`.
  Route: `g.POST("/antrian/:id/done", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Done)`.
- `mpp/display` module:
  - `repository/display.repository.go` — `Snapshot(ctx, instansiID)`: latest `CALLED`/`SERVING`
    per loket (current) + head of `WAITING` (next up).
  - `service` + `handler` + `main.display.go`:
    ```go
    d := rg.Group(""); d.Use(middleware.JWTAuth())
    d.GET("/display", middleware.RequirePermission("mpp.display:read"), m.Handler.Snapshot)
    ```
  - TV also subscribes WS channel `display:<instansi>`; the snapshot is for initial load /
    reconnect (offline resilience, NFR-AVAIL-02).

## Frontend — TV display (mini-PC, 3 windows, shared offline TTS)

- **Route:** `src/app/(tv)/display/[instansi]/page.tsx` → `sections/tv/view/display-view.tsx`.
  Full-screen, high-contrast, large type (NFR-UX-02): big **current call** (nomor + loket) +
  **next-up** list + optional running text. Uses the tv API-key instance + WS client.
- **Shared audio queue (BR-18):** one mini-PC drives 3 windows sharing one speaker. Elect a
  single **leader** via `BroadcastChannel` (fallback `localStorage` lock); only the leader
  plays audio. Incoming `call.created`/`call.recalled` push `tts_text` onto a FIFO; the leader
  plays **one at a time** (next waits for the current utterance to end — no overlap). Dedupe by
  `antrian_id` + sequence.
- **Offline TTS (NFR-AVAIL-02), preference order (docs-recommended):**
  1. **Pre-rendered audio fragments** — bundle Indonesian clips for digits (`nol`…`sembilan`),
     letters (`A`…), and fixed phrases (`"Nomor antrian"`, `"silakan menuju loket"`) under
     `public/audio/`; concatenate per `tts_text`. Deterministic, no engine, works offline.
     **Primary.**
  2. Local offline TTS engine (system voice) via a small local agent — fallback.
  3. `speechSynthesis` with a local Indonesian voice — last resort (availability varies).
  Store all assets **locally**; keep displaying the last-known snapshot during an outage.
- api-layer: `endpoints.ts` `mpp.display`; `display.ts` zod; `use-display.ts` +
  `useDisplaySocket(instansi)`.
- Loket panel: add the **Selesai** button (`useDoneMutation`) → on success the item clears and
  the loket goes idle.

## Tests

Backend:
- `Done`: `SERVING → DONE` records `done_at` + `durasi_detik > 0`; loket `last_idle_at`
  refreshed; `Done` on non-`SERVING` → `409`.
- `Snapshot`: after a `call.created` then `start`, the snapshot `current` shows that nomor +
  loket and `next` lists the following `WAITING` numbers in order.
- httptest: `/done` happy path + `409`; `/display` shape.

Smoke:
```bash
curl -X POST .../mpp/v1/antrian/$AID/done -H "Authorization: Bearer $STAFF" | jq '.data|{status,durasi_detik}'
curl "http://localhost:8080/mpp/v1/display?instansi_id=$IID" -H "X-API-Key: $TV_KEY" | jq .data
```

FE gate: `tsc:check` + `lint`; **manual e2e (full skeleton):** book (01) → QR (02) → check-in
(03) → number `A-014` `WAITING` (04) → loket Panggil → Mulai (05) → **Selesai** → TV shows
`A-014 · Loket 3` and speaks it once; open 2–3 TV windows and confirm audio **does not
overlap** (single leader).

## Out of scope
- Hold/transfer/second-service, FO verification gating, reporting/analytics, daily-reset
  worker, WhatsApp agent — post-skeleton (later delivery-plan phases).
- Running-text/broadcast admin controls (may stub the display area).

## Definition of Done
- [ ] `/done` → `DONE` with `durasi_detik`; `serving_session` closed `DONE`; loket idle refreshed; `409` if not `SERVING`.
- [ ] `GET /display` returns current call + next-up (device API-key).
- [ ] TV renders nomor + loket and speaks `tts_text` **offline**; 3 windows share one
      non-overlapping audio queue (single leader).
- [ ] `go test ./internal/modules/mpp/...` green; smoke pasted.
- [ ] **Full end-to-end skeleton** (slices 01→06) passes the manual e2e; `tsc:check` + `lint` green.
