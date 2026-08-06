# RBAC Matrix — MPP _(EN)_

Maps MPP roles to the skeleton's permission model.

## How permissions actually work (skeleton)

Two representations, connected by a fixed expansion — **know both** before reading the
matrix:

1. **Stored on the role** — `core.roles.permissions` is JSONB in the form
   `{"resource": "level"}`, where `level` ∈ `viewer | editor | admin`
   (`internal/modules/core/role/domain/role.go`). Roles are seeded `is_system` and assigned
   via `core.user_roles`.
2. **Checked on the endpoint** — handlers guard with
   `RequirePermission("resource:action")` (e.g. `mpp.antrian:update`). At login the stored
   levels are **expanded** to a flat `resource:action` list
   (`Permissions.ToStringList()`), cached in Redis, and matched by exact string
   (`internal/shared/authz/service.go`).

### Level → action vocabulary (fixed)

From `internal/modules/core/role/domain/permission_levels.go` — this is the **single
source of truth**; a level grants exactly these actions and no others:

| Level | Expands to actions |
|-------|--------------------|
| `viewer` | `read` |
| `editor` | `create`, `read`, `update`, `delete` |
| `admin` | `create`, `read`, `update`, `delete`, `export`, `import`, `restore`, `approve` |

> There is **no** `call`/`skip`/`verify`/`reset` action in the vocabulary. Any endpoint
> guarded on an invented verb would be denied to everyone (except super-admin). MPP domain
> actions must therefore be expressed **as one of the CRUD verbs above** (see next table).
> Adding new verbs would require extending `LevelActions` in shared `core` code — **YAGNI**;
> map onto CRUD instead.

### MPP domain actions → CRUD verb to guard on

| Domain action | Endpoint(s) | Guard permission | Level needed |
|---------------|-------------|------------------|--------------|
| List/read catalog, queue, reports | `GET …` | `mpp.<res>:read` | viewer+ |
| Create booking (web/walk-in) | `POST /booking`, `/walkin` | `mpp.booking:create` | editor+ (public/kiosk key) |
| Check-in | `POST /checkin` | `mpp.checkin:create` | editor (kiosk key) |
| Call / recall / start / done / skip / hold / transfer / second-service | `POST /queue/next`, `/antrian/{id}/*` | `mpp.antrian:update` | editor+ |
| FO verify | `POST /fo/{id}/verify` | `mpp.fo:update` | editor+ |
| Cancel booking | `POST /booking/{id}/cancel` | `mpp.booking:update` | editor+ |
| Manual reset | `POST /admin/reset` | `mpp.queue:update` | editor+ (admin/supervisor) |
| Broadcast to TV | `POST /admin/broadcast` | `mpp.display:update` | editor+ |
| Export report | `GET /reports/export` | `mpp.report:export` | **admin** |
| Manage master (instansi/layanan/loket/kuota/config) | `POST/PUT/DELETE …` | `mpp.<res>:create|update|delete` | editor/admin |

## Roles

| Role | Scope | Summary |
|------|-------|---------|
| `admin` | tenant-wide | full master data, config, users, all reports (incl. export), monitoring |
| `supervisor` | assigned agency(ies) | monitor + limited ops (open/close loket, reset, broadcast), agency reports/export |
| `front_office` | tenant / assigned desk | document verification |
| `petugas_loket` | assigned loket | queue operations at the loket |
| _device: kiosk_ | API-key | check-in + walk-in + print |
| _device: tv_ | API-key | display snapshot + subscribe |
| _citizen_ | none (public) | registration/check-in via public endpoints |

## Matrix (level per role)

Legend: **A** = admin, **E** = editor, **V** = viewer, — = none. Cell = the level to put in
the role's `{"resource":"level"}` map.

| Resource | admin | supervisor¹ | front_office | petugas_loket² | kiosk | tv |
|----------|:-----:|:-----------:|:------------:|:--------------:|:-----:|:--:|
| `mpp.instansi` | A | V | V | V | V | — |
| `mpp.layanan` | A | V | V | V | V | — |
| `mpp.loket` | A | E | — | V | — | — |
| `mpp.kuota` | A | V | — | — | — | — |
| `mpp.booking` | A | V | V | V | E | — |
| `mpp.checkin` | A | — | — | — | E | — |
| `mpp.queue` | A | E | V | E | — | V |
| `mpp.antrian` | A | V | V | E | — | V |
| `mpp.fo` | A | V | E | V | — | — |
| `mpp.display` | A | E | — | — | — | V |
| `mpp.monitoring` | A | V | — | — | — | — |
| `mpp.report` | A | A | — | — | — | — |
| `mpp.config` | A | — | — | — | — | — |
| `mpp.audit` | A | — | — | — | — | — |
| `core.user` | A | — | — | — | — | — |
| `core.role` | A | — | — | — | — | — |

Footnotes:
1. **Scoped to assigned agency** — a supervisor's levels apply only within their own
   instansi(s); the check passes but agency scoping still blocks other agencies' data.
2. **Scoped to assigned loket** — petugas acts only on their own loket's queue items.

## Enforcement

- Every mutating MPP endpoint declares the exact `resource:action` string (a CRUD verb from
  the vocabulary above); middleware checks it server-side (Redis-cached, DB fallback).
- **Agency/loket scoping** is enforced **in addition** to the permission check: holding
  `mpp.antrian:update` still does not let a supervisor/petugas act outside their scope.
- Devices use **pre-scoped API-keys** carrying a fixed permission set, never user JWTs.
- Super-admin bypasses all checks (`middleware.RequirePermission`); use sparingly.

## Seeding

Seed the four MPP roles under `seeders/mpp/` as `{"resource":"level"}` maps taken directly
from the matrix (e.g. `petugas_loket` → `{"mpp.antrian":"editor","mpp.queue":"editor",
"mpp.booking":"viewer","mpp.layanan":"viewer","mpp.loket":"viewer","mpp.fo":"viewer"}`),
following the `seeders/core` pattern (roles → users → assignments). Keep them `is_system`.
