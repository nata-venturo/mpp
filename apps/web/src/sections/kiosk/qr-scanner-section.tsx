'use client';

import jsQR from 'jsqr';
import { useRef, useState, useEffect, useCallback } from 'react';

import Box from '@mui/material/Box';
import Alert from '@mui/material/Alert';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

// ----------------------------------------------------------------------
// Pemindai QR kiosk.
//
// Dua jalur masuk sekaligus, karena kiosk di lapangan bisa punya salah
// satu atau keduanya:
//
//   1. KAMERA — video + jsQR. Dekode dilakukan pustaka, bukan tangan.
//   2. PEMINDAI USB — perangkat HID berperilaku seperti keyboard: ia
//      "mengetik" token lalu menekan Enter. Tidak butuh kamera sama
//      sekali, jadi input penampungnya selalu aktif di belakang layar.
//
// Kamera hanya boleh diakses di secure context: https, atau localhost.

/** Jeda antar percobaan dekode. 10/detik sudah jauh di atas kebutuhan. */
const SCAN_INTERVAL_MS = 100;

type CameraState = 'idle' | 'starting' | 'streaming' | 'denied' | 'unavailable';

type Props = {
  /** Dipanggil sekali per token yang terbaca (kamera maupun pemindai). */
  onScan: (token: string) => void;
  /** Menahan pemindaian saat request sedang berjalan / hasil ditampilkan. */
  paused?: boolean;
};

function cameraMessage(state: CameraState, detail: string): string {
  switch (state) {
    case 'denied':
      // Instruksi harus menyebut TEMPATNYA. "Izinkan kamera di browser"
      // tidak menolong siapa pun yang dialog izinnya memang tidak muncul.
      return (
        'Kamera diblokir untuk situs ini. Klik ikon gembok di address bar → ' +
        'Izin situs → Kamera → Izinkan, lalu muat ulang. Di Windows, pastikan juga ' +
        'Settings → Privacy & security → Camera menyala untuk browser Anda.'
      );
    case 'unavailable':
      return detail || 'Kamera tidak tersedia di perangkat ini. Gunakan pemindai QR.';
    default:
      return '';
  }
}

export function QrScannerSection({ onScan, paused = false }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  /** Permintaan izin sedang terbang — mencegah permintaan kedua. */
  const startingRef = useRef(false);
  /** Effect sudah dibersihkan sementara izin masih ditunggu. */
  const unmountedRef = useRef(false);

  const [camera, setCamera] = useState<CameraState>('idle');
  const [detail, setDetail] = useState('');

  // `paused` dibaca lewat ref di dalam loop supaya loop tidak perlu
  // dibangun ulang setiap kali status berubah.
  const pausedRef = useRef(paused);
  pausedRef.current = paused;

  const onScanRef = useRef(onScan);
  onScanRef.current = onScan;

  const focusInput = useCallback(() => {
    inputRef.current?.focus();
  }, []);

  // ── Pemindai USB (HID) ──────────────────────────────────────────────
  useEffect(() => {
    focusInput();

    // Layar sentuh memindahkan fokus ke mana-mana; rebut kembali berkala
    // supaya pemindai selalu punya tempat mengetik.
    const interval = setInterval(focusInput, 1_000);
    return () => clearInterval(interval);
  }, [focusInput]);

  // ── Kamera ──────────────────────────────────────────────────────────
  const startCamera = useCallback(async () => {
    if (streamRef.current || startingRef.current) return;

    if (typeof navigator === 'undefined' || !navigator.mediaDevices?.getUserMedia) {
      setCamera('unavailable');
      setDetail(
        window.isSecureContext
          ? 'Browser ini tidak mendukung akses kamera.'
          : 'Kamera hanya bisa diakses lewat https atau localhost — bukan lewat alamat IP biasa.'
      );
      return;
    }

    // Penjaga HARUS disetel sebelum await. React StrictMode menjalankan
    // effect mount dua kali, jadi dua getUserMedia bisa terbang bersamaan;
    // browser menolak yang kedua dengan NotAllowedError dan dialog izin
    // tidak pernah sempat muncul.
    startingRef.current = true;
    unmountedRef.current = false;
    setCamera('starting');

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        // Kamera belakang untuk tablet kiosk; laptop mengabaikannya.
        video: { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 720 } },
        audio: false,
      });

      // Effect sempat dibersihkan saat izin masih ditunggu: matikan
      // langsung, kalau tidak lampu kamera menyala tanpa pemilik.
      if (unmountedRef.current) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }

      streamRef.current = stream;

      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        await videoRef.current.play();
      }

      setCamera('streaming');
    } catch (error) {
      const name = error instanceof DOMException ? error.name : '';

      // Tanya Permissions API sebelum menyimpulkan "ditolak": pesan yang
      // salah membuat orang mencari-cari pengaturan yang tidak bermasalah.
      let blocked = name === 'NotAllowedError';
      if (blocked) {
        try {
          const status = await navigator.permissions.query({
            name: 'camera' as PermissionName,
          });
          blocked = status.state === 'denied';
        } catch {
          // Permissions API tidak mendukung 'camera' di semua browser —
          // pertahankan kesimpulan dari nama error.
        }
      }

      setCamera(blocked ? 'denied' : 'unavailable');
      setDetail(
        name === 'NotFoundError'
          ? 'Tidak ada kamera yang terdeteksi di perangkat ini.'
          : name === 'NotReadableError'
            ? 'Kamera sedang dipakai aplikasi lain. Tutup aplikasi itu lalu coba lagi.'
            : blocked
              ? ''
              : 'Kamera tidak dapat dinyalakan. Coba lagi, atau gunakan pemindai QR.'
      );
    } finally {
      startingRef.current = false;
    }
  }, []);

  const stopCamera = useCallback(() => {
    unmountedRef.current = true;
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;

    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
  }, []);

  useEffect(() => {
    void startCamera();

    // Melepas track WAJIB: tanpa ini lampu kamera tetap menyala dan
    // perangkat terkunci untuk aplikasi lain setelah layar ditinggalkan.
    return stopCamera;
  }, [startCamera, stopCamera]);

  // ── Loop dekode ─────────────────────────────────────────────────────
  useEffect(() => {
    if (camera !== 'streaming') return undefined;

    const timer = setInterval(() => {
      if (pausedRef.current) return;

      const video = videoRef.current;
      const canvas = canvasRef.current;
      if (!video || !canvas || video.readyState !== video.HAVE_ENOUGH_DATA) return;

      const { videoWidth: width, videoHeight: height } = video;
      if (!width || !height) return;

      canvas.width = width;
      canvas.height = height;

      const context = canvas.getContext('2d', { willReadFrequently: true });
      if (!context) return;

      context.drawImage(video, 0, 0, width, height);
      const image = context.getImageData(0, 0, width, height);

      const found = jsQR(image.data, width, height, { inversionAttempts: 'dontInvert' });
      if (found?.data) {
        onScanRef.current(found.data.trim());
      }
    }, SCAN_INTERVAL_MS);

    return () => clearInterval(timer);
  }, [camera]);

  const warning = cameraMessage(camera, detail);

  return (
    <Stack spacing={2} sx={{ width: 1, alignItems: 'center', minHeight: 0 }}>
      <Box
        sx={{
          position: 'relative',
          width: 1,
          maxWidth: 420,
          flex: '1 1 auto',
          minHeight: 0,
          aspectRatio: '1 / 1',
          borderRadius: 3,
          overflow: 'hidden',
          bgcolor: 'common.black',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Box
          component="video"
          ref={videoRef}
          muted
          playsInline
          sx={{
            width: 1,
            height: 1,
            objectFit: 'cover',
            display: camera === 'streaming' ? 'block' : 'none',
          }}
        />

        {camera !== 'streaming' && (
          <Stack spacing={2} sx={{ px: 3, alignItems: 'center' }}>
            <Typography sx={{ color: 'common.white', textAlign: 'center' }}>
              {camera === 'starting' ? 'Menyalakan kamera…' : 'Kamera tidak aktif'}
            </Typography>

            {/* Tombol eksplisit: permintaan izin yang dipicu ketukan
                pengguna jauh lebih mungkin memunculkan dialog daripada
                yang otomatis saat halaman dibuka. */}
            {camera !== 'starting' && (
              <Button variant="contained" onClick={() => void startCamera()}>
                Nyalakan kamera
              </Button>
            )}
          </Stack>
        )}

        {/* Bingkai bidik: memberi tahu orang ke mana QR harus diarahkan. */}
        {camera === 'streaming' && (
          <Box
            sx={{
              position: 'absolute',
              inset: '15%',
              borderRadius: 2,
              border: '3px solid',
              borderColor: 'primary.main',
              boxShadow: '0 0 0 100vmax rgba(0,0,0,.35)',
              pointerEvents: 'none',
            }}
          />
        )}
      </Box>

      {/* Kanvas kerja jsQR — tidak pernah ditampilkan. */}
      <Box component="canvas" ref={canvasRef} sx={{ display: 'none' }} />

      {warning && (
        <Alert
          severity="warning"
          sx={{ width: 1, maxWidth: 420, textAlign: 'left' }}
          action={
            <Button color="inherit" size="small" onClick={() => void startCamera()}>
              Coba lagi
            </Button>
          }
        >
          {warning}
        </Alert>
      )}

      {/* Penampung pemindai USB.
          JANGAN pakai display:none — elemen yang disembunyikan begitu tidak
          bisa menerima fokus, sehingga pemindai tidak akan terbaca. Ukuran
          1px di dalam wrapper relatif; `width: 1` pada sx MUI berarti 100%
          dan akan menutupi seluruh layar. */}
      <Box sx={{ position: 'relative', width: 0, height: 0, overflow: 'hidden' }}>
        <Box
          component="input"
          ref={inputRef}
          aria-label="Hasil pemindaian QR"
          autoComplete="off"
          onBlur={focusInput}
          onKeyDown={(event: React.KeyboardEvent<HTMLInputElement>) => {
            if (event.key !== 'Enter') return;

            event.preventDefault();
            const input = event.currentTarget;
            const value = input.value.trim();
            input.value = '';
            if (value) onScanRef.current(value);
          }}
          sx={{ position: 'absolute', width: '1px', height: '1px', opacity: 0, border: 0, p: 0 }}
        />
      </Box>
    </Stack>
  );
}
