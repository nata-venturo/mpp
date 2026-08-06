# Tech Stack — MPP _(EN)_

## Backend — `apps/api` (from `venturo-skeleton-go`)

| Concern        | Choice | Notes |
|----------------|--------|-------|
| Language       | Go 1.26 | static binaries, `CGO_ENABLED=0` |
| HTTP framework | Gin (`gin-gonic/gin`) | + `gin-contrib/cors`, ginzap logging |
| DB driver      | pgx v5 (`jackc/pgx/v5`) | **no ORM** — hand-written SQL repositories |
| Migrations     | golang-migrate | per-module folders `migrations/<module>/`; `MODULE=` Make targets |
| Cache/coord    | Redis (`redis/go-redis/v9`) | permission cache + queue counters/state/pub-sub |
| Auth           | JWT (`golang-jwt/jwt/v5`) + API-key | company-scoped RBAC; roles store `resource:level`, endpoints check `resource:action` (level expanded to fixed CRUD verbs) |
| Config         | `os.Getenv` (+ `config.GetDSN()`) | no `.env` autoload; export via compose/air |
| Logging        | zap | structured |
| Email          | `gomail.v2` (`pkg/email`) | SMTP |
| Storage        | GCS adapter (`pkg/storage`) | Supabase Storage (S3) as alternative |
| Realtime       | WebSocket hub (to add under `mpp`) | backed by Redis pub/sub |
| Layout         | modular monolith | `internal/modules/<domain>/<feature>/{domain,dto,handler,repository,service}` |

**MPP additions (new modules, not in the skeleton):** `internal/modules/mpp/{instansi,
layanan,loket,kuota,booking,checkin,antrian,display,loket_ops,fo,report}` +
`migrations/mpp/` + `seeders/mpp/`, registered in `internal/router/router.go`, plus a
WebSocket hub and worker jobs. **Module path renamed** to
`github.com/ndollem/mpp/apps/api`.

## Frontend — `apps/web` (from `venturo-skeleton-next.js`)

| Concern        | Choice | Notes |
|----------------|--------|-------|
| Framework      | Next.js 16 (App Router) | RSC + ISR; dev port 8002 |
| UI runtime     | React 19 | |
| Component lib  | MUI v9 (+ Emotion) | Zone UI theme; no Tailwind/shadcn |
| Server state   | TanStack Query v5 | |
| HTTP client    | `ky` | unauthenticated seam ready for auth hooks (`beforeRequest`/`afterResponse`) |
| Forms          | react-hook-form + Zod | |
| Language       | TypeScript (strict) | imports rooted at `src/…` (`baseUrl:"."`, no `@/`) |
| Package manager| **Yarn 1 (Classic)** | committed `yarn.lock`; kept as-is |
| Env            | Zod-validated (`src/lib/env.ts` → `CONFIG`) | `NEXT_PUBLIC_API_URL` → backend; `X-Company-Slug` tenant header |
| Lint/format    | ESLint 9 (flat) + Prettier 3 + Husky | `tsc:check` is the CI gate |

**MPP additions:** citizen registration flows, public queue status, kiosk (QR + walk-in +
print), TV display (+ TTS), loket app, FO app, admin dashboard, reporting — plus an
**auth layer** (token store + ky hooks) since the skeleton ships unauthenticated.

## Data & infra

| Concern     | Choice | Notes |
|-------------|--------|-------|
| Database    | PostgreSQL 16 | Supabase-compatible; schemas `core` + `mpp` |
| Cache/queue | Redis 7 | required; not provided by Supabase |
| Object store| GCS or Supabase Storage (S3) | QR, uploads, media |
| Local dev   | docker-compose + Make | postgres + redis + api + web |
| Prod        | Kubernetes | skeleton ships k8s manifests |

## Devices

| Device | Runtime | Purpose |
|--------|---------|---------|
| Kiosk  | Browser (Next route) + QR scanner + thermal printer | check-in, walk-in, ticket |
| TV set | Mini PC, one browser, 3 windows | queue display + offline Indonesian TTS (shared audio queue) |
| Loket  | Browser (Next route) | operator queue actions |

## Notable constraints carried from the skeletons

- Backend has **no compose file** upstream and does **not autoload `.env`** — the
  monorepo adds both.
- Frontend `NEXT_PUBLIC_*` are **build-time frozen** — per-environment rebuilds needed.
- Frontend is **Yarn 1** — do not migrate to pnpm/npm workspaces without intent
  (meta-repo keeps it isolated).
