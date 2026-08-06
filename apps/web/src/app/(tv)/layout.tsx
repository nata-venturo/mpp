import Box from '@mui/material/Box';

// ----------------------------------------------------------------------
// Display TV: penuh layar, tanpa kromium situs. Layar ini tidak pernah
// disentuh siapa pun — hanya dilihat dan didengar, dari seberang ruang
// tunggu.

type Props = {
  children: React.ReactNode;
};

export default function Layout({ children }: Props) {
  // Layar TV tidak pernah di-scroll: tidak ada yang menyentuhnya, jadi
  // apa pun yang jatuh di bawah lipatan layar tidak akan pernah terlihat.
  return <Box sx={{ height: '100dvh', overflow: 'hidden' }}>{children}</Box>;
}
