'use client';

import type { CheckInResult } from 'src/lib/api/mpp';

import { useRef, useState } from 'react';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { RouterLink } from 'src/routes/components';

import { ApiError } from 'src/lib/api/client';
import { useCheckInMutation } from 'src/lib/api/use-mpp';

import { TicketSection } from '../ticket-section';
import { QrScannerSection } from '../qr-scanner-section';

// ----------------------------------------------------------------------

/** Jendela abai untuk kode yang sama — kamera membacanya 10x per detik. */
const DEBOUNCE_MS = 3_000;

function messageFor(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.status) {
      case 409:
        return 'QR ini sudah dipakai untuk check-in. Silakan minta bantuan petugas.';
      case 410:
        return 'QR sudah kedaluwarsa atau bukan untuk hari ini. Silakan minta bantuan petugas.';
      case 404:
        return 'QR tidak dikenali. Pastikan Anda memindai kode dari halaman booking.';
      default:
        return error.message || 'Check-in gagal. Silakan minta bantuan petugas.';
    }
  }

  return 'Check-in gagal. Silakan minta bantuan petugas.';
}

export function KioskCheckInView() {
  const lastScanRef = useRef<{ token: string; at: number } | null>(null);

  const [result, setResult] = useState<CheckInResult | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const checkIn = useCheckInMutation();

  const handleScan = async (token: string) => {
    if (!token || checkIn.isPending) return;

    // Kamera mengirim kode yang sama berkali-kali per detik, dan pemindai
    // USB kadang mengulang. Tanpa peredam ini satu QR dibelanjakan
    // berkali-kali ke backend — yang kedua dan seterusnya pasti 409.
    const last = lastScanRef.current;
    if (last && last.token === token && Date.now() - last.at < DEBOUNCE_MS) return;
    lastScanRef.current = { token, at: Date.now() };

    setErrorMessage(null);

    try {
      setResult(await checkIn.mutateAsync(token));
    } catch (error) {
      setErrorMessage(messageFor(error));
    }
  };

  const reset = () => {
    setResult(null);
    setErrorMessage(null);
    lastScanRef.current = null;
  };

  if (result) {
    return (
      <TicketSection
        nomor={result.nomor}
        instansi={result.instansi.name}
        layanan={result.layanan.name}
        etaMenit={result.eta_menit}
        onDone={reset}
      />
    );
  }

  return (
    <Stack
      spacing={{ xs: 1.5, sm: 2 }}
      sx={{ width: 1, height: 1, alignItems: 'center', textAlign: 'center', minHeight: 0 }}
    >
      <Box sx={{ flexShrink: 0 }}>
        <Typography sx={{ typography: { xs: 'h4', sm: 'h3' } }}>Pindai QR Anda</Typography>
        <Typography sx={{ color: 'text.secondary', typography: { xs: 'body2', sm: 'body1' } }}>
          Arahkan QR ke kamera, atau gunakan pemindai di bawah layar.
        </Typography>
      </Box>

      <QrScannerSection onScan={handleScan} paused={checkIn.isPending || Boolean(result)} />

      <Box sx={{ width: 1, maxWidth: 420, flexShrink: 0 }}>
        {errorMessage ? (
          <Alert severity="error" sx={{ textAlign: 'left' }}>
            {errorMessage}
          </Alert>
        ) : (
          <Typography variant="body2" sx={{ color: 'primary.main' }}>
            {checkIn.isPending ? 'Memproses…' : 'Menunggu pemindaian…'}
          </Typography>
        )}
      </Box>

      <Stack
        direction="row"
        spacing={1}
        sx={{ flexShrink: 0, justifyContent: 'center', flexWrap: 'wrap', gap: 1 }}
      >
        <Button variant="outlined" component={RouterLink} href={paths.mpp.kiosk.root}>
          Kembali
        </Button>
        <Button variant="text" component={RouterLink} href={paths.mpp.kiosk.walkin}>
          Tidak punya QR? Daftar di sini
        </Button>
      </Stack>
    </Stack>
  );
}
