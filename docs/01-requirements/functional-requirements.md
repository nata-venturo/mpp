# Functional Requirements (FR) — per Modul _(ID)_

Notasi ID: `FR-<modul>-<nomor>`. Prioritas: **M** (must), **S** (should), **C** (could).

---

## Modul 1 — Master Instansi

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-INST-01 | M | Admin dapat membuat/ubah/nonaktifkan instansi (nama, deskripsi, logo, jam operasional). |
| FR-INST-02 | M | Setiap instansi punya **prefix** unik untuk penomoran, 1–4 huruf (umumnya satu, mis. A, B, C). |
| FR-INST-03 | M | Instansi punya konfigurasi mode antrean default (FIFO / Booking-prioritas). |
| FR-INST-04 | S | Instansi dapat diatur jam buka/tutup dan hari libur (memengaruhi ketersediaan booking). |
| FR-INST-05 | S | Nonaktifkan instansi menyembunyikannya dari kanal registrasi tanpa menghapus histori. |

## Modul 2 — Master Jenis Layanan

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-SVC-01 | M | Admin mengelola jenis layanan di bawah satu instansi (nama, deskripsi, aktif/nonaktif). |
| FR-SVC-02 | M | Jenis layanan menyimpan **daftar syarat dokumen** (nama, wajib/opsional, keterangan). |
| FR-SVC-03 | M | Jenis layanan menyimpan **estimasi durasi** layanan (menit) untuk perhitungan ETA. |
| FR-SVC-04 | S | Jenis layanan dapat menandai apakah butuh **verifikasi front office** sebelum ke loket. |
| FR-SVC-05 | C | Jenis layanan dapat memicu **second service** default (rekomendasi layanan lanjutan). |

## Modul 3 — Master Loket

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-LKT-01 | M | Admin mengelola loket per instansi (nama/nomor loket, aktif/nonaktif). |
| FR-LKT-02 | M | Satu loket dapat melayani satu atau beberapa jenis layanan (mapping loket↔layanan). |
| FR-LKT-03 | M | Loket dipetakan ke petugas yang sedang login (sesi loket). |
| FR-LKT-04 | S | Loket bisa diberi status buka/tutup/istirahat yang memengaruhi alokasi antrean. |

## Modul 4 — Manajemen Kuota Booking

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-QTA-01 | M | Admin menetapkan **kuota booking per tanggal per instansi** (atau per jenis layanan). |
| FR-QTA-02 | M | Sistem menolak booking bila kuota tanggal tersebut penuh. |
| FR-QTA-03 | S | Admin dapat mengatur kuota berulang (mis. default harian) dan pengecualian tanggal. |
| FR-QTA-04 | S | Sisa kuota tampil real-time di kanal registrasi. |

## Modul 5 — Registrasi via WhatsApp AI Agent

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-WA-01 | M | Warga memulai percakapan via WhatsApp Business API; **AI agent (LLM)** memandu registrasi. |
| FR-WA-02 | M | AI agent melakukan slot-filling: pilih instansi → jenis layanan → tanggal (booking). |
| FR-WA-03 | M | AI agent menampilkan **syarat dokumen** jenis layanan terpilih. |
| FR-WA-04 | M | Setelah data lengkap & kuota tersedia, sistem membuat booking dan mengirim **QR check-in** + ringkasan. |
| FR-WA-05 | S | AI agent dapat menjawab pertanyaan umum (lokasi, jam, syarat) di luar alur booking. |
| FR-WA-06 | S | Warga dapat membatalkan/ubah booking lewat WhatsApp. |
| FR-WA-07 | C | Fallback ke menu terstruktur (button/list) bila LLM tidak yakin. |

## Modul 6 — Registrasi via Website Publik

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-WEB-01 | M | Warga dapat memilih instansi & jenis layanan, melihat syarat dokumen, dan memilih tanggal booking di web. |
| FR-WEB-02 | M | Web menampilkan **sisa kuota** per tanggal dan menolak bila penuh. |
| FR-WEB-03 | M | Setelah booking, web menampilkan **QR check-in** + ringkasan (dapat diunduh/kirim email). |
| FR-WEB-04 | S | Warga dapat melihat status booking dan membatalkan. |
| FR-WEB-05 | S | Halaman publik informasi antrean berjalan (nomor sedang dilayani per instansi). |

## Modul 7 — Registrasi Walk-in (Kiosk)

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-WLK-01 | M | Warga tanpa booking dapat mendaftar langsung di **kiosk**: pilih instansi & jenis layanan. |
| FR-WLK-02 | M | Sistem membuat nomor antrean walk-in dan **mencetak tiket termal**. |
| FR-WLK-03 | M | Walk-in langsung masuk antrean aktif (status `WAITING`) sesuai mode instansi. |
| FR-WLK-04 | S | Kiosk menampilkan syarat dokumen sebelum konfirmasi. |

## Modul 8 — Check-in QR

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-CHK-01 | M | Warga booking melakukan check-in dengan memindai **QR token sekali pakai** di kiosk. |
| FR-CHK-02 | M | Token yang sudah dipakai / kedaluwarsa / di luar hari-H ditolak dengan pesan jelas. |
| FR-CHK-03 | M | Check-in sukses mengubah status `BOOKED → CHECKED_IN` lalu masuk antrean (`WAITING`). |
| FR-CHK-04 | M | Setelah check-in, kiosk **mencetak tiket termal** berisi nomor antrean & estimasi. |
| FR-CHK-05 | S | Booking yang tidak check-in sampai batas waktu menjadi `EXPIRED` (no-show). |

## Modul 9 — Engine Antrian

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-QUE-01 | M | **Satu aliran antrean per jenis layanan** (single stream) untuk pemanggilan; **penomoran berprefix instansi & berurutan per instansi/hari** (semua layanan satu instansi berbagi satu deret nomor). |
| FR-QUE-02 | M | Nomor berikutnya dialokasikan ke **loket yang paling lama idle** (round-robin adil). |
| FR-QUE-03 | M | Mendukung mode **FIFO** dan **Booking-prioritas** per instansi (dikonfigurasi admin). |
| FR-QUE-04 | M | **Reset harian pukul 00:00** — penomoran kembali ke awal. |
| FR-QUE-05 | M | Perhitungan ETA berdasarkan estimasi durasi layanan & posisi antrean (rumus di `../02-domain/business-rules.md` BR-29). |
| FR-QUE-06 | M | State antrean & counter nomor dikelola di Redis; perubahan disiarkan via WebSocket. |
| FR-QUE-07 | S | Prefix & format nomor dapat dikonfigurasi (mis. `A-014`). |

## Modul 10 — Display TV + Pemanggilan Suara (TTS)

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-TV-01 | M | TV menampilkan nomor **sedang dipanggil**, loket tujuan, dan daftar nomor berikutnya. |
| FR-TV-02 | M | Saat dipanggil, sistem membunyikan **TTS Bahasa Indonesia offline** (mis. "Nomor A-014, silakan ke loket 3"). |
| FR-TV-03 | M | Satu **mini PC** menyalakan **3 TV** (satu browser, tiga window) dengan **satu antrean audio bersama** agar suara tidak tumpang tindih. |
| FR-TV-04 | M | Pemanggilan suara tetap berfungsi meski koneksi internet terputus (audio & suara lokal). |
| FR-TV-05 | S | TV dapat menampilkan konten informasi/berjalan (running text) di sela pemanggilan. |
| FR-TV-06 | S | Pengulangan panggilan (call 2×/3×) juga membunyikan ulang TTS. |

## Modul 11 — Aplikasi Loket (Petugas)

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-OPR-01 | M | Petugas login dan memilih/terhubung ke loketnya (sesi loket). |
| FR-OPR-02 | M | Aksi: **Panggil berikutnya**, **Panggil ulang**, **Lewati (skip)**, **Mulai layani**, **Selesai**. |
| FR-OPR-03 | M | Aturan **panggil maksimal 3× lalu skip** bila warga tidak hadir. |
| FR-OPR-04 | M | Aksi **Transfer** ke loket/layanan lain dan **Hold** (tahan sementara). |
| FR-OPR-05 | M | Petugas dapat memicu **Second Service** untuk warga yang sama (tanpa daftar ulang). |
| FR-OPR-06 | S | Tampilan detail pemohon & checklist dokumen (hasil verifikasi FO). |
| FR-OPR-07 | S | Statistik ringkas petugas (jumlah dilayani, rata-rata durasi hari ini). |

## Modul 12 — Second Service (Layanan Kedua Otomatis)

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-SEC-01 | M | Petugas/sistem dapat menandai warga butuh layanan lanjutan saat `SERVING`. Antrian asal ditutup ke status `QUEUED_NEXT` (terminal; `serving_session` tetap `DONE`) dan sistem membuat **antrian anak** (`SECOND_SERVICE`, `parent_antrian_id`) langsung di `WAITING` layanan tujuan. |
| FR-SEC-02 | M | Antrian anak masuk antrean layanan kedua **tanpa registrasi ulang**, membawa `pemohon` yang sama. |
| FR-SEC-03 | S | Estimasi & prioritas layanan kedua mengikuti aturan instansi tujuan. |

## Modul 13 — Verifikasi Dokumen Front Office

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-FO-01 | M | FO melihat daftar pemohon yang menunggu verifikasi beserta **checklist syarat dokumen**. |
| FR-FO-02 | M | FO menandai dokumen **lengkap/tidak lengkap**; bila tidak lengkap, beri catatan & tahan dari antrean loket. |
| FR-FO-03 | S | FO dapat mengunggah/scan dokumen pendukung (opsional, sesuai kebijakan PII). |
| FR-FO-04 | S | Hasil verifikasi tampil di aplikasi loket. |

## Modul 14 — Manajemen Pengguna & RBAC

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-USR-01 | M | Admin mengelola akun pengguna & penetapan peran (admin, supervisor, front office, petugas loket). |
| FR-USR-02 | M | Hak akses per peran ditegakkan di backend (lihat `../06-security/rbac-matrix.md`). |
| FR-USR-03 | M | Autentikasi JWT; dukungan API-key untuk perangkat (kiosk/TV) bila diperlukan. |
| FR-USR-04 | S | Supervisor & petugas dibatasi pada instansi/loket yang menjadi tanggung jawabnya. |

## Modul 15 — Dashboard Admin & Monitoring Real-time

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-ADM-01 | M | Dashboard menampilkan status antrean seluruh instansi/loket secara real-time. |
| FR-ADM-02 | M | Admin mengelola master (instansi, layanan, loket, kuota) dan konfigurasi mode antrean. |
| FR-ADM-03 | S | Monitoring beban loket, nomor menunggu, dan waktu tunggu berjalan. |
| FR-ADM-04 | S | Kontrol operasional: buka/tutup loket, reset manual, broadcast informasi ke TV. |

## Modul 16 — Pelaporan & Analitik

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-RPT-01 | M | Laporan jumlah dilayani, no-show, rata-rata & p90 waktu tunggu/lama layanan, per instansi/loket/jenis/hari. |
| FR-RPT-02 | S | Filter rentang tanggal, instansi, jenis layanan; ekspor CSV/Excel. |
| FR-RPT-03 | C | Tren harian/mingguan & jam sibuk (heatmap). |

## Modul 17 — Notifikasi

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-NTF-01 | M | Kirim konfirmasi booking + QR via WhatsApp (dan/atau email SMTP). |
| FR-NTF-02 | S | Pengingat H-1 / hari-H untuk mengurangi no-show. |
| FR-NTF-03 | S | Notifikasi "sebentar lagi dipanggil" (mis. N nomor sebelum giliran). |
| FR-NTF-04 | C | Notifikasi SMS sebagai kanal cadangan. |

## Modul 18 — Konfigurasi Sistem & Audit Log

| ID | Prioritas | Kebutuhan |
|----|-----------|-----------|
| FR-CFG-01 | M | Konfigurasi global & per-instansi (mode antrean, jam operasional, format nomor, batas check-in). |
| FR-CFG-02 | M | **Audit log** untuk aksi penting (ubah master, konfigurasi, aksi loket sensitif). |
| FR-CFG-03 | S | Pengaturan template pesan (WA/email) dan teks TTS. |
