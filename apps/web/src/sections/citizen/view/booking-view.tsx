'use client';

import type { BookingFormValues } from '../booking-form-section';

import { useState } from 'react';

import Card from '@mui/material/Card';
import Alert from '@mui/material/Alert';
import Container from '@mui/material/Container';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { useRouter } from 'src/routes/hooks';

import { ApiError } from 'src/lib/api/client';
import {
  useLayananQuery,
  useInstansiQuery,
  useAvailabilityQuery,
  useCreateBookingMutation,
} from 'src/lib/api/use-mpp';

import { BookingFormSection } from '../booking-form-section';

// ----------------------------------------------------------------------

export function BookingView() {
  const router = useRouter();

  // Pilihan yang sedang aktif di formulir, diangkat ke sini karena kuota
  // harus ikut berubah saat instansi/layanan/tanggal berganti.
  const [selection, setSelection] = useState({ instansi_id: '', layanan_id: '', tanggal: '' });
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const instansiQuery = useInstansiQuery();
  const layananQuery = useLayananQuery(selection.instansi_id);
  const availabilityQuery = useAvailabilityQuery({
    instansiId: selection.instansi_id,
    layananId: selection.layanan_id,
    date: selection.tanggal,
  });

  const createBooking = useCreateBookingMutation();

  const handleSubmit = async (values: BookingFormValues) => {
    setErrorMessage(null);

    try {
      const booking = await createBooking.mutateAsync({
        instansi_id: values.instansi_id,
        layanan_id: values.layanan_id,
        tanggal: values.tanggal,
        pemohon: {
          name: values.name,
          phone: values.phone,
          email: values.email ? values.email : null,
        },
      });

      router.push(paths.mpp.booking(booking.id));
    } catch (error) {
      // 409 bukan kesalahan pengisian: kuota habis diambil orang lain
      // sementara formulir terbuka. Katakan itu apa adanya.
      if (error instanceof ApiError && error.status === 409) {
        setErrorMessage('Maaf, kuota tanggal ini baru saja penuh. Silakan pilih tanggal lain.');
        availabilityQuery.refetch();
        return;
      }

      setErrorMessage(
        error instanceof Error ? error.message : 'Pendaftaran gagal. Silakan coba lagi.'
      );
    }
  };

  return (
    <Container maxWidth="sm" sx={{ py: { xs: 3, sm: 5 } }}>
      <Typography sx={{ mb: 1, typography: { xs: 'h4', sm: 'h3' } }}>
        Daftar layanan MPP
      </Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Pilih instansi dan layanan, tentukan tanggal, lalu isi data Anda. QR check-in akan
        diterbitkan setelah pendaftaran berhasil.
      </Typography>

      {instansiQuery.isError && (
        <Alert severity="error" sx={{ mb: 3 }}>
          Daftar instansi tidak dapat dimuat. Pastikan layanan MPP sedang aktif.
        </Alert>
      )}

      <Card sx={{ p: { xs: 2, sm: 3 } }}>
        <BookingFormSection
          instansiList={instansiQuery.data ?? []}
          layananList={layananQuery.data ?? []}
          availability={availabilityQuery.data}
          isLoadingLayanan={layananQuery.isFetching}
          isCheckingQuota={availabilityQuery.isFetching}
          errorMessage={errorMessage}
          onChange={(values) => setSelection((prev) => ({ ...prev, ...values }))}
          onSubmit={handleSubmit}
        />
      </Card>
    </Container>
  );
}
