-- =====================================================
-- MPP SEEDER - ROLES
-- =====================================================
-- Seeder 001: MPP RBAC roles (global system roles)
--
-- Permission levels: "viewer" (read), "editor" (act/CRUD), "admin" (full).
-- Resource strings map to docs/06-security/rbac-matrix.md.
-- Agency/loket scoping is enforced in addition to these levels by the backend.
-- =====================================================

INSERT INTO core.roles (id, code, name, description, permissions, is_system, is_active) VALUES
-- MPP Admin: full access to the MPP domain
(
    'a0000000-0000-0000-0000-000000000001',
    'mpp_admin',
    'MPP Administrator',
    'Full access to MPP master data, configuration, monitoring, and reports.',
    '{
        "mpp.instansi": "admin",
        "mpp.layanan": "admin",
        "mpp.loket": "admin",
        "mpp.kuota": "admin",
        "mpp.booking": "admin",
        "mpp.checkin": "admin",
        "mpp.queue": "admin",
        "mpp.antrian": "admin",
        "mpp.fo": "admin",
        "mpp.display": "admin",
        "mpp.monitoring": "admin",
        "mpp.report": "admin",
        "mpp.config": "admin",
        "mpp.audit": "viewer",
        "user-management": "admin",
        "roles": "admin"
    }'::jsonb,
    TRUE,
    TRUE
),
-- MPP Supervisor: monitor + limited ops, scoped to assigned agency
(
    'a0000000-0000-0000-0000-000000000002',
    'mpp_supervisor',
    'MPP Supervisor',
    'Monitor queues and perform limited operations (open/close loket, reset, broadcast) within the assigned agency.',
    '{
        "mpp.instansi": "viewer",
        "mpp.layanan": "viewer",
        "mpp.loket": "editor",
        "mpp.kuota": "viewer",
        "mpp.booking": "viewer",
        "mpp.queue": "editor",
        "mpp.antrian": "editor",
        "mpp.fo": "viewer",
        "mpp.display": "editor",
        "mpp.monitoring": "viewer",
        "mpp.report": "viewer"
    }'::jsonb,
    TRUE,
    TRUE
),
-- MPP Front Office: document verification only
(
    'a0000000-0000-0000-0000-000000000003',
    'mpp_front_office',
    'MPP Front Office',
    'Verify applicant document completeness before they proceed to a loket.',
    '{
        "mpp.instansi": "viewer",
        "mpp.layanan": "viewer",
        "mpp.booking": "viewer",
        "mpp.antrian": "viewer",
        "mpp.fo": "editor"
    }'::jsonb,
    TRUE,
    TRUE
),
-- MPP Counter Operator: queue operations at the assigned loket
(
    'a0000000-0000-0000-0000-000000000004',
    'mpp_petugas_loket',
    'MPP Petugas Loket',
    'Operate the assigned loket: call/recall/start/finish/skip/hold/transfer, and trigger second service.',
    '{
        "mpp.layanan": "viewer",
        "mpp.loket": "viewer",
        "mpp.queue": "editor",
        "mpp.antrian": "editor",
        "mpp.display": "viewer"
    }'::jsonb,
    TRUE,
    TRUE
)
ON CONFLICT DO NOTHING;

-- =====================================================
-- DEVICE ROLES (kiosk / TV)
-- =====================================================
-- An API key's effective permission set is the INTERSECTION of the key's
-- scoped_permissions and the OWNING USER's permissions (see
-- ApiKey.GetEffectivePermissions). A device key owned by a user with no
-- MPP permissions therefore resolves to an empty set — hence these two
-- narrow roles, assigned to the dedicated device users in 009_device_keys.
INSERT INTO core.roles (id, code, name, description, permissions, is_system, is_active) VALUES
(
    'a0000000-0000-0000-0000-000000000011',
    'mpp_device_kiosk',
    'MPP Kiosk Device',
    'Self-service kiosk: scan a QR to check in, register walk-ins, read the catalog.',
    '{
        "mpp.instansi": "viewer",
        "mpp.layanan": "viewer",
        "mpp.booking": "editor",
        "mpp.checkin": "editor"
    }'::jsonb,
    TRUE,
    TRUE
),
(
    'a0000000-0000-0000-0000-000000000012',
    'mpp_device_tv',
    'MPP TV Display Device',
    'Read-only display: current call + waiting stream for one agency.',
    '{
        "mpp.display": "viewer",
        "mpp.queue": "viewer"
    }'::jsonb,
    TRUE,
    TRUE
)
ON CONFLICT DO NOTHING;
