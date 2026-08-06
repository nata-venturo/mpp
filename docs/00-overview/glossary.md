# Glossary — Kamus Istilah _(ID/EN)_

Istilah domain yang dipakai konsisten di seluruh dokumen dan kode.

| Istilah (ID)          | Term (EN)                | Definisi                                                                                          |
|-----------------------|--------------------------|--------------------------------------------------------------------------------------------------|
| Instansi              | Agency                   | Lembaga penyelenggara layanan di MPP (Dukcapil, Imigrasi, dll.). Punya **prefix** 1–4 huruf (kolom `VARCHAR(4)`), umumnya satu huruf (A/B/C…). |
| Jenis layanan         | Service (type)           | Layanan spesifik dalam satu instansi; membawa **syarat dokumen** & **estimasi durasi**.          |
| Katalog dua tingkat   | Two-level catalog        | Instansi → Jenis layanan. Nomor antrian mengikuti prefix instansi.                               |
| Loket                 | Counter                  | Meja pelayanan fisik. Satu instansi bisa punya banyak loket.                                      |
| Antrian / Nomor       | Queue number / Ticket    | Satu permohonan layanan oleh satu warga; punya nomor berprefix (mis. `A-014`).                    |
| Booking               | Booking                  | Registrasi terjadwal (memilih tanggal), dibatasi **kuota per tanggal per instansi**.             |
| Walk-in               | Walk-in                  | Registrasi langsung di lokasi (kiosk) tanpa booking sebelumnya.                                   |
| Kuota                 | Quota                    | Batas jumlah booking per tanggal per instansi.                                                    |
| Check-in              | Check-in                 | Konfirmasi kehadiran di lokasi via **QR token sekali pakai** di kiosk → masuk antrean aktif.     |
| Token QR              | QR token                 | Kode sekali pakai (one-time) untuk check-in; kedaluwarsa setelah dipakai / lewat waktu.           |
| Kiosk                 | Kiosk                    | Perangkat mandiri: pemindai QR + printer termal untuk cetak tiket, juga registrasi walk-in.      |
| Display TV            | TV display               | Layar pemanggilan antrean; menampilkan nomor & loket, disertai suara.                            |
| TTS                   | Text-to-Speech           | Sintesis suara **Bahasa Indonesia offline** untuk memanggil nomor di TV.                          |
| Mini PC               | Mini PC                  | Satu komputer di lokasi yang menyalakan **3 TV** (satu browser, tiga window) + antrean audio.    |
| Front office (FO)     | Front office             | Petugas yang memverifikasi kelengkapan dokumen sebelum warga menuju loket.                        |
| Petugas loket         | Counter operator         | Petugas yang memanggil & melayani warga di loket.                                                 |
| Supervisor            | Supervisor               | Pemantau performa loket/antrean satu instansi.                                                    |
| Admin                 | Admin                    | Pengelola master data, konfigurasi, dan laporan lintas instansi.                                  |
| Second service        | Second service           | Layanan lanjutan untuk warga yang sama tanpa daftar ulang. Antrian asal → `QUEUED_NEXT` (terminal); antrian anak baru dibuat di `WAITING` layanan tujuan (`parent_antrian_id`).  |
| Panggil 3× lalu skip  | Call-3×-then-skip        | Nomor dipanggil maksimal 3 kali; jika tak hadir → di-skip.                                        |
| Alokasi idle-terlama  | Idle-longest allocation  | Nomor berikutnya diarahkan ke loket yang paling lama menganggur (round-robin adil).              |
| Mode FIFO             | FIFO mode                | Urutan murni berdasarkan waktu masuk antrean.                                                     |
| Mode Booking-prioritas| Booking-priority mode    | Booking didahulukan dari walk-in (dikonfigurasi admin per instansi).                             |
| Reset harian          | Daily reset              | Nomor antrean di-reset ke awal setiap hari pukul 00:00.                                           |
| Transfer              | Transfer                 | Memindahkan antrian ke loket/layanan lain.                                                        |
| Hold                  | Hold                     | Menahan sementara antrian yang sedang dilayani (mis. warga menyiapkan dokumen).                   |
| No-show               | No-show                  | Warga tidak hadir/di-skip setelah dipanggil.                                                      |
| RBAC                  | RBAC                     | Role-Based Access Control; hak akses per peran.                                                   |
| Company (tenant)      | Company (tenant)         | Konsep multi-tenant bawaan skeleton backend; dipetakan ke **instansi/MPP** (lihat domain docs).  |
