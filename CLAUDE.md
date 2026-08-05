# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

MPP — **Sistem Antrian Mall Pelayanan Publik**: a multi-agency public-service-mall
queue system. Registration via WhatsApp AI-agent / web / walk-in, QR kiosk check-in,
TV displays with offline Indonesian TTS voice calling, counter (loket) operator apps,
front-office document verification, admin dashboards, reporting.

## Monorepo shape — read this first

Lightweight **polyglot meta-repo**. Two apps with fully independent toolchains; the
root only orchestrates them via `Makefile` + `docker-compose.yml`. There is **no**
Turborepo/Nx/shared JS dependency graph — Go and Next don't share one.

- [apps/api/](apps/api/) — Go 1.26 + Gin + pgx v5 (no ORM). Has its own [CLAUDE.md](apps/api/CLAUDE.md).
- [apps/web/](apps/web/) — Next.js 16 + MUI 9 + TypeScript. Has its own [CLAUDE.md](apps/web/CLAUDE.md).
- [packages/api-contract/](packages/api-contract/) — shared FE↔BE contract (placeholder).
- [infra/supabase/](infra/supabase/) — Postgres/Supabase hosting notes.
- [docs/](docs/) — full requirement docs, the **source of truth for the MPP domain**.

**When working inside an app, defer to that app's CLAUDE.md** — it holds the module
patterns, conventions, and gotchas. This root file is only for cross-app orchestration
and the big picture.

> Both apps were **vendored** (copied whole, no upstream git history) from Venturo
> skeletons: api from `venturo-skeleton-go`, web from `venturo-skeleton-next.js`. Their
> CLAUDE.md files still carry some upstream naming (e.g. "Tuai", "marketplace-be",
> Venturo marketing verticals) — treat those as skeleton context, not MPP truth. The
> MPP domain lives in `docs/` and in the `mpp` DB migration module.

## Commands (from repo root)

```bash
make bootstrap   # install Go modules + Yarn deps, copy .env files
make up          # start infra only (postgres + redis)
make up-full     # start full stack in containers (postgres + redis + api + web)
make db-setup    # run migrations + seeders (core, then mpp)
make db-reset    # drop, recreate, migrate, seed
make api-dev     # backend hot-reload (air)  → http://localhost:8080
make web-dev     # frontend dev server       → http://localhost:8002
make check       # backend tests + frontend type-check (the CI gate)
make help        # full list
```

Per-app commands (tests, lint, migrations, single-test) live in each app's Makefile /
package.json and its CLAUDE.md. Root `make` targets just delegate (e.g. `make api-test`
→ `cd apps/api && go test ./...`).

## Architecture big picture

**Backend** ([apps/api/](apps/api/)) is a modular monolith. Modules under
`internal/modules/<domain>/<module>/` each split into `domain/dto/handler/repository/
service` + a `main.<module>.go` exposing `Initialize()` + `SetupRoutes()`, all wired in
[internal/router/router.go](apps/api/internal/router/router.go). Multi-tenant: filter by
`company_id` everywhere; middleware chain `JWTAuth → CompanyContext → RequirePermission`.
**Redis is load-bearing** — it backs the permission cache *and* the queue engine (ticket
counters, active state, pub/sub); router bootstrap treats a failed Redis connect as fatal.

**Two-schema DB.** Migrations are split into two independently-tracked modules:
- `internal/database/migrations/core/` → table `schema_migrations_core` (auth, users, RBAC, companies, branches, approvals, clients — skeleton infra).
- `internal/database/migrations/mpp/` → table `schema_migrations_mpp` (the queue domain: master, registration, antrian, config).

`make migrate-*` targets take `MODULE=core` or `MODULE=mpp`.

The MPP queue domain is implemented as the walking skeleton of
[docs/prompt/](docs/prompt/): modules `instansi · loket · kuota · booking · checkin ·
antrian · loket_ops · display · config · ws` under `internal/modules/mpp/`, all wired
into a `/mpp/v1` router group. Numbering runs on a Redis `INCR` counter per
agency/day (DB unique index as the backstop); realtime fan-out is a WebSocket hub over
Redis pub/sub.

**Two rules the MPP modules depend on, easy to break by accident:**
- Any repository read that happens *inside* a transaction takes a `dbx.Querier` and is
  handed the `pgx.Tx`. Reaching for the pool while a transaction is open checks out a
  second connection, and once concurrent transactions outnumber the pool everything
  deadlocks — a failure that only shows up under load.
- MPP integration tests share one database, so run them with `-p 1`
  (`cd apps/api && make test-mpp`). In parallel, one package's `TRUNCATE` lands inside
  another package's transaction.

**Frontend** ([apps/web/](apps/web/)) follows a strict **page → view → section** App
Router pattern and a canonical ky+zod+TanStack API layer in `src/lib/api/`. See its
CLAUDE.md before touching UI.

**Realtime**: WebSocket (queue calls → TV / loket / kiosk) over Redis pub/sub.

## Database & Supabase

Local dev uses Postgres + Redis from `docker-compose.yml`. Supabase is an option for
managed Postgres (`DB_SSLMODE=require`, pooler endpoint) **but provides no Redis** —
which the backend requires for both permission cache and the queue engine — so a
separate Redis (local compose or hosted e.g. Upstash) is mandatory. See
[infra/supabase/README.md](infra/supabase/README.md).

## Docs language convention

Business docs (PRD/FRD/business-rules/UI) are written in **Bahasa Indonesia**; technical
docs (architecture/API/domain/security) in **English**. Start at
[docs/README.md](docs/README.md).
