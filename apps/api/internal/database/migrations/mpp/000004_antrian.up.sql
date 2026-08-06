-- =====================================================
-- MPP SCHEMA - QUEUE (ANTRIAN)
-- =====================================================
-- Migration 004: antrian, serving_session, fo_verification
-- =====================================================

-- -----------------------------------------------------
-- ANTRIAN (Queue number / ticket)
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS mpp.antrian (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Origin
    booking_id UUID REFERENCES mpp.booking(id) ON DELETE SET NULL, -- NULL for walk-in
    pemohon_id UUID NOT NULL REFERENCES mpp.pemohon(id) ON DELETE CASCADE,
    instansi_id UUID NOT NULL REFERENCES mpp.instansi(id) ON DELETE RESTRICT,
    jenis_layanan_id UUID NOT NULL REFERENCES mpp.jenis_layanan(id) ON DELETE RESTRICT,

    -- Numbering
    nomor VARCHAR(20) NOT NULL,          -- formatted, e.g. 'A-014'
    nomor_seq INT NOT NULL,              -- sequence within service/day
    queue_date DATE NOT NULL DEFAULT CURRENT_DATE,

    source mpp.antrian_source NOT NULL DEFAULT 'WALK_IN',
    status mpp.antrian_status NOT NULL DEFAULT 'WAITING',

    -- Serving
    loket_id UUID REFERENCES mpp.loket(id) ON DELETE SET NULL,
    call_count INT NOT NULL DEFAULT 0 CHECK (call_count >= 0 AND call_count <= 3),
    priority INT NOT NULL DEFAULT 0,     -- derived from mode (booking-priority)
    fo_verified BOOLEAN,                 -- NULL = not required / not yet verified

    -- Second service linkage
    parent_antrian_id UUID REFERENCES mpp.antrian(id) ON DELETE SET NULL,

    -- Lifecycle timestamps
    queued_at TIMESTAMPTZ DEFAULT NOW(),
    called_at TIMESTAMPTZ,
    served_at TIMESTAMPTZ,
    done_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID
);

-- No duplicate number within an AGENCY on a given day. Queue numbers use the
-- agency prefix (e.g. A-014) and the sequence is per-instansi/day, so the pair
-- (instansi, day, seq) must be unique across all of that agency's services.
CREATE UNIQUE INDEX antrian_instansi_day_seq_key ON mpp.antrian(instansi_id, queue_date, nomor_seq);

-- Waiting stream is still read per service (one stream per jenis_layanan)
CREATE INDEX idx_antrian_service_status_seq ON mpp.antrian(jenis_layanan_id, status, nomor_seq);
CREATE INDEX idx_antrian_loket_status ON mpp.antrian(loket_id, status);
CREATE INDEX idx_antrian_instansi_date ON mpp.antrian(instansi_id, queue_date);
CREATE INDEX idx_antrian_status ON mpp.antrian(status);
CREATE INDEX idx_antrian_pemohon_id ON mpp.antrian(pemohon_id);
CREATE INDEX idx_antrian_parent_id ON mpp.antrian(parent_antrian_id) WHERE parent_antrian_id IS NOT NULL;

-- -----------------------------------------------------
-- SERVING_SESSION (One record per loket serving one antrian)
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS mpp.serving_session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    antrian_id UUID NOT NULL REFERENCES mpp.antrian(id) ON DELETE CASCADE,
    loket_id UUID NOT NULL REFERENCES mpp.loket(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES core.users(id) ON DELETE SET NULL,

    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    outcome mpp.serving_outcome,
    notes TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_serving_session_antrian_id ON mpp.serving_session(antrian_id);
CREATE INDEX idx_serving_session_loket_id ON mpp.serving_session(loket_id);
CREATE INDEX idx_serving_session_started_at ON mpp.serving_session(started_at);

-- -----------------------------------------------------
-- FO_VERIFICATION (Front-office document check)
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS mpp.fo_verification (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    antrian_id UUID NOT NULL REFERENCES mpp.antrian(id) ON DELETE CASCADE,
    user_id UUID REFERENCES core.users(id) ON DELETE SET NULL,

    result mpp.fo_result NOT NULL,
    checklist JSONB DEFAULT '{}',        -- {"<syarat_dokumen_id>": true/false}
    notes TEXT,
    verified_at TIMESTAMPTZ DEFAULT NOW(),

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_fo_verification_antrian_id ON mpp.fo_verification(antrian_id);

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE mpp.antrian IS 'Active queue item (from check-in or walk-in); see queue state machine';
COMMENT ON COLUMN mpp.antrian.call_count IS 'Call attempts (0..3); 3 then skip (BR-16)';
COMMENT ON COLUMN mpp.antrian.parent_antrian_id IS 'Links a second-service item to its origin';
COMMENT ON TABLE mpp.serving_session IS 'Record of a loket serving one antrian (durations, outcome)';
COMMENT ON TABLE mpp.fo_verification IS 'Front-office document verification outcome per antrian';
