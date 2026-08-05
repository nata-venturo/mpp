import { MainLayout } from 'src/layouts/main';

// ----------------------------------------------------------------------
// Grup rute warga: alur pendaftaran publik memakai shell situs yang sama
// dengan landing page, jadi orang tidak merasa berpindah aplikasi.

type Props = {
  children: React.ReactNode;
};

export default function Layout({ children }: Props) {
  return <MainLayout>{children}</MainLayout>;
}
