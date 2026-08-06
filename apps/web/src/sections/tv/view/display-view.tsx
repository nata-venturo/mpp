'use client';

import type { WsEvent } from 'src/lib/ws';

import { useRef, useMemo, useState, useEffect, useCallback } from 'react';

import Box from '@mui/material/Box';
import Grid from '@mui/material/Grid';
import Chip from '@mui/material/Chip';
import Card from '@mui/material/Card';
import Fade from '@mui/material/Fade';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Typography from '@mui/material/Typography';

import { CONFIG } from 'src/global-config';
import { useQueueSocket } from 'src/lib/ws';
import { useDisplayQuery } from 'src/lib/api/use-mpp';
import { isDeviceConfigured } from 'src/lib/api/device-client';
import { AudioQueue, AudioLeader, createSpeechEngine } from 'src/lib/tv/audio-queue';

// ----------------------------------------------------------------------

type Props = { instansiId: string };

export function DisplayView({ instansiId }: Props) {
  const configured = isDeviceConfigured('tv');

  const snapshot = useDisplayQuery(instansiId);
  const [leading, setLeading] = useState(false);
  /** Nomor yang baru saja dipanggil — dipakai untuk menyorot kartunya. */
  const [flash, setFlash] = useState<string | null>(null);

  const queueRef = useRef<AudioQueue | null>(null);
  const leaderRef = useRef<AudioLeader | null>(null);

  // refetch dibungkus ref: `snapshot` adalah objek baru setiap render,
  // dan handler socket tidak boleh ikut berubah karenanya.
  const refetchRef = useRef(snapshot.refetch);
  refetchRef.current = snapshot.refetch;

  // Satu mini-PC menyalakan beberapa jendela display yang berbagi satu
  // speaker: hanya jendela terpilih (leader) yang bersuara, dan ia memutar
  // satu pengumuman pada satu waktu (BR-18).
  useEffect(() => {
    const queue = new AudioQueue(createSpeechEngine('id-ID'));
    const leader = new AudioLeader();

    queueRef.current = queue;
    leaderRef.current = leader;

    leader.start(setLeading);

    return () => {
      leader.stop();
      queue.clear();
      queueRef.current = null;
      leaderRef.current = null;
    };
  }, []);

  const handleEvent = useCallback((event: WsEvent) => {
    const data = event.data ?? {};

    if (event.type === 'call.created' || event.type === 'call.recalled') {
      const text = typeof data.tts_text === 'string' ? data.tts_text : '';
      const antrianId = typeof data.antrian_id === 'string' ? data.antrian_id : '';
      const callCount = typeof data.call_count === 'number' ? data.call_count : 0;
      const nomor = typeof data.nomor === 'string' ? data.nomor : null;

      // Dedupe pakai antrian + jumlah panggilan: pengiriman event bersifat
      // at-least-once, jadi frame yang sama bisa datang dua kali.
      if (text && leaderRef.current?.isLeading()) {
        queueRef.current?.push({ id: `${antrianId}:${callCount}`, text });
      }

      setFlash(nomor);
    }

    // Setiap transisi antrean menggeser apa yang harus tampil. Server
    // sudah mengirim `queue.updated` untuk SETIAP perubahan, jadi satu
    // refetch di sini membuat layar ikut bergerak seketika — polling 20
    // detik pada useDisplayQuery tinggal jaring pengaman kalau socket mati.
    refetchRef.current();
  }, []);

  const { status } = useQueueSocket(
    configured ? [`display:${instansiId}`] : [],
    { kind: 'device', apiKey: CONFIG.tvApiKey },
    handleEvent
  );

  // Setiap kali socket (kembali) terbuka, tarik ulang snapshot. Tanpa ini
  // layar menampilkan keadaan sebelum putus sampai polling berikutnya —
  // persis saat operator paling butuh layar itu benar.
  useEffect(() => {
    if (status === 'open') refetchRef.current();
  }, [status]);

  // Sorotan nomor baru memudar sendiri; tanpa ini kartu tersorot selamanya.
  useEffect(() => {
    if (!flash) return undefined;

    const timer = setTimeout(() => setFlash(null), 8_000);
    return () => clearTimeout(timer);
  }, [flash]);

  const data = snapshot.data;
  const current = useMemo(() => data?.current ?? [], [data]);
  const next = useMemo(() => data?.next ?? [], [data]);

  if (!configured) {
    return (
      <Box sx={{ p: { xs: 3, md: 6 } }}>
        <Alert severity="warning">
          Display belum dikonfigurasi: setel <code>NEXT_PUBLIC_TV_API_KEY</code> lalu muat ulang.
        </Alert>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        height: 1,
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        p: { xs: 2, md: 4 },
        bgcolor: 'grey.900',
        color: 'common.white',
      }}
    >
      <Stack
        direction="row"
        spacing={2}
        sx={{
          flexShrink: 0,
          mb: { xs: 2, md: 3 },
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Typography sx={{ typography: { xs: 'h5', md: 'h3' } }}>
          {data?.instansi.name ?? 'Mall Pelayanan Publik'}
        </Typography>

        {/* Layar ini tidak berpenjaga. Kalau socket mati, antrean yang
            membeku terlihat persis seperti antrean yang sedang sepi —
            jadi statusnya harus terbaca dari seberang ruangan. */}
        <Stack direction="row" spacing={1} sx={{ flexShrink: 0, alignItems: 'center' }}>
          <Chip
            size="small"
            label={
              status === 'open' ? 'Live' : status === 'connecting' ? 'Menyambung…' : 'Terputus'
            }
            color={status === 'open' ? 'success' : status === 'connecting' ? 'warning' : 'error'}
            variant="soft"
          />
          <Chip
            size="small"
            label={leading ? 'Suara aktif' : 'Suara di layar lain'}
            color={leading ? 'success' : 'default'}
            variant="soft"
          />
        </Stack>
      </Stack>

      <Typography variant="overline" sx={{ flexShrink: 0, opacity: 0.7 }}>
        Sedang dipanggil
      </Typography>

      {/* Blok panggilan mengambil sisa tinggi dan menggulir DI DALAM
          dirinya kalau semua loket ramai — halamannya sendiri tidak
          pernah bergerak. */}
      <Grid
        container
        spacing={{ xs: 1.5, md: 3 }}
        sx={{ mt: 0.5, mb: { xs: 2, md: 3 }, flex: '1 1 auto', minHeight: 0, overflowY: 'auto' }}
      >
        {current.length === 0 && (
          <Grid size={12}>
            <Typography variant="h5" sx={{ opacity: 0.6 }}>
              Belum ada nomor yang dipanggil.
            </Typography>
          </Grid>
        )}

        {current.map((call) => (
          <Grid key={call.antrian_id} size={{ xs: 12, md: 6, lg: 4 }}>
            <Fade in timeout={400}>
              <Card
                sx={{
                  p: { xs: 2, md: 4 },
                  textAlign: 'center',
                  bgcolor: 'common.white',
                  color: 'grey.900',
                  transition: 'box-shadow .3s, transform .3s',
                  ...(flash === call.nomor && {
                    transform: 'scale(1.03)',
                    boxShadow: (theme) => `0 0 0 6px ${theme.palette.warning.main}`,
                  }),
                }}
              >
                {/* Tipografi besar & kontras tinggi: dibaca dari seberang
                    ruang tunggu (NFR-UX-02). Ukuran ikut viewport supaya
                    tetap terbaca di TV 55" maupun monitor kecil. */}
                <Typography
                  sx={{ fontSize: 'clamp(48px, 9vw, 112px)', fontWeight: 800, lineHeight: 1 }}
                >
                  {call.nomor}
                </Typography>
                <Typography sx={{ mt: 1, typography: { xs: 'h6', md: 'h4' } }}>
                  {call.loket}
                </Typography>
                <Typography variant="body2" sx={{ mt: 1, color: 'text.secondary' }}>
                  {call.status === 'SERVING' ? 'Sedang dilayani' : 'Silakan menuju loket'}
                </Typography>
              </Card>
            </Fade>
          </Grid>
        ))}
      </Grid>

      <Typography variant="overline" sx={{ flexShrink: 0, opacity: 0.7 }}>
        Berikutnya
      </Typography>

      <Stack
        direction="row"
        spacing={2}
        sx={{ flexShrink: 0, mt: 1, flexWrap: 'wrap', gap: { xs: 1, md: 2 }, overflow: 'hidden' }}
      >
        {next.length === 0 && (
          <Typography variant="h6" sx={{ opacity: 0.6 }}>
            Tidak ada antrean menunggu.
          </Typography>
        )}

        {next.map((item) => (
          <Box
            key={item.antrian_id}
            sx={{
              px: { xs: 2, md: 3 },
              py: { xs: 1, md: 2 },
              borderRadius: 1,
              border: '2px solid',
              borderColor: 'grey.700',
            }}
          >
            <Typography
              sx={{ fontSize: 'clamp(24px, 3.5vw, 40px)', fontWeight: 700, lineHeight: 1 }}
            >
              {item.nomor}
            </Typography>
            <Typography variant="caption" sx={{ opacity: 0.7 }}>
              {item.layanan}
            </Typography>
          </Box>
        ))}
      </Stack>
    </Box>
  );
}
