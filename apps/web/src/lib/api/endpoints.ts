// ----------------------------------------------------------------------
// Centralized backend endpoint map (mirror of src/routes/paths.ts for API URLs).
//
// Paths are relative (no leading slash) so they resolve against ky's
// `baseUrl` even when it carries a path prefix.

export const endpoints = {
  articles: {
    list: 'api/articles',
    details: (slug: string) => `api/articles/${encodeURIComponent(slug)}`,
    categories: 'api/article-categories',
  },
  faq: {
    list: 'api/faq',
  },
  siteContent: {
    map: 'api/site-content',
  },
  /**
   * Core auth — dipakai staf (loket/FO/admin). Kiosk & TV tidak lewat sini.
   */
  auth: {
    signIn: 'core/v1/auth/signin',
    me: 'core/v1/auth/me',
    logout: 'core/v1/auth/logout',
  },
  /**
   * Domain antrian MPP. Publik: katalog, availability, booking.
   * Staf (JWT): queue, loket, aksi antrian. Perangkat (X-API-Key):
   * checkin, walkin, display.
   */
  mpp: {
    instansi: {
      list: 'mpp/v1/instansi',
      details: (id: string) => `mpp/v1/instansi/${encodeURIComponent(id)}`,
      layanan: (id: string) => `mpp/v1/instansi/${encodeURIComponent(id)}/layanan`,
    },
    availability: 'mpp/v1/availability',
    booking: {
      create: 'mpp/v1/booking',
      details: (id: string) => `mpp/v1/booking/${encodeURIComponent(id)}`,
    },
    checkin: 'mpp/v1/checkin',
    walkin: 'mpp/v1/walkin',
    queue: 'mpp/v1/queue',
    loket: {
      list: 'mpp/v1/loket',
      session: (id: string) => `mpp/v1/loket/${encodeURIComponent(id)}/session`,
    },
    antrian: {
      next: 'mpp/v1/queue/next',
      recall: (id: string) => `mpp/v1/antrian/${encodeURIComponent(id)}/recall`,
      start: (id: string) => `mpp/v1/antrian/${encodeURIComponent(id)}/start`,
      skip: (id: string) => `mpp/v1/antrian/${encodeURIComponent(id)}/skip`,
      done: (id: string) => `mpp/v1/antrian/${encodeURIComponent(id)}/done`,
    },
    display: 'mpp/v1/display',
    ws: 'mpp/v1/ws',
  },
};
