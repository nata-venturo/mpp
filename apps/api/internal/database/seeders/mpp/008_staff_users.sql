-- =====================================================
-- MPP SEEDER - STAFF USERS
-- =====================================================
-- Seeder 008: demo operator/FO/supervisor/admin accounts for the MPP
-- building. All four share the password `Petugas2026*`.
--
-- SignIn auto-scopes the JWT to the user's PRIMARY company membership
-- (auth.service SignIn → GetPrimaryCompany), and RequirePermission then
-- resolves permissions from core.company_users.role_id for that company.
-- So the membership row below is what actually grants the MPP role.
-- =====================================================

INSERT INTO core.users (id, email, username, password_hash, full_name, phone, is_active, is_email_verified, email_verified_at) VALUES
(
    'a9000000-0000-0000-0000-000000000001',
    'petugas@mpp.test',
    'mpp_petugas',
    '$2a$10$BGoEbS/QKg0Mrf1Vrfzcce5sFPdJ4rDeLmsugNczFrG/ercaAAhOy', -- Petugas2026*
    'Petugas Loket Dukcapil',
    '+6281299000001',
    TRUE,
    TRUE,
    NOW()
),
(
    'a9000000-0000-0000-0000-000000000002',
    'fo@mpp.test',
    'mpp_fo',
    '$2a$10$BGoEbS/QKg0Mrf1Vrfzcce5sFPdJ4rDeLmsugNczFrG/ercaAAhOy', -- Petugas2026*
    'Petugas Front Office',
    '+6281299000002',
    TRUE,
    TRUE,
    NOW()
),
(
    'a9000000-0000-0000-0000-000000000003',
    'supervisor@mpp.test',
    'mpp_supervisor_user',
    '$2a$10$BGoEbS/QKg0Mrf1Vrfzcce5sFPdJ4rDeLmsugNczFrG/ercaAAhOy', -- Petugas2026*
    'Supervisor MPP',
    '+6281299000003',
    TRUE,
    TRUE,
    NOW()
),
(
    'a9000000-0000-0000-0000-000000000004',
    'adminmpp@mpp.test',
    'mpp_admin_user',
    '$2a$10$BGoEbS/QKg0Mrf1Vrfzcce5sFPdJ4rDeLmsugNczFrG/ercaAAhOy', -- Petugas2026*
    'Administrator MPP',
    '+6281299000004',
    TRUE,
    TRUE,
    NOW()
)
ON CONFLICT DO NOTHING;

-- Membership in the MPP building, carrying the MPP role.
INSERT INTO core.company_users (id, company_id, user_id, role_id, is_primary, is_active, joined_at) VALUES
-- Petugas loket → mpp_petugas_loket
(
    'a9100000-0000-0000-0000-000000000001',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000004',
    TRUE, TRUE, NOW()
),
-- Front office → mpp_front_office
(
    'a9100000-0000-0000-0000-000000000002',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000003',
    TRUE, TRUE, NOW()
),
-- Supervisor → mpp_supervisor
(
    'a9100000-0000-0000-0000-000000000003',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000002',
    TRUE, TRUE, NOW()
),
-- Admin → mpp_admin
(
    'a9100000-0000-0000-0000-000000000004',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000004',
    'a0000000-0000-0000-0000-000000000001',
    TRUE, TRUE, NOW()
)
ON CONFLICT DO NOTHING;
