'use client';

import type { AntrianAction, AntrianActionKind } from 'src/lib/api/mpp';

import { useState, useEffect } from 'react';

import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Select from '@mui/material/Select';
import MenuItem from '@mui/material/MenuItem';
import Snackbar from '@mui/material/Snackbar';
import Container from '@mui/material/Container';
import InputLabel from '@mui/material/InputLabel';
import Typography from '@mui/material/Typography';
import FormControl from '@mui/material/FormControl';

import { paths } from 'src/routes/paths';
import { useRouter } from 'src/routes/hooks';

import { useQueueSocket } from 'src/lib/ws';
import { ApiError } from 'src/lib/api/client';
import { useAccessToken } from 'src/lib/api/use-auth';
import {
  useLoketQuery,
  useQueueQuery,
  useInstansiQuery,
  useCallNextMutation,
  useLoketSessionMutation,
  useAntrianActionMutation,
} from 'src/lib/api/use-mpp';

// ----------------------------------------------------------------------

/** Aksi yang boleh dijalankan pada tiap status — juga dijaga backend. */
const ALLOWED: Record<string, AntrianActionKind[]> = {
  CALLED: ['recall', 'start', 'skip'],
  SERVING: ['done'],
};

export function LoketPanelView() {
  const router = useRouter();
  const token = useAccessToken();

  const [instansiId, setInstansiId] = useState('');
  const [loketId, setLoketId] = useState('');
  const [sessionOpen, setSessionOpen] = useState(false);
  const [current, setCurrent] = useState<AntrianAction | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const instansiQuery = useInstansiQuery();
  const loketQuery = useLoketQuery(instansiId);

  const sessionMutation = useLoketSessionMutation();
  const callNext = useCallNextMutation();
  const action = useAntrianActionMutation();

  const layananId = current?.layanan_id ?? '';
  const queueQuery = useQueueQuery(layananId);

  // Antrean bergerak karena loket lain juga memanggil; dengarkan stream
  // layanan yang sedang dikerjakan supaya daftar tunggu ikut segar.
  useQueueSocket(layananId ? [`layanan:${layananId}`] : [], { kind: 'staff' }, () => {
    queueQuery.refetch();
  });

  // Panel ini butuh sesi staf. Tanpa token, arahkan ke login dan kembali
  // ke sini setelah berhasil.
  useEffect(() => {
    if (token === null) {
      router.replace(`${paths.signIn}?next=${encodeURIComponent(paths.mpp.loket)}`);
    }
  }, [token, router]);

  const runAction = async (kind: AntrianActionKind) => {
    if (!current) return;

    try {
      const result = await action.mutateAsync({ id: current.antrian_id, kind });
      setCurrent(kind === 'done' || kind === 'skip' ? null : result);
      setNotice(
        kind === 'done'
          ? `Selesai — ${result.nomor}`
          : kind === 'skip'
            ? `Dilewati — ${result.nomor}`
            : `${result.nomor} · panggilan ke-${result.call_count}`
      );
    } catch (error) {
      // 409 = status antrean sudah berubah (mis. panggilan ke-4, atau
      // "Mulai" pada nomor yang belum dipanggil). Bukan crash, cuma tolakan.
      setNotice(
        error instanceof ApiError && error.status === 409
          ? 'Aksi ini tidak berlaku untuk status antrean saat ini.'
          : error instanceof Error
            ? error.message
            : 'Aksi gagal.'
      );
    }
  };

  const handleCallNext = async () => {
    try {
      const result = await callNext.mutateAsync(loketId);
      if (!result) {
        setNotice('Tidak ada antrean menunggu.');
        return;
      }
      setCurrent(result);
      setNotice(`Memanggil ${result.nomor}`);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : 'Gagal memanggil antrean.');
    }
  };

  const toggleSession = async () => {
    try {
      const result = await sessionMutation.mutateAsync({
        loketId,
        action: sessionOpen ? 'close' : 'open',
      });
      setSessionOpen(result.is_active);
      setNotice(result.is_active ? `Sesi ${result.loket} dibuka` : `Sesi ${result.loket} ditutup`);
    } catch (error) {
      setNotice(
        error instanceof ApiError && error.status === 403
          ? 'Loket ini sedang dipegang petugas lain.'
          : error instanceof Error
            ? error.message
            : 'Gagal mengubah sesi.'
      );
    }
  };

  const allowed = current ? (ALLOWED[current.status] ?? []) : [];

  return (
    <Container maxWidth="md" sx={{ py: { xs: 2, sm: 4 } }}>
      <Typography sx={{ mb: { xs: 2, sm: 3 }, typography: { xs: 'h5', sm: 'h4' } }}>
        Panel loket
      </Typography>

      <Card sx={{ p: { xs: 2, sm: 3 }, mb: { xs: 2, sm: 3 } }}>
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={2}
          sx={{ alignItems: { xs: 'stretch', sm: 'center' } }}
        >
          <FormControl fullWidth>
            <InputLabel id="instansi-label">Instansi</InputLabel>
            <Select
              labelId="instansi-label"
              label="Instansi"
              value={instansiId}
              onChange={(event) => {
                setInstansiId(event.target.value);
                setLoketId('');
                setSessionOpen(false);
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
            <InputLabel id="loket-label">Loket</InputLabel>
            <Select
              labelId="loket-label"
              label="Loket"
              value={loketId}
              onChange={(event) => {
                setLoketId(event.target.value);
                setSessionOpen(false);
              }}
            >
              {(loketQuery.data ?? []).map((loket) => (
                <MenuItem key={loket.id} value={loket.id}>
                  {loket.name} · {loket.status}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <Button
            size="large"
            variant={sessionOpen ? 'outlined' : 'contained'}
            disabled={!loketId || sessionMutation.isPending}
            onClick={toggleSession}
            sx={{ flexShrink: 0, minWidth: { sm: 160 } }}
          >
            {sessionOpen ? 'Tutup sesi' : 'Buka sesi'}
          </Button>
        </Stack>
      </Card>

      {!sessionOpen && loketId && (
        <Alert severity="info" sx={{ mb: 3 }}>
          Buka sesi loket sebelum memanggil antrean.
        </Alert>
      )}

      <Card sx={{ p: { xs: 2, sm: 3 }, mb: { xs: 2, sm: 3 } }}>
        <Typography variant="overline" sx={{ color: 'text.secondary' }}>
          Sekarang
        </Typography>

        {current ? (
          <Box sx={{ mt: 1 }}>
            <Stack
              direction="row"
              spacing={2}
              sx={{ mb: 1, alignItems: 'center', flexWrap: 'wrap', gap: 1 }}
            >
              <Typography sx={{ fontSize: { xs: 40, sm: 56 }, fontWeight: 800, lineHeight: 1 }}>
                {current.nomor}
              </Typography>
              <Chip label={current.status} color="primary" variant="soft" />
              <Chip label={`Panggilan ke-${current.call_count}`} variant="outlined" />
            </Stack>
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
              {current.pemohon_name} · {current.layanan}
            </Typography>
          </Box>
        ) : (
          <Typography variant="body1" sx={{ mt: 1, color: 'text.secondary' }}>
            Belum ada antrean yang sedang dilayani.
          </Typography>
        )}

        <Box
          sx={{
            mt: 3,
            display: 'grid',
            gap: 1.5,
            gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(auto-fit, minmax(150px, 1fr))' },
          }}
        >
          <Button
            size="large"
            variant="contained"
            disabled={!sessionOpen || Boolean(current) || callNext.isPending}
            onClick={handleCallNext}
          >
            Panggil berikutnya
          </Button>
          <Button
            size="large"
            variant="outlined"
            disabled={!allowed.includes('recall')}
            onClick={() => runAction('recall')}
          >
            Panggil ulang
          </Button>
          <Button
            size="large"
            variant="outlined"
            disabled={!allowed.includes('start')}
            onClick={() => runAction('start')}
          >
            Mulai
          </Button>
          <Button
            size="large"
            variant="outlined"
            color="warning"
            disabled={!allowed.includes('skip')}
            onClick={() => runAction('skip')}
          >
            Lewati
          </Button>
          <Button
            size="large"
            variant="contained"
            color="success"
            disabled={!allowed.includes('done')}
            onClick={() => runAction('done')}
          >
            Selesai
          </Button>
        </Box>
      </Card>

      <Card sx={{ p: { xs: 2, sm: 3 } }}>
        <Typography variant="overline" sx={{ color: 'text.secondary' }}>
          Menunggu
        </Typography>

        <Stack direction="row" spacing={1} sx={{ mt: 1.5, flexWrap: 'wrap', gap: 1 }}>
          {(queueQuery.data ?? []).length === 0 && (
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
              {layananId
                ? 'Tidak ada antrean menunggu.'
                : 'Panggil satu antrean untuk melihat stream layanan.'}
            </Typography>
          )}

          {(queueQuery.data ?? []).map((item) => (
            <Chip
              key={item.antrian_id}
              label={item.nomor}
              variant={item.source === 'BOOKING' ? 'filled' : 'outlined'}
            />
          ))}
        </Stack>
      </Card>

      <Snackbar
        open={Boolean(notice)}
        autoHideDuration={4000}
        onClose={() => setNotice(null)}
        message={notice ?? ''}
      />
    </Container>
  );
}
