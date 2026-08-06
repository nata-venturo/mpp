'use client';

import type { Antrian } from 'src/lib/api/mpp';

import { z } from 'zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import MenuItem from '@mui/material/MenuItem';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { RouterLink } from 'src/routes/components';

import { useLayananQuery, useInstansiQuery, useWalkInMutation } from 'src/lib/api/use-mpp';

import { Form, Field } from 'src/components/hook-form';

import { TicketSection } from '../ticket-section';

// ----------------------------------------------------------------------

const WalkInSchema = z.object({
  instansi_id: z.string().min(1, { message: 'Pilih instansi' }),
  layanan_id: z.string().min(1, { message: 'Pilih layanan' }),
  name: z.string().min(2, { message: 'Nama minimal 2 karakter' }),
  phone: z.string().min(6, { message: 'Nomor telepon tidak valid' }),
});

type WalkInValues = z.infer<typeof WalkInSchema>;

export function KioskWalkInView() {
  const [result, setResult] = useState<Antrian | null>(null);
  const [labels, setLabels] = useState({ instansi: '', layanan: '' });
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const methods = useForm<WalkInValues>({
    resolver: zodResolver(WalkInSchema),
    defaultValues: { instansi_id: '', layanan_id: '', name: '', phone: '' },
  });

  const instansiId = methods.watch('instansi_id');

  // Kiosk membaca katalog dengan kunci perangkat, bukan sesi warga.
  const instansiQuery = useInstansiQuery({ device: true });
  const layananQuery = useLayananQuery(instansiId, { device: true });
  const walkIn = useWalkInMutation();

  const onSubmit = methods.handleSubmit(async (values) => {
    setErrorMessage(null);

    try {
      const antrian = await walkIn.mutateAsync({
        instansi_id: values.instansi_id,
        layanan_id: values.layanan_id,
        pemohon: { name: values.name, phone: values.phone },
      });

      setLabels({
        instansi: instansiQuery.data?.find((i) => i.id === values.instansi_id)?.name ?? '',
        layanan: layananQuery.data?.find((l) => l.id === values.layanan_id)?.name ?? '',
      });
      setResult(antrian);
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : 'Pendaftaran gagal. Silakan minta bantuan petugas.'
      );
    }
  });

  if (result) {
    return (
      <TicketSection
          nomor={result.nomor}
        instansi={labels.instansi}
        layanan={labels.layanan}
        etaMenit={result.eta_menit}
        onDone={() => {
          setResult(null);
          methods.reset();
        }}
      />
    );
  }

  return (
    <Stack sx={{ width: 1, height: 1, minHeight: 0 }}>
      <Box sx={{ flexShrink: 0, mb: 2 }}>
        <Typography sx={{ typography: { xs: 'h4', sm: 'h3' } }}>Daftar tanpa booking</Typography>
        <Typography sx={{ color: 'text.secondary', typography: { xs: 'body2', sm: 'body1' } }}>
          Pilih layanan yang Anda butuhkan, lalu isi data singkat.
        </Typography>
      </Box>

      {/* Halaman kiosk tidak boleh di-scroll, tetapi formulir bisa lebih
          tinggi dari layar pendek — jadi yang menggulir isinya, bukan
          halamannya. */}
      <Card sx={{ p: { xs: 2, sm: 3 }, flex: '1 1 auto', minHeight: 0, overflowY: 'auto' }}>
        <Form methods={methods} onSubmit={onSubmit}>
          <Stack spacing={3}>
            <Field.Select
              name="instansi_id"
              label="Instansi"
              onChange={(event) => {
                methods.setValue('instansi_id', event.target.value);
                methods.setValue('layanan_id', '');
              }}
            >
              {(instansiQuery.data ?? []).map((instansi) => (
                <MenuItem key={instansi.id} value={instansi.id}>
                  {instansi.name}
                </MenuItem>
              ))}
            </Field.Select>

            <Field.Select name="layanan_id" label="Layanan" disabled={!instansiId}>
              {(layananQuery.data ?? []).map((layanan) => (
                <MenuItem key={layanan.id} value={layanan.id}>
                  {layanan.name} · ± {layanan.estimasi_durasi_menit} menit
                </MenuItem>
              ))}
            </Field.Select>

            <Field.Text name="name" label="Nama lengkap" />
            <Field.Text name="phone" label="Nomor telepon" placeholder="6281…" />

            {errorMessage && <Alert severity="error">{errorMessage}</Alert>}

            <Button
              type="submit"
              size="large"
              variant="contained"
              disabled={methods.formState.isSubmitting}
            >
              {methods.formState.isSubmitting ? 'Memproses…' : 'Ambil nomor antrean'}
            </Button>

            <Button size="large" variant="text" component={RouterLink} href={paths.mpp.kiosk.root}>
              Batal
            </Button>
          </Stack>
        </Form>
      </Card>
    </Stack>
  );
}
