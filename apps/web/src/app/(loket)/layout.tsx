import Box from '@mui/material/Box';

// ----------------------------------------------------------------------
// Aplikasi loket: dipakai petugas sepanjang hari di satu layar, jadi
// tanpa navigasi situs — hanya panel kerjanya.

type Props = {
  children: React.ReactNode;
};

export default function Layout({ children }: Props) {
  return <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>{children}</Box>;
}
