'use client';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { RouterLink } from 'src/routes/components';

import { isDeviceConfigured } from 'src/lib/api/device-client';

// ----------------------------------------------------------------------

export function KioskIdleView() {
  const configured = isDeviceConfigured('kiosk');

  return (
    <Stack
      spacing={{ xs: 3, sm: 5 }}
      sx={{ width: 1, height: 1, justifyContent: 'center', textAlign: 'center', minHeight: 0 }}
    >
      <Box sx={{ flexShrink: 0 }}>
        <Typography sx={{ typography: { xs: 'h3', sm: 'h2' }, mb: 1 }}>
          Selamat datang di MPP
        </Typography>
        <Typography
          sx={{ color: 'text.secondary', typography: { xs: 'body1', sm: 'h6' }, fontWeight: 400 }}
        >
          Silakan pilih salah satu.
        </Typography>
      </Box>

      {!configured && (
        <Alert severity="warning" sx={{ textAlign: 'left' }}>
          Kiosk ini belum dikonfigurasi: setel <code>NEXT_PUBLIC_KIOSK_API_KEY</code> lalu muat
          ulang.
        </Alert>
      )}

      {/* Tombol besar & berjarak lebar: dioperasikan dengan jari, sering
          oleh orang yang berdiri sambil membawa berkas. Tingginya ikut
          layar supaya keduanya tetap muat tanpa scroll. */}
      <Stack spacing={{ xs: 2, sm: 3 }} sx={{ flexShrink: 0 }}>
        <Button
          size="large"
          variant="contained"
          component={RouterLink}
          href={paths.mpp.kiosk.checkin}
          sx={{ py: { xs: 2.5, sm: 4 }, fontSize: { xs: 18, sm: 24 } }}
        >
          Check-in dengan QR
        </Button>

        <Button
          size="large"
          variant="outlined"
          component={RouterLink}
          href={paths.mpp.kiosk.walkin}
          sx={{ py: { xs: 2.5, sm: 4 }, fontSize: { xs: 18, sm: 24 } }}
        >
          Daftar tanpa booking
        </Button>
      </Stack>
    </Stack>
  );
}
