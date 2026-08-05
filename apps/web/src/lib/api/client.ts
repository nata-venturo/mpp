import { z } from 'zod';
import ky, { HTTPError, TimeoutError } from 'ky';

import { CONFIG } from 'src/global-config';

import { getAccessToken, clearAccessToken } from './token-store';

// ----------------------------------------------------------------------
// Response envelope — see marketplace-be/docs/api-contract/README.md
// { data, message, meta: { pagination? }, errors }

/** Locale yang didukung endpoint publik marketplace-be (param `locale`). */
export type ApiLocale = 'id' | 'en';

export const apiPaginationSchema = z.object({
  page: z.number(),
  limit: z.number(),
  total: z.number(),
  total_pages: z.number(),
});

export type ApiPagination = z.infer<typeof apiPaginationSchema>;

export type ApiMeta = {
  // `unknown`, bukan ApiPagination: envelope tidak divalidasi di apiFetch —
  // fetcher yang memakainya wajib mem-parse dulu (lihat getArticles).
  pagination?: unknown;
} | null;

export type ApiEnvelope<T> = {
  data: T;
  message: string;
  meta?: ApiMeta;
  errors?: Record<string, string[]> | null;
};

export class ApiError extends Error {
  status: number;

  errors: Record<string, string[]> | null;

  constructor(status: number, message: string, errors: Record<string, string[]> | null = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.errors = errors;
  }
}

// ----------------------------------------------------------------------

/**
 * The browser always talks to the public API URL. On the server (RSC, sitemap)
 * an internal URL can be preferred via the non-public `API_URL` env — useful when
 * FE and BE share a private network (e.g. k8s) — falling back to the public one.
 */
function getBaseUrl() {
  const raw =
    typeof window === 'undefined' && CONFIG.serverApiUrl ? CONFIG.serverApiUrl : CONFIG.apiUrl;

  // Standard URL resolution drops the last path segment of a slash-less base
  // (new URL('api/x', 'https://h/prefix') → https://h/api/x) — force exactly
  // one trailing slash so a path-prefixed API URL keeps its prefix.
  return raw ? `${raw.replace(/\/+$/, '')}/` : '';
}

/**
 * Shared ky instance for the Go backend. Injects the `X-Company-Slug` tenant
 * header required by all public `/api/*` endpoints. ky forwards Next.js cache
 * options (`next: { revalidate, tags }`) to the patched fetch on the server
 * (ky >= 1.13), and browsers ignore them — one instance serves both runtimes.
 *
 * Hook `beforeRequest` melampirkan token staf bila ada; endpoint publik
 * (katalog, booking, availability) tetap berfungsi tanpa token. Hook
 * `afterResponse` membuang token yang sudah ditolak backend sehingga UI
 * tidak terus memakai kredensial mati.
 */
export const api = ky.create({
  baseUrl: getBaseUrl(),
  timeout: 10_000,
  hooks: {
    beforeRequest: [
      ({ request }) => {
        const token = getAccessToken();
        if (token && !request.headers.has('Authorization')) {
          request.headers.set('Authorization', `Bearer ${token}`);
        }
      },
    ],
    afterResponse: [
      ({ response }) => {
        // 401 = kredensial hilang/kedaluwarsa. Token dibuang di sini —
        // satu tempat — supaya tidak ada layar yang menahan token mati.
        // Redirect diserahkan ke UI (guard rute), bukan ke layer fetch,
        // karena hook ini juga jalan untuk halaman publik.
        if (response.status === 401 && getAccessToken()) {
          clearAccessToken();
        }
        return response;
      },
    ],
  },
  // Pembagian peran retry: ky me-retry di level TRANSPORT (network error /
  // 408/413/429/5xx) hanya untuk GET yang idempoten; retry level QUERY di
  // browser adalah urusan TanStack Query (default 3x). Default ky ikut
  // me-retry PUT/DELETE — non-idempoten, jangan dilonggarkan tanpa sadar.
  retry: { limit: 2, methods: ['get'] },
  // Only send the tenant header when configured — an empty `X-Company-Slug:`
  // would still be transmitted and rejected by the backend middleware.
  headers: CONFIG.companySlug ? { 'X-Company-Slug': CONFIG.companySlug } : undefined,
});

export type ApiFetchOptions = {
  /** Query string params — empty string / null / undefined entries are skipped. */
  params?: Record<string, string | number | boolean | null | undefined>;
  method?: 'get' | 'post' | 'put' | 'delete';
  body?: unknown;
  signal?: AbortSignal;
  /** Next.js Data Cache options — honored on the server, ignored by browsers. */
  next?: NextFetchRequestConfig;
  cache?: RequestCache;
};

function cleanParams(params: ApiFetchOptions['params']) {
  const search: Record<string, string> = {};

  for (const [key, value] of Object.entries(params ?? {})) {
    if (value !== undefined && value !== null && value !== '') {
      search[key] = String(value);
    }
  }

  return search;
}

/**
 * Typed request against the Go backend: unwraps the response envelope and
 * throws `ApiError` on any failure — non-2xx (status + message + field errors
 * dari backend), timeout, dan network error (keduanya status 0). Callers cukup
 * menangani SATU kelas error; hanya abort (pembatalan TanStack/navigasi) yang
 * diteruskan apa adanya karena bukan kegagalan.
 *
 * NOTE: paths are relative (no leading slash) so they resolve against
 * `baseUrl` even when it carries a path prefix — use the `endpoints` map,
 * which follows this convention.
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {}
): Promise<ApiEnvelope<T>> {
  if (!CONFIG.companySlug) {
    throw new ApiError(
      0,
      'Company slug is not configured — set NEXT_PUBLIC_COMPANY_SLUG in .env (required X-Company-Slug header)'
    );
  }

  return runRequest<T>(api, path, options);
}

/**
 * Inti request bersama: dipakai `apiFetch` (instance publik/staf) dan
 * `deviceFetch` (instance ber-API-key). Penanganan error dijaga di SATU
 * tempat supaya kiosk/TV memunculkan kelas error yang sama dengan web.
 */
export async function runRequest<T>(
  instance: typeof api,
  path: string,
  { params, method = 'get', body, ...options }: ApiFetchOptions = {}
): Promise<ApiEnvelope<T>> {
  if (!getBaseUrl()) {
    throw new ApiError(0, 'API URL is not configured — set NEXT_PUBLIC_API_URL in .env');
  }

  try {
    return await instance<ApiEnvelope<T>>(path, {
      method,
      searchParams: cleanParams(params),
      ...(body !== undefined && { json: body }),
      ...options,
    }).json();
  } catch (error) {
    if (error instanceof HTTPError) {
      const payload = (await error.response.json().catch(() => null)) as ApiEnvelope<null> | null;

      throw new ApiError(
        error.response.status,
        payload?.message || error.message,
        payload?.errors ?? null
      );
    }

    if (error instanceof TimeoutError) {
      throw new ApiError(0, `Request timeout: ${path}`);
    }

    // fetch melempar TypeError untuk kegagalan jaringan (DNS, refused, offline).
    if (error instanceof TypeError) {
      throw new ApiError(0, `Network error: ${error.message}`);
    }

    // Sisanya (terutama AbortError dari pembatalan) diteruskan apa adanya.
    throw error;
  }
}
