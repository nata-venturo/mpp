import { z } from 'zod';

import { apiFetch } from './client';
import { endpoints } from './endpoints';
import { setAccessToken, clearAccessToken } from './token-store';

// ----------------------------------------------------------------------
// Auth staf (petugas loket / front office / supervisor / admin).
// Skema divalidasi di batas fetch — hanya field yang benar-benar dipakai
// yang diwajibkan, sisanya dibiarkan lewat.

export const signInResultSchema = z.object({
  access_token: z.string().min(1),
  refresh_token: z.string().nullish(),
  user: z
    .object({
      id: z.string(),
      email: z.string().nullish(),
      full_name: z.string().nullish(),
    })
    .nullish(),
});

export type SignInResult = z.infer<typeof signInResultSchema>;

export type SignInPayload = {
  /** Email atau username. */
  login: string;
  password: string;
};

export const authKeys = {
  me: ['auth', 'me'] as const,
};

/**
 * Login staf. Token langsung disimpan di sini supaya tidak ada pemanggil
 * yang bisa lupa menyimpannya (dan lalu terlihat "berhasil login" tapi
 * request berikutnya 401).
 */
export async function signIn(payload: SignInPayload): Promise<SignInResult> {
  const { data } = await apiFetch<unknown>(endpoints.auth.signIn, {
    method: 'post',
    body: payload,
  });

  const result = signInResultSchema.parse(data);
  setAccessToken(result.access_token);

  return result;
}

/** Logout lokal: buang token apa pun hasil panggilan backend. */
export async function signOut(): Promise<void> {
  try {
    await apiFetch(endpoints.auth.logout, { method: 'post' });
  } catch {
    // Sesi backend mungkin sudah mati; membuang token lokal tetap benar.
  } finally {
    clearAccessToken();
  }
}
