'use client';

import Card from '@mui/material/Card';
import Alert from '@mui/material/Alert';
import Stack from '@mui/material/Stack';
import Divider from '@mui/material/Divider';
import Container from '@mui/material/Container';
import Typography from '@mui/material/Typography';

import { useBookingDetailQuery } from 'src/lib/api/use-mpp';

import { BookingQrSection } from '../booking-qr-section';

// ----------------------------------------------------------------------

type Props = { id: string };

/** Instant UTC → waktu lokal perangkat (WIB/WITA/WIT). */
function formatLocal(value?: string | null) {
  if (!value) return '-';
  return new Date(value).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' });
}

export function BookingConfirmView({ id }: Props) {
  const { data, isPending, isError, error } = useBookingDetailQuery(id);

  if (isPending) {
    return (
      <Container maxWidth="sm" sx={{ py: 5 }}>
        <Typography>Memuat booking…</Typography>
      </Container>
    );
  }

  if (isError || !data) {
    return (
      <Container maxWidth="sm" sx={{ py: 5 }}>
        <Alert severity="error">
          Booking tidak ditemukan. {(error as Error | null)?.message ?? ''}
        </Alert>
      </Container>
    );
  }

  return (
    <Container maxWidth="sm" sx={{ py: { xs: 3, sm: 5 } }}>
      <Typography sx={{ mb: 1, typography: { xs: 'h4', sm: 'h3' } }}>Booking berhasil</Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Simpan QR di bawah ini. Anda membutuhkannya untuk check-in di lokasi.
      </Typography>

      <Card sx={{ p: { xs: 2, sm: 3 } }}>
        <Stack spacing={1} sx={{ mb: 3 }}>
          <Typography variant="h6">{data.instansi.name}</Typography>
          <Typography variant="body2">{data.layanan.name}</Typography>
          <Typography variant="body2">Tanggal: {data.tanggal}</Typography>
          <Typography variant="body2">Atas nama: {data.pemohon_name}</Typography>
          <Typography variant="body2">Status: {data.status}</Typography>
        </Stack>

        <Divider sx={{ mb: 3 }} />

        {data.qr_token ? (
          <>
            <BookingQrSection token={data.qr_token} fileName={`qr-mpp-${data.id}`} />
            <Typography variant="caption" sx={{ display: 'block', mt: 2, textAlign: 'center' }}>
              Berlaku sampai {formatLocal(data.qr_expires_at)}
            </Typography>
          </>
        ) : (
          <Alert severity="warning">
            QR sudah tidak tersedia untuk booking ini (sudah dipakai atau dibatalkan).
          </Alert>
        )}
      </Card>
    </Container>
  );
}
