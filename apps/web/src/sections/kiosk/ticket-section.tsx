'use client';

import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import Typography from '@mui/material/Typography';

// ----------------------------------------------------------------------

type Props = {
  nomor: string;
  instansi: string;
  layanan: string;
  etaMenit: number;
  onDone: () => void;
};

/**
 * Karcis antrean.
 *
 * Dicetak lewat dialog cetak browser (`window.print()`), dengan stylesheet
 * `@media print` yang menyembunyikan seluruh halaman kecuali karcis —
 * printer termal apa pun yang dikenal sistem operasi ikut bekerja tanpa
 * driver khusus.
 *
 * Tidak ada data pribadi di atas kertas: hanya nomor, layanan, dan waktu.
 *
 * ponytail: agen ESC/POS lokal bisa menggantikan ini kalau kualitas cetak
 * atau kecepatan jadi masalah — kontraknya tetap komponen ini saja.
 */
export function TicketSection({ nomor, instansi, layanan, etaMenit, onDone }: Props) {
  return (
    <>
      <style>{`
        @media print {
          body * { visibility: hidden !important; }
          #mpp-ticket, #mpp-ticket * { visibility: visible !important; }
          #mpp-ticket {
            position: absolute; inset: 0;
            margin: 0; padding: 12px;
            width: 100%;
            color: #000; background: #fff;
          }
          .mpp-no-print { display: none !important; }
        }
      `}</style>

      <Stack
        spacing={{ xs: 2, sm: 3 }}
        sx={{ width: 1, height: 1, alignItems: 'center', justifyContent: 'center', minHeight: 0 }}
      >
        <Box
          id="mpp-ticket"
          sx={{
            p: { xs: 2, sm: 3 },
            width: 1,
            maxWidth: 360,
            minHeight: 0,
            overflow: 'auto',
            textAlign: 'center',
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1,
            bgcolor: 'background.paper',
          }}
        >
          <Typography variant="overline" sx={{ color: 'text.secondary' }}>
            Nomor antrean Anda
          </Typography>

          {/* Angka mengecil sendiri di layar pendek, supaya karcis tidak
              pernah mendorong tombol keluar dari layar. */}
          <Typography
            sx={{
              fontSize: { xs: 56, sm: 72 },
              fontWeight: 800,
              lineHeight: 1.1,
              my: 1,
            }}
          >
            {nomor}
          </Typography>

          <Divider sx={{ my: 2 }} />

          <Typography variant="subtitle1">{instansi}</Typography>
          <Typography variant="body2" sx={{ mb: 1 }}>
            {layanan}
          </Typography>
          <Typography variant="body2">
            {new Date().toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' })}
          </Typography>

          <Typography variant="body2" sx={{ mt: 2 }}>
            Perkiraan menunggu: ± {etaMenit} menit
          </Typography>
          <Typography variant="caption" sx={{ display: 'block', mt: 2 }}>
            Perhatikan layar TV dan dengarkan panggilan nomor Anda.
          </Typography>
        </Box>

        <Stack
          direction="row"
          spacing={2}
          className="mpp-no-print"
          sx={{ flexShrink: 0, flexWrap: 'wrap', gap: 1, justifyContent: 'center' }}
        >
          <Button size="large" variant="contained" onClick={() => window.print()}>
            Cetak karcis
          </Button>
          <Button size="large" variant="outlined" onClick={onDone}>
            Selesai
          </Button>
        </Stack>
      </Stack>
    </>
  );
}
