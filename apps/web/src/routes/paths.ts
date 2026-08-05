// ----------------------------------------------------------------------
// Semua string route terpusat di sini — jangan hardcode URL di komponen.
//
// Kebijakan trailing slash: entri di bawah TANPA trailing slash (Next
// menormalkan saat navigasi karena `trailingSlash: true`). Untuk URL yang
// dipublikasikan ke crawler (canonical, sitemap, JSON-LD), SELALU tambahkan
// '/' di akhir — gunakan `pathWithSlash()` supaya konsisten.

export const pathWithSlash = (path: string) => (path.endsWith('/') ? path : `${path}/`);

export const paths = {
  home: '/',
  /**
   * Article
   */
  article: {
    root: '/article',
    details: (slug: string) => `/article/${slug}`,
  },
  /**
   * Common
   */
  maintenance: '/maintenance',
  comingSoon: '/coming-soon',
  support: '/support',
  page404: '/error/404',
  page500: '/error/500',
  /**
   * MPP — antrian Mall Pelayanan Publik
   *
   * Grup rute `(citizen)`/`(kiosk)`/`(loket)`/`(tv)` hanya berbagi layout;
   * segmen URL tetap eksplisit di bawah ini supaya tidak ada rute yang
   * bertabrakan dengan landing page di '/'.
   */
  mpp: {
    /** Warga: pilih instansi -> layanan -> tanggal -> isi data. */
    daftar: '/daftar',
    /** Konfirmasi + QR check-in. */
    booking: (id: string) => `/booking/${id}`,
    /** Status antrean publik. */
    status: '/status',
    /** Kiosk (layar sentuh, kunci perangkat). */
    kiosk: {
      root: '/kiosk',
      checkin: '/kiosk/checkin',
      walkin: '/kiosk/walkin',
    },
    /** Aplikasi loket (login staf). */
    loket: '/loket',
    /** Display TV per instansi (kunci perangkat). */
    display: (instansiId: string) => `/display/${instansiId}`,
  },
  /** Login staf MPP. */
  signIn: '/signin',
  /**
   * Others
   */
  blank: '/blank',
  components: '/components',
};
