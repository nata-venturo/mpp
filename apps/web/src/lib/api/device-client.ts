import type { ApiEnvelope, ApiFetchOptions } from './client';

import { CONFIG } from 'src/global-config';

import { api, ApiError, runRequest } from './client';

// ----------------------------------------------------------------------
// Klien perangkat (kiosk & TV).
//
// Perangkat tidak login sebagai user: keduanya membawa API key ber-scope
// pada header `X-API-Key`. Karena itu instance-nya TERPISAH dari `api` —
// mencampur token staf dengan kunci perangkat akan membuat satu layar
// diam-diam memakai wewenang layar lain.

export type DeviceRole = 'kiosk' | 'tv';

function keyFor(role: DeviceRole): string {
  return role === 'kiosk' ? CONFIG.kioskApiKey : CONFIG.tvApiKey;
}

/** Apakah kunci untuk perangkat ini sudah dikonfigurasi. */
export function isDeviceConfigured(role: DeviceRole): boolean {
  return Boolean(keyFor(role));
}

const deviceApi = api.extend({
  // Perangkat berada di jaringan lokal gedung dan menunggu manusia di
  // depannya; beri sedikit lebih banyak waktu daripada web publik.
  timeout: 15_000,
});

/**
 * Request atas nama sebuah perangkat. Kunci dilampirkan per-request
 * (bukan pada instance) supaya satu halaman kiosk yang juga membaca
 * display tidak perlu instance ketiga.
 */
export async function deviceFetch<T>(
  role: DeviceRole,
  path: string,
  options: ApiFetchOptions = {}
): Promise<ApiEnvelope<T>> {
  const key = keyFor(role);

  if (!key) {
    throw new ApiError(
      0,
      role === 'kiosk'
        ? 'Kiosk belum dikonfigurasi — set NEXT_PUBLIC_KIOSK_API_KEY di .env'
        : 'Display belum dikonfigurasi — set NEXT_PUBLIC_TV_API_KEY di .env'
    );
  }

  return runRequest<T>(deviceApi.extend({ headers: { 'X-API-Key': key } }), path, options);
}
