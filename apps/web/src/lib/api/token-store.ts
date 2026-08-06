// ----------------------------------------------------------------------
// Penyimpanan token staf (loket / FO / admin).
//
// Sumber kebenaran adalah variabel di memori; localStorage hanya cermin
// supaya sesi bertahan saat reload. Membaca localStorage pada setiap
// request akan gagal di server (tidak ada window) dan lambat di klien.
//
// Kiosk & TV TIDAK memakai store ini — perangkat membawa API key
// ber-scope, lihat device-client.ts.

const STORAGE_KEY = 'mpp.staff.token';

type Listener = (token: string | null) => void;

let accessToken: string | null = null;
const listeners = new Set<Listener>();

function notify() {
  listeners.forEach((listener) => listener(accessToken));
}

/** Token aktif, atau null bila belum/tidak lagi login. */
export function getAccessToken(): string | null {
  if (accessToken !== null) return accessToken;

  // Hidrasi sekali dari localStorage saat modul dipakai pertama kali di
  // browser (mis. setelah reload halaman).
  if (typeof window !== 'undefined') {
    accessToken = window.localStorage.getItem(STORAGE_KEY);
  }

  return accessToken;
}

export function setAccessToken(token: string | null) {
  accessToken = token;

  if (typeof window !== 'undefined') {
    if (token) {
      window.localStorage.setItem(STORAGE_KEY, token);
    } else {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  }

  notify();
}

export function clearAccessToken() {
  setAccessToken(null);
}

/** Berlangganan perubahan token (dipakai guard rute staf). */
export function subscribeToken(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
