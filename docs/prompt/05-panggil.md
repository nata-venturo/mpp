# Slice 05 — Panggil (idle-longest, max 3× then skip)

> **Goal:** operator calls the next number to their loket; recall up to 3×; start serving.
> **State:** `WAITING → CALLED → SERVING` (no-show → `SKIPPED`).
> **KOMPLIT inti:** `CALLED → SERVING`; the **4th recall is rejected (`409`)**.

Read [`README.md`](./README.md) first. **This slice introduces WebSocket** — build P8 (hub)
and the FE client `src/lib/ws.ts` here.

## Depends on
- Slice 04 (`antrian` in `WAITING`, `/queue` read).
- P6 FE auth (loket = staff JWT), P7 `(loket)` route group, P8 backend WS hub.

## Contract (loket app — staff JWT, `mpp.antrian:update` / `mpp.queue:read`)

| Method | Path | Effect |
|--------|------|--------|
| POST | `/mpp/v1/loket/{id}/session` | open/close operator session (`loket_session`) |
| POST | `/mpp/v1/queue/next` `{loket_id}` | pick head of a served service → `CALLED`, `call_count=1` |
| POST | `/mpp/v1/antrian/{id}/recall` | `call_count += 1` (**≤3**; 4th → `409`) |
| POST | `/mpp/v1/antrian/{id}/start` | `CALLED → SERVING` (open `serving_session`) |
| POST | `/mpp/v1/antrian/{id}/skip` | no-show → `SKIPPED`; loket `last_idle_at=NOW()` |

Example `POST /queue/next`:
```json
{ "loket_id":"…" }        // → 200
{ "data": { "antrian_id":"…","nomor":"A-014","loket":"Loket 3","status":"CALLED",
            "call_count":1,"tts_text":"Nomor antrian A - nol satu empat, silakan menuju loket tiga" },
  "message":"Called" }
```
Errors: `409` on illegal transition (e.g. `start` on a non-`CALLED` item, `recall` at
`call_count==3`), `404` out of scope, `403` wrong loket/agency, `409`/`204` when the stream
is empty (choose `200` with `data:null` "no waiting").

## Allocation model (reconcile with docs)

Pull model: the **operator's own loket** calls next for a service that loket serves
(`loket_layanan`). "Idle-longest" (BR-12) is fairness ordering **when choosing among candidate
lokets in an auto/push scenario**; in this operator-pull flow the calling loket takes the item
and its `last_idle_at` is refreshed on `done`/`skip` so future auto-suggestions stay fair.
- **Mode** (`instansi.queue_mode`): `FIFO` → order `WAITING` by `queued_at ASC`.
  `BOOKING_PRIORITY` → bookings (`source=BOOKING`) ahead of `WALK_IN`, then `queued_at`.
  Reuse `antrian.priority` (set at enqueue from mode) + `queued_at` in the `ORDER BY`.
- Guard/pick head **atomically**: `UPDATE … SET status='CALLED', loket_id=$loket,
  called_at=NOW(), call_count=1 WHERE id = (SELECT id FROM … WHERE layanan_id=$svc AND
  status='WAITING' ORDER BY priority DESC, queued_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED)
  RETURNING …` — `FOR UPDATE SKIP LOCKED` prevents two lokets grabbing the same item.

## Backend — `apps/api/internal/modules/mpp/loket_ops`

- `dto/loket_ops.dto.go` — `CallNextRequest{LoketID}`, `SessionRequest`, `AntrianActionResponse`.
- `repository/loket_ops.repository.go`:
  - `OpenSession`/`CloseSession` (`loket_session`, unique active-per-loket index already exists).
  - `CallNext(ctx, tx, loketID, servedLayananIDs)` — the `SKIP LOCKED` pick above.
  - `Recall(ctx, id)` — `UPDATE … SET call_count=call_count+1 WHERE id=$1 AND status='CALLED'
    AND call_count < 3 RETURNING call_count` → **0 rows = 409** (already at 3, or not CALLED).
  - `Start(ctx, tx, id)` — `WHERE id=$1 AND status='CALLED'` → `SERVING`, `served_at=NOW()`;
    open `serving_session`.
  - `Skip(ctx, tx, id)` — `WHERE id=$1 AND status IN ('CALLED')` → `SKIPPED`; close
    `serving_session` if any (outcome `SKIPPED`); loket `last_idle_at=NOW()`.
- `service/loket_ops.service.go` — orchestrates the above; **builds `tts_text`** (see below);
  **publishes WS events** via the hub after each transition; enforces loket/agency scope
  (operator's `loket_session`).
- `handler` + `main.loket_ops.go`:
  ```go
  g := rg.Group(""); g.Use(middleware.JWTAuth())
  g.POST("/loket/:id/session", middleware.RequirePermission("mpp.queue:update"),  m.Handler.Session)
  g.POST("/queue/next",        middleware.RequirePermission("mpp.antrian:update"), m.Handler.CallNext)
  g.POST("/antrian/:id/recall",middleware.RequirePermission("mpp.antrian:update"), m.Handler.Recall)
  g.POST("/antrian/:id/start", middleware.RequirePermission("mpp.antrian:update"), m.Handler.Start)
  g.POST("/antrian/:id/skip",  middleware.RequirePermission("mpp.antrian:update"), m.Handler.Skip)
  ```
  (`recall`/`start`/`skip`/`call` all guard on `mpp.antrian:update` — the CRUD verb the level
  vocabulary actually grants; see [`../06-security/rbac-matrix.md`](../06-security/rbac-matrix.md).)

### `tts_text` (built server-side, consumed by TV in slice 06)
Ready-to-speak Indonesian, e.g. `"Nomor antrian A - nol satu empat, silakan menuju loket tiga"`.
Digits spoken individually (`014 → nol satu empat`), prefix as a letter, loket as a number
word. Keep the phrasing in a small helper (config `system_config` key `tts_text`, FR-CFG-03).

### WebSocket publish (via P8 hub, Redis pub/sub)
| Event | When | Payload (key fields) |
|-------|------|----------------------|
| `call.created` | `/queue/next` | `{antrian_id, nomor, loket, tts_text}` |
| `call.recalled` | `/recall` | `{antrian_id, nomor, loket, call_count, tts_text}` |
| `serving.started` | `/start` | `{antrian_id, loket}` |
| `queue.updated` | any change | `{layanan_id, waiting_count, next[]}` |
Publish to channels `layanan:<id>`, `loket:<id>`, `display:<instansi>`, `instansi:<prefix>`
per [`../04-api/websocket-events.md`](../04-api/websocket-events.md).

## Frontend — loket app + WS client

- **`src/lib/ws.ts`** (first WS use): singleton WebSocket to `GET /mpp/v1/ws` with the staff
  token; `subscribe` frame on open; auto-reconnect + re-`snapshot` on reconnect; dedupe by
  `antrian_id` + sequence. Expose a small `useQueueSocket(channels)` hook feeding TanStack
  cache / local state.
- api-layer: `endpoints.ts` loket ops; `loket-ops.ts` zod; `use-loket-ops.ts` mutations.
- Pages: `src/app/(loket)/page.tsx` (login + pick loket → open session) →
  `sections/loket/view/loket-panel-view.tsx`:
  - **SEKARANG** card (current `CALLED`/`SERVING` `nomor`, `call_count`).
  - One-tap actions: **Panggil berikutnya · Panggil ulang · Mulai · Lewati** (slice 06 adds
    **Selesai**). Disable illegally per state; on `409` show a toast.
  - Live **Menunggu** list from `useQueueSocket('layanan:<id>')`, falling back to `/queue`.
- Staff JWT required; agency/loket scope enforced server-side too.

## Tests

Backend (`.../loket_ops/...`):
- State machine table test: `CallNext` on empty → no-item; `WAITING→CALLED`; `recall` 1→2→3
  ok, **4th → 409**; `start` only from `CALLED`; `skip` from `CALLED` sets `last_idle_at`.
- **Concurrent CallNext** from two lokets on the same stream → each gets a *different* item
  (proves `FOR UPDATE SKIP LOCKED`), never the same `antrian_id`.
- WS: publishing a `call.created` reaches a subscribed test client (hub unit test / fake pub-sub).
- httptest: `/recall` at `call_count==3` → `409`; `/start` on `WAITING` → `409`.

Smoke:
```bash
curl -X POST .../mpp/v1/loket/$LOKET/session -H "Authorization: Bearer $STAFF" -d '{"action":"open"}'
curl -X POST .../mpp/v1/queue/next -H "Authorization: Bearer $STAFF" -d '{"loket_id":"'$LOKET'"}' | jq '.data|{nomor,call_count,tts_text}'
for i in 1 2 3; do curl -X POST .../mpp/v1/antrian/$AID/recall -H "Authorization: Bearer $STAFF"; done   # 4th → 409
curl -X POST .../mpp/v1/antrian/$AID/start -H "Authorization: Bearer $STAFF"
```

FE gate: `tsc:check` + `lint`; manual: open session → Panggil berikutnya shows a number and a
WS event lands; recall increments; 4th recall blocked; Mulai → SERVING.

## Out of scope
- **Selesai (`done`) + TV display** → slice 06 (this slice stops at `SERVING`).
- Hold/resume, transfer, second-service (FR-OPR-04/05) — add after the skeleton closes.
- Idle-longest auto-allocation/push mode — pull model is enough for the slice.

## Definition of Done
- [ ] `/queue/next` → `CALLED` (atomic, `SKIP LOCKED`, mode-aware); `/start` → `SERVING`.
- [ ] Recall capped at 3; 4th → `409` (DB `CHECK` + service guard).
- [ ] WS hub live; `call.created`/`call.recalled`/`serving.started`/`queue.updated` published;
      FE `ws.ts` receives them.
- [ ] `go test ./internal/modules/mpp/loket_ops/...` green (state + concurrent call); smoke pasted.
- [ ] Loket panel one-tap actions work; `tsc:check` + `lint` green.
