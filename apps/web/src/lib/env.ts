import { z } from 'zod';

// ----------------------------------------------------------------------
// Satu-satunya tempat env dibaca & divalidasi (zod), sekali saat module load.
//
// - Var wajib yang hilang/salah bentuk MENGGAGALKAN build/boot dengan pesan
//   jelas — bukan diam-diam fallback string kosong lalu rusak di runtime
//   (mode gagal yang paling sulit di-debug).
// - Akses `process.env.NEXT_PUBLIC_*` HARUS literal per-key (bukan dinamis /
//   loop) supaya Next meng-inline nilainya ke bundle client saat build.
// - Konsumsi lewat `CONFIG` di src/global-config.ts, jangan baca process.env
//   langsung di tempat lain.

const isProd = process.env.NODE_ENV === 'production';

// Var yang di-set tapi kosong ("") diperlakukan sama dengan tidak di-set.
const emptyToUndefined = (value: string | undefined) => (value === '' ? undefined : value);

const rawEnv = {
  NEXT_PUBLIC_API_URL: emptyToUndefined(process.env.NEXT_PUBLIC_API_URL),
  NEXT_PUBLIC_COMPANY_SLUG: emptyToUndefined(process.env.NEXT_PUBLIC_COMPANY_SLUG),
  NEXT_PUBLIC_CLIENT_SLUG: emptyToUndefined(process.env.NEXT_PUBLIC_CLIENT_SLUG),
  NEXT_PUBLIC_SITE_URL: emptyToUndefined(process.env.NEXT_PUBLIC_SITE_URL),
  NEXT_PUBLIC_ASSETS_DIR: emptyToUndefined(process.env.NEXT_PUBLIC_ASSETS_DIR),
  NEXT_PUBLIC_SHOW_COMPONENTS: emptyToUndefined(process.env.NEXT_PUBLIC_SHOW_COMPONENTS),
  // Kunci API perangkat MPP. Kiosk dan TV TIDAK login sebagai user — mereka
  // membawa API key ber-scope (X-API-Key) yang di-bake saat build. Kosongkan
  // di deploy yang tidak memasang perangkat tersebut.
  NEXT_PUBLIC_KIOSK_API_KEY: emptyToUndefined(process.env.NEXT_PUBLIC_KIOSK_API_KEY),
  NEXT_PUBLIC_TV_API_KEY: emptyToUndefined(process.env.NEXT_PUBLIC_TV_API_KEY),
  // Origin WebSocket (ws:// atau wss://). Kosong -> diturunkan dari
  // NEXT_PUBLIC_API_URL, jadi cukup di-set bila realtime dilayani host lain.
  NEXT_PUBLIC_WS_URL: emptyToUndefined(process.env.NEXT_PUBLIC_WS_URL),
  // Server-only: override URL API internal (mis. DNS service k8s) untuk
  // fetch RSC/sitemap. Tidak pernah masuk bundle client.
  API_URL: typeof window === 'undefined' ? emptyToUndefined(process.env.API_URL) : undefined,
  // Server-only: bearer token webhook POST /api/revalidate (invalidasi ISR
  // on-demand dari backend/CMS). Endpoint nonaktif bila tidak di-set.
  REVALIDATE_TOKEN:
    typeof window === 'undefined' ? emptyToUndefined(process.env.REVALIDATE_TOKEN) : undefined,
};

const schema = z.object({
  NEXT_PUBLIC_API_URL: z.url(),
  NEXT_PUBLIC_COMPANY_SLUG: z.string().min(1),
  NEXT_PUBLIC_CLIENT_SLUG: z.string().default(''),
  // Production wajib diisi (canonical/OG/sitemap); dev boleh fallback localhost.
  NEXT_PUBLIC_SITE_URL: isProd ? z.url() : z.url().default('http://localhost:8002'),
  NEXT_PUBLIC_ASSETS_DIR: z.string().default(''),
  NEXT_PUBLIC_SHOW_COMPONENTS: z.string().default(''),
  NEXT_PUBLIC_KIOSK_API_KEY: z.string().default(''),
  NEXT_PUBLIC_TV_API_KEY: z.string().default(''),
  NEXT_PUBLIC_WS_URL: z.string().default(''),
  API_URL: z.url().optional(),
  REVALIDATE_TOKEN: z.string().min(16).optional(),
});

const parsed = schema.safeParse(rawEnv);

if (!parsed.success) {
  const detail = parsed.error.issues
    .map((issue) => `  - ${issue.path.join('.')}: ${issue.message}`)
    .join('\n');

  throw new Error(
    `[env] Konfigurasi environment tidak valid:\n${detail}\n` +
      '→ Salin .env.example ke .env lalu isi variabelnya (lihat komentar di dalam file).'
  );
}

export const env = {
  ...parsed.data,
  // Trailing slash dinormalisasi supaya `${siteUrl}/path` tak menghasilkan `//`.
  NEXT_PUBLIC_SITE_URL: parsed.data.NEXT_PUBLIC_SITE_URL.replace(/\/+$/, ''),
  NEXT_PUBLIC_SHOW_COMPONENTS: parsed.data.NEXT_PUBLIC_SHOW_COMPONENTS === 'true',
};
