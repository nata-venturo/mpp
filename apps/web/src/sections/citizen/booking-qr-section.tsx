'use client';

import { useRef } from 'react';
import { QRCodeCanvas } from 'qrcode.react';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

// ----------------------------------------------------------------------

type Props = {
  token: string;
  fileName: string;
};

export function BookingQrSection({ token, fileName }: Props) {
  const wrapperRef = useRef<HTMLDivElement>(null);

  // QR memuat token opaque itu sendiri — kiosk mengirim persis string ini
  // ke /mpp/v1/checkin. Tidak ada data pribadi yang ikut di dalam kode.
  const handleDownload = () => {
    const canvas = wrapperRef.current?.querySelector('canvas');
    if (!canvas) return;

    const link = document.createElement('a');
    link.download = `${fileName}.png`;
    link.href = canvas.toDataURL('image/png');
    link.click();
  };

  return (
    <Stack spacing={2} sx={{ alignItems: 'center' }}>
      {/* Kanvas QR punya lebar piksel tetap; batasi wadahnya supaya di
          layar sempit ia mengecil, bukan mendorong halaman ke samping. */}
      <Box
        ref={wrapperRef}
        sx={{
          p: 2,
          maxWidth: 1,
          bgcolor: 'common.white',
          borderRadius: 1,
          lineHeight: 0,
          '& canvas': { maxWidth: '100%', height: 'auto' },
        }}
      >
        <QRCodeCanvas value={token} size={220} level="M" marginSize={2} />
      </Box>

      <Typography variant="body2" sx={{ color: 'text.secondary', textAlign: 'center' }}>
        Tunjukkan QR ini di kiosk MPP untuk check-in.
      </Typography>

      <Button variant="outlined" size="large" onClick={handleDownload}>
        Unduh QR
      </Button>
    </Stack>
  );
}
