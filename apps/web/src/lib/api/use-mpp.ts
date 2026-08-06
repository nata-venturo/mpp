'use client';

import type { WalkInPayload, AntrianActionKind, CreateBookingPayload } from './mpp';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

import {
  mppKeys,
  checkIn,
  getQueue,
  callNext,
  getBooking,
  actOnAntrian,
  getLoketList,
  createBooking,
  getLayananList,
  registerWalkIn,
  getAvailability,
  setLoketSession,
  getInstansiList,
  getDisplaySnapshot,
  getKioskLayananList,
  getKioskInstansiList,
} from './mpp';

// ----------------------------------------------------------------------
// Katalog — jarang berubah, jadi stale time panjang.

const CATALOG_STALE_TIME = 5 * 60 * 1000;

export function useInstansiQuery(options: { device?: boolean } = {}) {
  const { device = false } = options;

  return useQuery({
    queryKey: [...mppKeys.instansi, device ? 'device' : 'public'],
    queryFn: ({ signal }) =>
      device ? getKioskInstansiList({ signal }) : getInstansiList({ signal }),
    staleTime: CATALOG_STALE_TIME,
  });
}

export function useLayananQuery(instansiId: string, options: { device?: boolean } = {}) {
  const { device = false } = options;

  return useQuery({
    queryKey: [...mppKeys.layanan(instansiId), device ? 'device' : 'public'],
    queryFn: ({ signal }) =>
      device ? getKioskLayananList(instansiId, { signal }) : getLayananList(instansiId, { signal }),
    enabled: Boolean(instansiId),
    staleTime: CATALOG_STALE_TIME,
  });
}

// ----------------------------------------------------------------------
// Pendaftaran

export function useAvailabilityQuery(params: {
  instansiId: string;
  layananId: string;
  date: string;
}) {
  const { instansiId, layananId, date } = params;

  return useQuery({
    queryKey: mppKeys.availability(instansiId, layananId, date),
    queryFn: ({ signal }) =>
      getAvailability({ instansi_id: instansiId, layanan_id: layananId, date }, { signal }),
    enabled: Boolean(instansiId && date),
    // Kuota bergerak karena orang lain ikut mendaftar; jangan pakai cache lama.
    staleTime: 15 * 1000,
  });
}

export function useCreateBookingMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreateBookingPayload) => createBooking(payload),
    onSuccess: (_booking, variables) => {
      // Kuota tanggal itu baru saja berkurang satu.
      queryClient.invalidateQueries({
        queryKey: mppKeys.availability(
          variables.instansi_id,
          variables.layanan_id,
          variables.tanggal
        ),
      });
    },
  });
}

export function useBookingDetailQuery(id: string) {
  return useQuery({
    queryKey: mppKeys.booking(id),
    queryFn: ({ signal }) => getBooking(id, { signal }),
    enabled: Boolean(id),
  });
}

// ----------------------------------------------------------------------
// Kiosk

export function useCheckInMutation() {
  return useMutation({ mutationFn: (token: string) => checkIn(token) });
}

export function useWalkInMutation() {
  return useMutation({ mutationFn: (payload: WalkInPayload) => registerWalkIn(payload) });
}

// ----------------------------------------------------------------------
// Loket

export function useQueueQuery(layananId: string, options: { refetchInterval?: number } = {}) {
  return useQuery({
    queryKey: mppKeys.queue(layananId),
    queryFn: ({ signal }) => getQueue(layananId, { signal }),
    enabled: Boolean(layananId),
    // WebSocket adalah jalur utama; polling di sini hanya jaring pengaman
    // saat socket putus.
    refetchInterval: options.refetchInterval ?? 30_000,
  });
}

export function useLoketQuery(instansiId: string) {
  return useQuery({
    queryKey: mppKeys.loket(instansiId),
    queryFn: ({ signal }) => getLoketList(instansiId, { signal }),
    enabled: Boolean(instansiId),
  });
}

export function useLoketSessionMutation() {
  return useMutation({
    mutationFn: (vars: { loketId: string; action: 'open' | 'close' }) =>
      setLoketSession(vars.loketId, vars.action),
  });
}

export function useCallNextMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (loketId: string) => callNext(loketId),
    onSuccess: (result) => {
      if (result) {
        queryClient.invalidateQueries({ queryKey: mppKeys.queue(result.layanan_id) });
      }
    },
  });
}

export function useAntrianActionMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { id: string; kind: AntrianActionKind }) => actOnAntrian(vars.id, vars.kind),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: mppKeys.queue(result.layanan_id) });
    },
  });
}

// ----------------------------------------------------------------------
// TV

export function useDisplayQuery(instansiId: string) {
  return useQuery({
    queryKey: mppKeys.display(instansiId),
    queryFn: ({ signal }) => getDisplaySnapshot(instansiId, { signal }),
    enabled: Boolean(instansiId),
    // Snapshot menutupi socket yang putus (NFR-AVAIL-02): layar tetap
    // menampilkan keadaan terakhir yang diketahui, bukan kosong.
    refetchInterval: 20_000,
  });
}
