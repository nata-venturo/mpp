# MPP Walkthrough — from `make` to a manual end-to-end test

How to start the system and walk the whole happy path by hand: a citizen books a
slot, gets a QR, checks in at the kiosk, receives number `A-014`, an operator calls
and serves them, and the TV shows and speaks the number.

Follow it top to bottom the first time. Steps 1–3 are one-time setup; steps 4 onward
are the daily loop.

---

## 0. What must already be running

| Thing | Where | Check |
|---|---|---|
| PostgreSQL 16 (15 works) | `localhost:5432` | `psql -h localhost -U postgres -l` |
| Redis (or Memurai on Windows) | `localhost:6379` | `redis-cli ping` → `PONG` |
| Go 1.26 | | `go version` |
| Node ≥ 22.12 + Yarn 1 | | `node -v && yarn -v` |

Redis is **load-bearing**, not optional: it backs the permission cache, the queue-number
counter, and the WebSocket fan-out. The API refuses to boot without it.

If you use the bundled Docker setup instead of local services, `make up` starts Postgres
and Redis for you. On a machine with Postgres/Memurai installed natively (no Docker),
skip `make up` — the services are already there.

---

## 1. Install dependencies and create the env files

```bash
make bootstrap
```

This downloads Go modules, runs `yarn install`, and copies `.env.example` → `.env` in the
root, `apps/api/`, and `apps/web/` (existing `.env` files are left alone).

Now open `apps/api/.env` and confirm the database block matches your Postgres:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=<your password>
DB_NAME=mpp
TZ=UTC                       # do not change — storage is UTC
MPP_LOCAL_TZ=Asia/Jakarta    # operating day for booking dates & queue numbers
MPP_COMPANY_ID=a1000000-0000-0000-0000-000000000001
```

Create the database if it does not exist yet:

```bash
psql -h localhost -U postgres -c "CREATE DATABASE mpp;"
```

`apps/web/.env` already carries the demo device keys and needs no edit for local work.

---

## 2. Migrate and seed

```bash
make db-setup
```

Runs both migration modules (`core`, then `mpp`) and every seeder. Afterwards you have:

- 3 agencies — Dukcapil (prefix `A`, FIFO), Imigrasi (`B`, booking-priority), BPJS (`C`)
- 7 services with document requirements
- 6 counters, all `OPEN`
- agency-wide quota of **30 per day** for today .. today+6
- MPP roles, staff users, and two scoped device API keys

Verify:

```bash
psql -h localhost -U postgres -d mpp -c "SELECT name, prefix, queue_mode FROM mpp.instansi;"
```

To wipe transactional data and start clean later, `make db-reset` (drops and rebuilds
everything).

---

## 3. Credentials and IDs you will need

**Staff logins** — all share the password `Petugas2026*`:

| Login | Role |
|---|---|
| `petugas@mpp.test` | counter operator (call/serve) |
| `fo@mpp.test` | front office |
| `supervisor@mpp.test` | supervisor |
| `adminmpp@mpp.test` | MPP admin |

**Device API keys** (demo values — rotate before any real deployment):

```
kiosk  wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90
tv     wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978
```

**Seeded UUIDs** used throughout this guide:

```bash
IID=a2000000-0000-0000-0000-000000000001   # Dukcapil
LID=a3000000-0000-0000-0000-000000000002   # Pencetakan / Perpanjangan KTP-el (10 min)
LOKET=a5000000-0000-0000-0000-000000000001 # Loket 1
```

---

## 4. Start the two apps

Two terminals, both from the repo root:

```bash
make api-dev     # backend, hot reload (air)  → http://localhost:8080
make web-dev     # frontend dev server        → http://localhost:8002
```

Health check:

```bash
curl http://localhost:8080/health
curl -s http://localhost:8080/mpp/v1/instansi | head -c 200
```

The second command should list the three seeded agencies. If it 404s, the API is an old
binary — stop it and let air rebuild.

---

## 5. Manual end-to-end test (browser)

Open the screens in **separate browser windows** — this mirrors the real deployment,
where the citizen is on a phone, the kiosk and TV are in the lobby, and the operator sits
at a counter.

### Slice 01 — Pendaftaran (booking + quota)

1. Go to **http://localhost:8002/daftar**.
2. Pick **Dinas Kependudukan dan Pencatatan Sipil** → **Pencetakan / Perpanjangan KTP-el**.
   The document requirements appear under the service.
3. Leave the date as today. A green chip shows **Sisa kuota: 30** (it drops as you book).
4. Fill name and WhatsApp number, then **Daftar sekarang**.

✅ You land on the confirmation screen. ❌ If the date is full you get
*"Kuota tanggal ini sudah penuh"* and the submit button is disabled — that is the
`409` path, not a bug.

**Prove there is no overbooking** (optional): shrink today's quota to 1 and submit twice.

```bash
psql -h localhost -U postgres -d mpp -c \
  "UPDATE mpp.kuota_booking SET kuota = 1, terpakai = 0
   WHERE instansi_id = 'a2000000-0000-0000-0000-000000000001'
     AND jenis_layanan_id IS NULL AND tanggal = CURRENT_DATE;"
```

The second booking is refused. Restore with `... SET kuota = 30`.

### Slice 02 — Terbitkan QR

On the confirm screen (`/booking/<id>`):

1. Agency, service, date, and name are shown.
2. A **QR code** is rendered, with *"Berlaku sampai …"* underneath (end of the booking
   day, in your local time).
3. Click **Unduh QR** — a PNG downloads.
4. Reload the page: the same QR reappears. It is stored on the booking, not regenerated.

**Keep this window open** — the kiosk needs to read the QR in the next step.

### Slice 03 + 04 — Check-in and queue number

1. Open **http://localhost:8002/kiosk** in a second window → **Check-in dengan QR**.
2. The camera preview starts and the screen says *"Menunggu pemindaian…"*.

Hold the QR from the confirm window up to the camera — decoding runs locally (jsQR), ten
frames a second, no network involved.

> **The camera needs a secure context.** `http://localhost:8002` counts as secure even
> though Chrome labels it *"not secure"*. A LAN address like `http://192.168.1.20:8002`
> does **not** — the browser hides `getUserMedia` entirely there, and the kiosk falls
> back to scanner-only. Serve over HTTPS for a real kiosk on the network.
>
> Camera is enabled only on `/kiosk/*`, via `Permissions-Policy: camera=(self)` in
> `next.config.ts`. Every other route keeps `camera=()`. If you move the scanner to a new
> route, extend that rule — otherwise the permission dialog never appears and the console
> logs *"Permissions policy violation: camera is not allowed in this document"*.

A USB QR scanner also works, and needs no camera: it behaves as a keyboard, typing the
token and pressing Enter. The kiosk listens for that at all times. To simulate it, click
the page, paste the token, and press **Enter**. Get the token with:

```bash
curl -s http://localhost:8080/mpp/v1/booking/<booking-id> | jq -r .data.qr_token
```

✅ A ticket appears: big number **A-001** (or the next in sequence), agency, service,
timestamp, and estimated wait. **Cetak karcis** opens the print dialog with only the
ticket on the page.

**Now scan the same QR again.** ✅ You get *"QR ini sudah dipakai untuk check-in"* — the
token is single-use, enforced by the database, not by the screen.

**Walk-in** (no booking): back on `/kiosk` → **Daftar tanpa booking** → pick agency and
service, enter a name and phone → you get the next number and the same printable ticket.

### Slice 05 — Panggil (call, recall, serve)

1. Open **http://localhost:8002/loket** in a third window. You are redirected to
   `/signin`.
2. Sign in as `petugas@mpp.test` / `Petugas2026*` → you land back on the panel.
3. Pick **Dukcapil** → **Loket 1** → **Buka sesi**.
4. **Panggil berikutnya** → the *Sekarang* card shows the number, status `CALLED`, and
   *Panggilan ke-1*. The **Menunggu** row below lists who is still waiting.
5. **Panggil ulang** twice → *Panggilan ke-2*, then *ke-3*.
6. **Panggil ulang** a fourth time → ❌ a toast says the action does not apply. That is
   BR-16: three calls maximum, then the number should be skipped.
7. **Mulai** → status becomes `SERVING`.

Try **Mulai** on a number that was never called, and it is refused the same way — the
state machine is enforced server-side, the disabled buttons are only a convenience.

**Two counters at once** (optional): sign in as `supervisor@mpp.test` in another window,
open **Loket 2**, and press *Panggil berikutnya* in both windows at the same moment. Each
counter gets a **different** person — never the same one.

### Slice 06 — Selesai and the TV display

1. Open **http://localhost:8002/display/a2000000-0000-0000-0000-000000000001** in a
   fourth window (full screen, `F11`).
2. The called number appears in large type with its counter, and the next numbers are
   listed below. The chip in the corner reads **Suara aktif**.
3. The TV **speaks** the announcement:
   *"Nomor antrian A - nol nol satu, silakan menuju loket satu"*.
4. Back on the loket panel, press **Selesai** → the counter clears, and the TV drops that
   number from *Sedang dipanggil*.

**Prove the shared audio queue (BR-18):** open the same display URL in **two or three
windows**. Only one shows **Suara aktif** — the others read *Suara di layar lain*. Call
another number: exactly one voice speaks, once. Close the leader window and within a few
seconds another window takes over the badge. This is what stops three TVs on one mini-PC
from talking over each other.

> Audio uses the browser's offline speech engine and needs an Indonesian system voice.
> If your machine has none, the display still works — it just stays silent. Chrome also
> blocks audio until the page has been interacted with once: click the display window.

---

## 6. Same flow, from the terminal

Faster than clicking, and it is the fastest way to see the contract. Paste as one block:

```bash
IID=a2000000-0000-0000-0000-000000000001
LID=a3000000-0000-0000-0000-000000000002
LOKET=a5000000-0000-0000-0000-000000000001
D=$(date +%Y-%m-%d)
KIOSK="wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
TV="wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978"
STAFF=$(curl -s -X POST http://localhost:8080/core/v1/auth/signin \
  -H 'Content-Type: application/json' \
  -d '{"login":"petugas@mpp.test","password":"Petugas2026*"}' | jq -r .data.access_token)

# 01 — availability + booking
curl -s "http://localhost:8080/mpp/v1/availability?instansi_id=$IID&layanan_id=$LID&date=$D" | jq .data
BR=$(curl -s -X POST http://localhost:8080/mpp/v1/booking -H 'Content-Type: application/json' \
  -d "{\"instansi_id\":\"$IID\",\"layanan_id\":\"$LID\",\"tanggal\":\"$D\",
       \"pemohon\":{\"name\":\"Ibu Sari\",\"phone\":\"628123456789\"}}")
BID=$(echo "$BR" | jq -r .data.id); TOK=$(echo "$BR" | jq -r .data.qr_token)

# 02 — the QR token lives on the booking
curl -s "http://localhost:8080/mpp/v1/booking/$BID" | jq '.data|{qr_token,qr_expires_at}'

# 03 + 04 — check-in issues the number; a second scan is refused
curl -s -X POST http://localhost:8080/mpp/v1/checkin -H "X-API-Key: $KIOSK" \
  -H 'Content-Type: application/json' -d "{\"token\":\"$TOK\"}" | jq '.data|{nomor,queue_status,eta_menit}'
curl -s -o /dev/null -w 'reuse → %{http_code}\n' -X POST http://localhost:8080/mpp/v1/checkin \
  -H "X-API-Key: $KIOSK" -H 'Content-Type: application/json' -d "{\"token\":\"$TOK\"}"

# 04 — walk-in and the waiting stream
curl -s -X POST http://localhost:8080/mpp/v1/walkin -H "X-API-Key: $KIOSK" \
  -H 'Content-Type: application/json' \
  -d "{\"instansi_id\":\"$IID\",\"layanan_id\":\"$LID\",\"pemohon\":{\"name\":\"Budi\",\"phone\":\"628111\"}}" \
  | jq '.data|{nomor,eta_menit}'
curl -s "http://localhost:8080/mpp/v1/queue?layanan_id=$LID" -H "Authorization: Bearer $STAFF" \
  | jq '[.data[].nomor]'

# 05 — open the counter, call, recall to the cap, start
curl -s -X POST "http://localhost:8080/mpp/v1/loket/$LOKET/session" -H "Authorization: Bearer $STAFF" \
  -H 'Content-Type: application/json' -d '{"action":"open"}' | jq '.data|{loket,is_active}'
CN=$(curl -s -X POST http://localhost:8080/mpp/v1/queue/next -H "Authorization: Bearer $STAFF" \
  -H 'Content-Type: application/json' -d "{\"loket_id\":\"$LOKET\"}")
echo "$CN" | jq '.data|{nomor,call_count,tts_text}'
AID=$(echo "$CN" | jq -r .data.antrian_id)
for i in 1 2 3; do
  printf 'recall %d → %s\n' "$i" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8080/mpp/v1/antrian/$AID/recall \
       -H "Authorization: Bearer $STAFF")"
done
curl -s -X POST http://localhost:8080/mpp/v1/antrian/$AID/start -H "Authorization: Bearer $STAFF" | jq '.data.status'

# 06 — finish and read the TV snapshot
curl -s -X POST http://localhost:8080/mpp/v1/antrian/$AID/done -H "Authorization: Bearer $STAFF" \
  | jq '.data|{status,durasi_detik}'
curl -s "http://localhost:8080/mpp/v1/display?instansi_id=$IID" -H "X-API-Key: $TV" | jq .data
```

**Expected:** `201` `BOOKED` with a token · check-in `200` with `A-00x` `WAITING` · reuse
`409` · call `200` with `tts_text` · recalls `200 200 409` (the number was already called
once, so the third recall is the fourth call) · `done` with `durasi_detik` · a display
snapshot with `current` and `next`.

---

## 7. Automated tests

```bash
cd apps/api && make test-mpp     # MPP suite against the dev database
cd apps/api && go build ./...    # compile check
cd apps/web && yarn tsc:check && yarn lint
```

`make test-mpp` runs with `-p 1` on purpose. The MPP packages share one database, and in
parallel one package's `TRUNCATE` lands inside another package's transaction, producing
deadlocks that have nothing to do with the code.

The tests cover the parts that are hard to check by hand: 20 concurrent bookings against
a 1-seat quota (exactly one wins), 25 concurrent registrations (numbers 1..25, no
duplicate, no gap), a Redis flush mid-day (numbering resumes from the database maximum,
never restarts at 1), and two counters calling at the same instant (always two different
people).

---

## 8. When something looks wrong

| Symptom | Cause | Fix |
|---|---|---|
| API exits: `Failed to connect to Redis … MISCONF` | Redis/Memurai cannot write its snapshot | Fix the data directory permissions, or `redis-cli config set stop-writes-on-bgsave-error no` as a stopgap |
| API exits: `Failed to connect to database` | env not loaded, or `mpp` database missing | Start it through `make api-dev` (it loads `.env`); create the database |
| `/mpp/v1/...` returns `404` | an old binary still holds port 8080 | Stop the stale process, let air rebuild |
| Everything returns `401` | no credentials | Staff need `Authorization: Bearer`, devices need `X-API-Key` |
| Authenticated but `403` | wrong scope | The TV key cannot read `/loket`; the kiosk key cannot call a number. That is the point |
| Booking always `409` | today's quota is exhausted | Raise `kuota`, reset `terpakai`, or book a later date |
| Kiosk says *"belum dikonfigurasi"* | device key missing from the bundle | Set `NEXT_PUBLIC_KIOSK_API_KEY` / `NEXT_PUBLIC_TV_API_KEY` in `apps/web/.env` and restart `web-dev` — `NEXT_PUBLIC_*` is baked in at build time |
| No camera prompt; console says *"Permissions policy violation: camera is not allowed"* | the route is not covered by `camera=(self)` | Extend the `/kiosk/:path*` header rule in `next.config.ts`, then restart the dev server (config changes are not hot-reloaded) |
| Camera silently unavailable, scanner still works | insecure origin | Use `http://localhost:8002`, not a LAN IP; or serve over HTTPS |
| TV silent | no Indonesian system voice, or audio not unlocked | Install a voice; click the window once to satisfy the browser's autoplay policy |
| TV shows a stale `CALLED` number | that ticket really is still called | Finish or skip it from the loket panel |

Reset the demo data at any time:

```bash
make db-reset     # drop, migrate, seed
```

---

## What is deliberately not here yet

The skeleton stops where `docs/prompt/` stops. Not built: hold/transfer/second service,
front-office document verification gating, reporting and analytics, the daily-reset
worker, the booking-expiry sweep, WhatsApp registration, and admin CRUD for master data
(reads only). See the per-slice **Out of scope** sections and
[`../08-roadmap/delivery-plan.md`](../08-roadmap/delivery-plan.md).
