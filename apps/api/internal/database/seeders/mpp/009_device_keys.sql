-- =====================================================
-- MPP SEEDER - DEVICE USERS & SCOPED API KEYS
-- =====================================================
-- Seeder 009: the kiosk and TV devices. Each device is a normal core
-- user (so the permission machinery works unchanged) carrying a narrow
-- device role, plus one API key whose scoped_permissions pin it further.
--
-- Demo keys (test environment — rotate before any real deployment):
--   kiosk  wiz_test_kiosk001_a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90
--   tv     wiz_test_tvdsp001_f0e1d2c3b4a596870f1e2d3c4b5a69780f1e2d3c4b5a69780f1e2d3c4b5a6978
--
-- Format is wiz_<env>_<key_id>_<secret> (middleware/auth.go splits on "_"
-- into exactly 4 parts); secret_hash below is SHA256(secret).
-- The keys carry NO password_hash — they are never meant to sign in.
-- =====================================================

INSERT INTO core.users (id, email, username, full_name, is_active, is_email_verified, email_verified_at) VALUES
(
    'a9000000-0000-0000-0000-000000000011',
    'kiosk@mpp.device',
    'mpp_device_kiosk',
    'Kiosk MPP (device)',
    TRUE, TRUE, NOW()
),
(
    'a9000000-0000-0000-0000-000000000012',
    'tv@mpp.device',
    'mpp_device_tv',
    'TV Display MPP (device)',
    TRUE, TRUE, NOW()
)
ON CONFLICT DO NOTHING;

INSERT INTO core.company_users (id, company_id, user_id, role_id, is_primary, is_active, joined_at) VALUES
(
    'a9100000-0000-0000-0000-000000000011',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000011',
    'a0000000-0000-0000-0000-000000000011',
    TRUE, TRUE, NOW()
),
(
    'a9100000-0000-0000-0000-000000000012',
    'a1000000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000012',
    'a0000000-0000-0000-0000-000000000012',
    TRUE, TRUE, NOW()
)
ON CONFLICT DO NOTHING;

INSERT INTO core.api_keys
    (id, user_id, company_id, key_id, secret_hash, key_prefix, name, description,
     environment, scoped_permissions, rate_limit, rate_limit_window)
VALUES
(
    'a9200000-0000-0000-0000-000000000001',
    'a9000000-0000-0000-0000-000000000011',
    'a1000000-0000-0000-0000-000000000001',
    'kiosk001',
    '79175e70eb2236876b0c003be58294690c0e36b44c0947ae80f599ea9d039833',
    'wiz_test_kiosk001_a1b2...',
    'MPP Kiosk (demo)',
    'Self-service kiosk: QR check-in and walk-in registration.',
    'test',
    '["mpp.checkin:create","mpp.booking:create","mpp.layanan:read","mpp.instansi:read"]'::jsonb,
    10000,
    3600
),
(
    'a9200000-0000-0000-0000-000000000002',
    'a9000000-0000-0000-0000-000000000012',
    'a1000000-0000-0000-0000-000000000001',
    'tvdsp001',
    '487861f4875899e40a108824c2e736a6a9e14a47ebd95cca609a2fa99f8b5f21',
    'wiz_test_tvdsp001_f0e1...',
    'MPP TV Display (demo)',
    'Read-only TV display: current call + waiting stream.',
    'test',
    '["mpp.display:read","mpp.queue:read"]'::jsonb,
    10000,
    3600
)
ON CONFLICT DO NOTHING;
