# Aturan Bisnis — MPP _(ID)_

Notasi: `BR-<n>`. Aturan ini mengikat perilaku engine antrian & registrasi.

## Penomoran

- **BR-01 — Prefix instansi.** Nomor antrean memakai prefix huruf instansi + urutan,
  contoh `A-014` (Dukcapil = A). Prefix unik per tenant.
- **BR-02 — Satu aliran antrean per jenis layanan; penomoran per instansi.** Setiap jenis
  layanan punya satu aliran antrean (single stream) untuk dipanggil terpisah. Namun
  **nomor berurutan per instansi per hari** (bukan per layanan): karena nomor memakai
  prefix instansi (`A-014`), seluruh layanan di bawah satu instansi berbagi satu deret
  nomor (`A-001`, `A-002`, …). Unik pada (`instansi_id`, `queue_date`, `nomor_seq`).
- **BR-03 — Reset harian 00:00.** Counter nomor kembali ke awal setiap tengah malam.
  Antrian sisa (`WAITING`/`CALLED`/`HOLD`) dari hari sebelumnya **default di-`CANCELLED`**
  (tidak ada status `EXPIRED` pada antrian — `EXPIRED` hanya milik `booking`). Perilaku
  dapat diubah admin lewat `system_config` key `daily_reset`.
- **BR-04 — Format nomor dikonfigurasi.** Pola nomor (mis. `A-014` vs `A014`) diatur
  di konfigurasi sistem.

## Booking & kuota

- **BR-05 — Kuota per tanggal per instansi.** Booking dibatasi kuota tanggal tersebut
  (opsional lebih rinci per jenis layanan). Penuh → booking ditolak.
- **BR-06 — Kuota atomik.** Penambahan pemakaian kuota harus atomik agar tidak terjadi
  overbooking saat bersamaan.
- **BR-07 — Pembatalan mengembalikan kuota.** Pembatalan sebelum batas waktu menambah
  sisa kuota; setelah batas, tidak.
- **BR-08 — Booking di luar jam/hari libur ditolak.** Mengikuti `operating_hours`
  instansi dan pengecualian tanggal.

## Check-in

- **BR-09 — QR sekali pakai.** Token QR hanya valid satu kali dan terikat waktu
  (hari-H + jendela waktu). Token dipakai ulang/kedaluwarsa → ditolak.
- **BR-10 — Check-in membentuk antrean.** Check-in sukses mengubah `BOOKED → CHECKED_IN`
  lalu memberi nomor & masuk `WAITING`; tiket termal dicetak.
- **BR-11 — No-show sebelum antre.** Booking yang tidak check-in sampai batas waktu
  menjadi `EXPIRED` dan tercatat sebagai no-show.

## Alokasi loket & mode antrean

- **BR-12 — Idle-terlama (round-robin adil).** Nomor berikutnya dialokasikan ke loket
  **eligible** (status OPEN, memetakan layanan tersebut) yang **paling lama menganggur**
  (`last_idle_at` terlama).
- **BR-13 — Mode FIFO.** Urutan murni waktu masuk antrean (`queued_at`).
- **BR-14 — Mode Booking-prioritas.** Pemohon dengan booking didahulukan dari walk-in.
  Mode dipilih admin **per instansi**.
- **BR-15 — Loket tutup/istirahat.** Loket `CLOSED`/`BREAK` tidak menerima alokasi baru.

## Pemanggilan

- **BR-16 — Panggil maksimal 3× lalu skip.** Nomor dipanggil hingga 3 kali; bila warga
  tetap tidak hadir → `SKIPPED` (no-show). Setiap panggilan membunyikan ulang TTS.
- **BR-17 — Grace requeue (opsional).** Alih-alih langsung skip, admin dapat
  mengaktifkan pengembalian ke akhir antrean satu kali.
- **BR-18 — Satu antrean audio TV.** Semua pemanggilan pada satu mini PC (3 TV) melewati
  **satu antrean audio bersama**; suara tidak boleh tumpang tindih.

## Pelayanan

- **BR-19 — Mulai/selesai mencatat durasi.** `SERVING → DONE` menutup `serving_session`
  dan mencatat lama layanan untuk pelaporan.
- **BR-20 — Hold tidak melepas nomor.** `HOLD` menahan sementara tanpa menghapus dari
  pelayanan; dapat dilanjutkan (`SERVING`).
- **BR-21 — Transfer.** Memindahkan pemohon ke loket/jenis layanan lain; membentuk item
  antrean di tujuan (`WAITING`), sesi asal ditutup sebagai `TRANSFERRED`.

## Second service (layanan kedua)

- **BR-22 — Tanpa daftar ulang.** Warga yang butuh layanan lanjutan tidak mendaftar
  ulang; sistem membuat item baru (`SECOND_SERVICE`) tertaut ke item asal
  (`parent_antrian_id`) dan memasukkannya ke antrean layanan tujuan (`QUEUED_NEXT → WAITING`).
- **BR-23 — Prioritas layanan kedua.** Mengikuti mode instansi tujuan; dapat dikonfigurasi
  agar layanan kedua diprioritaskan.

## Verifikasi dokumen (front office)

- **BR-24 — Verifikasi sebelum loket (bila diwajibkan).** Jika `requires_fo_verification`
  aktif, pemohon harus **lengkap** menurut FO sebelum dapat dipanggil ke loket.
- **BR-25 — Dokumen kurang menahan antrean.** Hasil `INCOMPLETE` menahan pemohon dari
  antrean loket sampai dilengkapi; diberi catatan.

## Estimasi waktu tunggu (ETA)

- **BR-29 — Rumus ETA.** Untuk satu antrian pada layanan `L`:

  ```
  posisi   = jumlah antrian di depannya (status WAITING atau CALLED) pada layanan L
  n_loket  = jumlah loket eligible untuk L yang berstatus OPEN (min. 1)
  ETA_menit = ceil(posisi / n_loket) × estimasi_durasi_menit(L)
  ```

  Bila `n_loket = 0` (belum ada loket buka), ETA tak ditampilkan (atau "menunggu loket
  dibuka"). ETA bersifat indikatif dan dihitung ulang saat antrean berubah (event
  `queue.updated`). Mode Booking-prioritas memengaruhi `posisi` (booking dihitung di depan
  walk-in).

## Operasional & audit

- **BR-26 — Kontrol supervisor terbatas instansi.** Supervisor hanya mengelola instansi
  yang ditugaskan (buka/tutup loket, reset, broadcast).
- **BR-27 — Audit aksi sensitif.** Perubahan master, konfigurasi, reset manual, dan
  transfer tercatat di audit log.
- **BR-28 — Zona waktu.** Penyimpanan waktu dalam UTC; tampilan dikonversi ke zona
  waktu lokal (WIB/WITA/WIT) sesuai konfigurasi.
