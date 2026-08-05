import Box from '@mui/material/Box';

// ----------------------------------------------------------------------
// Shell kiosk: SATU layar penuh, tidak pernah di-scroll.
//
// Perangkat ini berdiri di lobi dan dioperasikan sambil berdiri — kalau
// tombol jatuh di bawah lipatan layar, orang tidak akan menemukannya.
// Jadi tinggi dikunci ke viewport dan isinya yang menyesuaikan (lihat
// `minHeight: 0` di dalam view: tanpa itu, anak flex menolak menyusut
// dan justru memaksa halaman memanjang).
//
// 100dvh, bukan 100vh: di browser mobile, vh ikut menghitung bilah URL
// yang hilang-timbul, sehingga layar "penuh" tetap bisa di-scroll sedikit.

type Props = {
  children: React.ReactNode;
};

export default function Layout({ children }: Props) {
  return (
    <Box
      sx={{
        height: '100dvh',
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        p: { xs: 2, sm: 3 },
        bgcolor: 'background.default',
      }}
    >
      <Box
        sx={{
          width: 1,
          height: 1,
          maxWidth: 720,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
