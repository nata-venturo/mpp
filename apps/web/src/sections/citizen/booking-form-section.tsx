'use client';

import type { Layanan, Instansi, Availability } from 'src/lib/api/mpp';

import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Alert from '@mui/material/Alert';
import Stack from '@mui/material/Stack';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import MenuItem from '@mui/material/MenuItem';
import Typography from '@mui/material/Typography';

import { Form, Field } from 'src/components/hook-form';

// ----------------------------------------------------------------------

export const BookingFormSchema = z.object({
  instansi_id: z.string().min(1, { message: 'Pilih instansi' }),
  layanan_id: z.string().min(1, { message: 'Pilih layanan' }),
  tanggal: z.string().min(1, { message: 'Pilih tanggal' }),
  name: z.string().min(2, { message: 'Nama minimal 2 karakter' }),
  phone: z.string().min(6, { message: 'Nomor WhatsApp tidak valid' }),
  email: z.string().email({ message: 'Email tidak valid' }).or(z.literal('')),
});

export type BookingFormValues = z.infer<typeof BookingFormSchema>;

type Props = {
  instansiList: Instansi[];
  layananList: Layanan[];
  availability?: Availability;
  isLoadingLayanan: boolean;
  isCheckingQuota: boolean;
  errorMessage?: string | null;
  onChange: (values: Partial<BookingFormValues>) => void;
  onSubmit: (values: BookingFormValues) => Promise<void>;
};

/** Tanggal hari ini dalam waktu perangkat, format YYYY-MM-DD. */
function todayISO() {
  const now = new Date();
  const offset = now.getTimezoneOffset() * 60_000;
  return new Date(now.getTime() - offset).toISOString().slice(0, 10);
}

export function BookingFormSection({
  instansiList,
  layananList,
  availability,
  isLoadingLayanan,
  isCheckingQuota,
  errorMessage,
  onChange,
  onSubmit,
}: Props) {
  const methods = useForm<BookingFormValues>({
    resolver: zodResolver(BookingFormSchema),
    defaultValues: {
      instansi_id: '',
      layanan_id: '',
      tanggal: todayISO(),
      name: '',
      phone: '',
      email: '',
    },
  });

  const values = methods.watch();
  const selectedLayanan = layananList.find((layanan) => layanan.id === values.layanan_id);

  // Kuota nol harus menghentikan pengiriman SEBELUM request: backend tetap
  // menolak dengan 409, ini hanya supaya warga tidak mengisi formulir sia-sia.
  const quotaExhausted = Boolean(availability && availability.remaining <= 0);

  const handleSubmit = methods.handleSubmit(async (formValues) => {
    await onSubmit(formValues);
  });

  return (
    <Form methods={methods} onSubmit={handleSubmit}>
      <Stack spacing={3}>
        <Field.Select
          name="instansi_id"
          label="Instansi"
          onChange={(event) => {
            const instansiId = event.target.value;
            methods.setValue('instansi_id', instansiId);
            methods.setValue('layanan_id', '');
            onChange({ instansi_id: instansiId, layanan_id: '' });
          }}
        >
          {instansiList.map((instansi) => (
            <MenuItem key={instansi.id} value={instansi.id}>
              {instansi.name}
            </MenuItem>
          ))}
        </Field.Select>

        <Field.Select
          name="layanan_id"
          label={isLoadingLayanan ? 'Memuat layanan…' : 'Layanan'}
          disabled={!values.instansi_id || isLoadingLayanan}
          onChange={(event) => {
            const layananId = event.target.value;
            methods.setValue('layanan_id', layananId);
            onChange({ layanan_id: layananId });
          }}
        >
          {layananList.map((layanan) => (
            <MenuItem key={layanan.id} value={layanan.id}>
              {layanan.name} · ± {layanan.estimasi_durasi_menit} menit
            </MenuItem>
          ))}
        </Field.Select>

        {selectedLayanan && selectedLayanan.syarat_dokumen.length > 0 && (
          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Dokumen yang perlu dibawa
            </Typography>
            <Stack spacing={0.5}>
              {selectedLayanan.syarat_dokumen.map((syarat) => (
                <Typography key={syarat.id} variant="body2" sx={{ color: 'text.secondary' }}>
                  • {syarat.name}
                  {syarat.is_required ? '' : ' (opsional)'}
                  {syarat.notes ? ` — ${syarat.notes}` : ''}
                </Typography>
              ))}
            </Stack>
          </Box>
        )}

        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={2}
          sx={{ alignItems: 'flex-start' }}
        >
          <Field.Text
            name="tanggal"
            label="Tanggal kunjungan"
            type="date"
            slotProps={{ inputLabel: { shrink: true }, htmlInput: { min: todayISO() } }}
            onChange={(event) => {
              methods.setValue('tanggal', event.target.value);
              onChange({ tanggal: event.target.value });
            }}
          />

          {values.tanggal && values.instansi_id && (
            <Chip
              sx={{ mt: 1 }}
              color={quotaExhausted ? 'error' : 'success'}
              variant="soft"
              label={
                isCheckingQuota
                  ? 'Mengecek kuota…'
                  : quotaExhausted
                    ? 'Kuota tanggal ini penuh'
                    : `Sisa kuota: ${availability?.remaining ?? 0}`
              }
            />
          )}
        </Stack>

        <Divider />

        <Typography variant="subtitle1">Data pemohon</Typography>

        <Field.Text name="name" label="Nama lengkap" />
        <Field.Text name="phone" label="Nomor WhatsApp" placeholder="6281…" />
        <Field.Text name="email" label="Email (opsional)" />

        {errorMessage && <Alert severity="error">{errorMessage}</Alert>}

        <Button
          type="submit"
          size="large"
          variant="contained"
          disabled={methods.formState.isSubmitting || quotaExhausted}
        >
          {methods.formState.isSubmitting ? 'Mendaftarkan…' : 'Daftar sekarang'}
        </Button>
      </Stack>
    </Form>
  );
}
