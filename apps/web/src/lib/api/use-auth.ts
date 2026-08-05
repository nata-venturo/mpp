'use client';

import type { SignInPayload } from './auth';

import { useSyncExternalStore } from 'react';
import { useMutation } from '@tanstack/react-query';

import { signIn, signOut } from './auth';
import { getAccessToken, subscribeToken } from './token-store';

// ----------------------------------------------------------------------

export function useSignInMutation() {
  return useMutation({
    mutationFn: (payload: SignInPayload) => signIn(payload),
  });
}

export function useSignOutMutation() {
  return useMutation({ mutationFn: () => signOut() });
}

/**
 * Token staf saat ini, reaktif terhadap login/logout/401.
 *
 * useSyncExternalStore dipakai supaya guard rute ikut berubah saat hook
 * `afterResponse` membuang token yang ditolak backend — tanpa itu, layar
 * loket akan tetap tampak "login" sampai reload berikutnya.
 */
export function useAccessToken(): string | null {
  return useSyncExternalStore(
    subscribeToken,
    () => getAccessToken(),
    // Server render tidak punya sesi: selalu null supaya HTML awal sama.
    () => null
  );
}
