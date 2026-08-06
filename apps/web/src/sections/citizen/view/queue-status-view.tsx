'use client';

import { useState } from 'react';

import Card from '@mui/material/Card';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Table from '@mui/material/Table';
import Select from '@mui/material/Select';
import TableRow from '@mui/material/TableRow';
import MenuItem from '@mui/material/MenuItem';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import Container from '@mui/material/Container';
import InputLabel from '@mui/material/InputLabel';
import Typography from '@mui/material/Typography';
import FormControl from '@mui/material/FormControl';

import { useQueueQuery, useLayananQuery, useInstansiQuery } from 'src/lib/api/use-mpp';

// ----------------------------------------------------------------------

export function QueueStatusView() {
  const [instansiId, setInstansiId] = useState('');
  const [layananId, setLayananId] = useState('');

  const instansiQuery = useInstansiQuery();
  const layananQuery = useLayananQuery(instansiId);
  // Stream antrean butuh izin staf; warga melihatnya lewat layar TV di
  // lokasi. Di sini kegagalan 401/403 ditampilkan apa adanya.
  const queueQuery = useQueueQuery(layananId, { refetchInterval: 15_000 });

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Typography sx={{ mb: 1, typography: { xs: 'h4', sm: 'h3' } }}>Status antrean</Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Pantau nomor yang sedang menunggu untuk sebuah layanan.
      </Typography>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 3 }}>
        <FormControl fullWidth>
          <InputLabel id="instansi-label">Instansi</InputLabel>
          <Select
            labelId="instansi-label"
            label="Instansi"
            value={instansiId}
            onChange={(event) => {
              setInstansiId(event.target.value);
              setLayananId('');
            }}
          >
            {(instansiQuery.data ?? []).map((instansi) => (
              <MenuItem key={instansi.id} value={instansi.id}>
                {instansi.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl fullWidth disabled={!instansiId}>
          <InputLabel id="layanan-label">Layanan</InputLabel>
          <Select
            labelId="layanan-label"
            label="Layanan"
            value={layananId}
            onChange={(event) => setLayananId(event.target.value)}
          >
            {(layananQuery.data ?? []).map((layanan) => (
              <MenuItem key={layanan.id} value={layanan.id}>
                {layanan.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>

      {queueQuery.isError && (
        <Alert severity="info">
          Stream antrean hanya dapat dibaca petugas. Lihat layar TV di lokasi untuk nomor yang
          sedang dipanggil.
        </Alert>
      )}

      {/* Tabel menggulir DI DALAM kartunya. Tanpa ini, tabel sempit
          sekalipun akan membuat seluruh halaman bisa digeser. */}
      {layananId && queueQuery.data && (
        <Card sx={{ overflowX: 'auto' }}>
          <Table sx={{ minWidth: 480 }}>
            <TableHead>
              <TableRow>
                <TableCell>Nomor</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Sumber</TableCell>
                <TableCell>Loket</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {queueQuery.data.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4}>
                    <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                      Belum ada antrean menunggu.
                    </Typography>
                  </TableCell>
                </TableRow>
              )}

              {queueQuery.data.map((item) => (
                <TableRow key={item.antrian_id}>
                  <TableCell>
                    <Typography variant="subtitle1">{item.nomor}</Typography>
                  </TableCell>
                  <TableCell>{item.status}</TableCell>
                  <TableCell>{item.source === 'BOOKING' ? 'Booking' : 'Walk-in'}</TableCell>
                  <TableCell>{item.loket ?? '-'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </Container>
  );
}
