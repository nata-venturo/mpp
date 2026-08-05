# Entity-Relationship Diagram — MPP _(EN)_

Schema `mpp` (queue domain) referencing `core` (users/companies from the skeleton).
See [`domain-model.md`](./domain-model.md) for field-level detail.

```mermaid
erDiagram
    COMPANY ||--o{ INSTANSI : "has (tenant = MPP building)"
    INSTANSI ||--o{ JENIS_LAYANAN : offers
    INSTANSI ||--o{ LOKET : has
    INSTANSI ||--o{ KUOTA_BOOKING : sets
    JENIS_LAYANAN ||--o{ SYARAT_DOKUMEN : requires
    JENIS_LAYANAN ||--o{ LOKET_LAYANAN : "served by"
    LOKET ||--o{ LOKET_LAYANAN : serves
    LOKET ||--o{ LOKET_SESSION : "operated in"
    CORE_USER ||--o{ LOKET_SESSION : opens

    PEMOHON ||--o{ BOOKING : makes
    INSTANSI ||--o{ BOOKING : "for"
    JENIS_LAYANAN ||--o{ BOOKING : "for"
    BOOKING ||--o| ANTRIAN : "becomes (on check-in)"
    PEMOHON ||--o{ ANTRIAN : "in line"
    JENIS_LAYANAN ||--o{ ANTRIAN : "of service"
    LOKET ||--o{ ANTRIAN : "called to"
    ANTRIAN ||--o{ SERVING_SESSION : "served in"
    ANTRIAN ||--o| FO_VERIFICATION : "verified by FO"
    ANTRIAN ||--o{ ANTRIAN : "second service (parent)"
    CORE_USER ||--o{ SERVING_SESSION : performs
    CORE_USER ||--o{ FO_VERIFICATION : performs

    COMPANY {
      uuid id PK
      string name
    }
    CORE_USER {
      uuid id PK
      string email
      uuid company_id FK
    }
    INSTANSI {
      uuid id PK
      uuid company_id FK
      string name
      string slug
      string prefix "unique per tenant, e.g. A"
      string queue_mode "FIFO | BOOKING_PRIORITY"
      jsonb operating_hours
      bool is_active
    }
    JENIS_LAYANAN {
      uuid id PK
      uuid instansi_id FK
      string name
      int estimasi_durasi_menit
      bool requires_fo_verification
      uuid default_second_service_id FK "nullable"
      bool is_active
    }
    SYARAT_DOKUMEN {
      uuid id PK
      uuid jenis_layanan_id FK
      string name
      bool is_required
      string notes
      int sort
    }
    LOKET {
      uuid id PK
      uuid instansi_id FK
      string code
      string name "nullable, e.g. Loket 1"
      string status "OPEN | CLOSED | BREAK"
      timestamp last_idle_at
      bool is_active
    }
    LOKET_LAYANAN {
      uuid loket_id FK
      uuid jenis_layanan_id FK
    }
    LOKET_SESSION {
      uuid id PK
      uuid loket_id FK
      uuid user_id FK
      timestamp opened_at
      timestamp closed_at
      bool is_active
    }
    KUOTA_BOOKING {
      uuid id PK
      uuid instansi_id FK
      uuid jenis_layanan_id FK "nullable"
      date tanggal
      int kuota
      int terpakai
    }
    PEMOHON {
      uuid id PK
      string name
      string phone
      string email "nullable"
      string nik_hash "hashed, only if service requires"
    }
    BOOKING {
      uuid id PK
      uuid pemohon_id FK
      uuid instansi_id FK
      uuid jenis_layanan_id FK
      date tanggal
      string channel "WHATSAPP | WEB"
      string qr_token "single-use"
      timestamp qr_expires_at
      string status "BOOKED | CHECKED_IN | EXPIRED | CANCELLED"
      timestamp checked_in_at "nullable"
    }
    ANTRIAN {
      uuid id PK
      uuid booking_id FK "nullable (walk-in)"
      uuid pemohon_id FK
      uuid instansi_id FK
      uuid jenis_layanan_id FK
      string nomor "e.g. A-014"
      int nomor_seq "per instansi/day"
      date queue_date
      string source "BOOKING | WALK_IN | SECOND_SERVICE"
      string status "see state machine"
      uuid loket_id FK "nullable"
      int call_count "0..3"
      int priority "derived from queue_mode"
      bool fo_verified "nullable"
      uuid parent_antrian_id FK "nullable"
      timestamp queued_at
      timestamp called_at
      timestamp served_at
      timestamp done_at
    }
    SERVING_SESSION {
      uuid id PK
      uuid antrian_id FK
      uuid loket_id FK
      uuid user_id FK
      timestamp started_at
      timestamp ended_at
      string outcome "DONE | SKIPPED | TRANSFERRED | HOLD"
    }
    FO_VERIFICATION {
      uuid id PK
      uuid antrian_id FK
      uuid user_id FK
      string result "COMPLETE | INCOMPLETE"
      jsonb checklist
      timestamp verified_at
    }
```

## Indexing notes

- `antrian`: composite index on (`jenis_layanan_id`, `status`, `nomor_seq`) for the
  waiting stream (one stream per service); index on (`loket_id`, `status`); **unique on
  (`instansi_id`, `queue_date`, `nomor_seq`)** — the number series is per-agency (shared
  prefix), so uniqueness is per instansi/day, not per service.
- `kuota_booking`: unique (`instansi_id`, `jenis_layanan_id`, `tanggal`).
- `booking`: unique `qr_token`; index (`status`, `tanggal`).
- `instansi`: unique (`company_id`, `prefix`) and (`company_id`, `slug`).
