import { z } from 'zod';

import { apiFetch } from './client';
import { endpoints } from './endpoints';
import { deviceFetch } from './device-client';

// ----------------------------------------------------------------------
// Kontrak domain antrian MPP, divalidasi di batas fetch (tanpa `as`).
// Drift kontrak backend gagal keras di sini, bukan jauh di dalam komponen.

const nullableList = <T extends z.ZodTypeAny>(item: T) =>
  z
    .array(item)
    .nullish()
    .transform((value) => value ?? []);

// ── Katalog ───────────────────────────────────────────────────────────

export const instansiSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  prefix: z.string(),
  description: z.string().nullish(),
  logo_url: z.string().nullish(),
  queue_mode: z.string(),
  is_active: z.boolean().optional().default(true),
});

export const syaratDokumenSchema = z.object({
  id: z.string(),
  name: z.string(),
  is_required: z.boolean(),
  notes: z.string().nullish(),
  sort: z.number().optional().default(0),
});

export const layananSchema = z.object({
  id: z.string(),
  instansi_id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  estimasi_durasi_menit: z.number(),
  requires_fo_verification: z.boolean().optional().default(false),
  syarat_dokumen: nullableList(syaratDokumenSchema),
});

export const availabilitySchema = z.object({
  date: z.string(),
  kuota: z.number(),
  terpakai: z.number(),
  remaining: z.number(),
});

// ── Booking ───────────────────────────────────────────────────────────

export const bookingSchema = z.object({
  id: z.string(),
  status: z.string(),
  instansi_id: z.string(),
  layanan_id: z.string(),
  tanggal: z.string(),
  channel: z.string(),
  qr_token: z.string().nullish(),
  qr_expires_at: z.string().nullish(),
  created_at: z.string(),
});

const catalogRefSchema = z.object({
  id: z.string(),
  name: z.string(),
  prefix: z.string().optional().default(''),
});

export const bookingDetailSchema = z.object({
  id: z.string(),
  status: z.string(),
  tanggal: z.string(),
  channel: z.string(),
  qr_token: z.string().nullish(),
  qr_expires_at: z.string().nullish(),
  checked_in_at: z.string().nullish(),
  pemohon_name: z.string(),
  instansi: catalogRefSchema,
  layanan: catalogRefSchema,
  created_at: z.string(),
});

// ── Antrian ───────────────────────────────────────────────────────────

export const antrianSchema = z.object({
  antrian_id: z.string(),
  nomor: z.string(),
  nomor_seq: z.number(),
  queue_status: z.string(),
  eta_menit: z.number(),
  instansi_id: z.string(),
  layanan_id: z.string(),
  queued_at: z.string(),
});

export const checkInResultSchema = z.object({
  booking_id: z.string(),
  status: z.string(),
  checked_in_at: z.string(),
  instansi: z.object({ id: z.string(), name: z.string(), prefix: z.string() }),
  layanan: z.object({ id: z.string(), name: z.string() }),
  antrian_id: z.string(),
  nomor: z.string(),
  nomor_seq: z.number(),
  queue_status: z.string(),
  eta_menit: z.number(),
  pemohon_name: z.string(),
});

export const queueItemSchema = z.object({
  antrian_id: z.string(),
  nomor: z.string(),
  status: z.string(),
  source: z.string(),
  call_count: z.number(),
  loket: z.string().nullish(),
  queued_at: z.string(),
});

// ── Loket & aksi antrian ──────────────────────────────────────────────

export const loketSchema = z.object({
  id: z.string(),
  instansi_id: z.string(),
  code: z.string(),
  name: z.string(),
  status: z.string(),
  is_active: z.boolean(),
});

export const loketSessionSchema = z.object({
  session_id: z.string(),
  loket_id: z.string(),
  loket: z.string(),
  is_active: z.boolean(),
  opened_at: z.string(),
  closed_at: z.string().nullish(),
});

export const antrianActionSchema = z.object({
  antrian_id: z.string(),
  nomor: z.string(),
  status: z.string(),
  loket_id: z.string().nullish(),
  loket: z.string().nullish(),
  call_count: z.number(),
  pemohon_name: z.string(),
  layanan_id: z.string(),
  layanan: z.string(),
  tts_text: z.string().optional().default(''),
  durasi_detik: z.number().nullish(),
  done_at: z.string().nullish(),
});

// ── Display ───────────────────────────────────────────────────────────

export const displayCurrentSchema = z.object({
  antrian_id: z.string(),
  nomor: z.string(),
  status: z.string(),
  loket_id: z.string(),
  loket: z.string(),
  tts_text: z.string(),
});

export const displayNextSchema = z.object({
  antrian_id: z.string(),
  nomor: z.string(),
  layanan: z.string(),
});

export const displaySnapshotSchema = z.object({
  instansi: z.object({ id: z.string(), name: z.string(), prefix: z.string() }),
  current: nullableList(displayCurrentSchema),
  next: nullableList(displayNextSchema),
});

// ----------------------------------------------------------------------

export type Instansi = z.infer<typeof instansiSchema>;
export type Layanan = z.infer<typeof layananSchema>;
export type SyaratDokumen = z.infer<typeof syaratDokumenSchema>;
export type Availability = z.infer<typeof availabilitySchema>;
export type Booking = z.infer<typeof bookingSchema>;
export type BookingDetail = z.infer<typeof bookingDetailSchema>;
export type Antrian = z.infer<typeof antrianSchema>;
export type CheckInResult = z.infer<typeof checkInResultSchema>;
export type QueueItem = z.infer<typeof queueItemSchema>;
export type Loket = z.infer<typeof loketSchema>;
export type LoketSession = z.infer<typeof loketSessionSchema>;
export type AntrianAction = z.infer<typeof antrianActionSchema>;
export type DisplaySnapshot = z.infer<typeof displaySnapshotSchema>;

export type CreateBookingPayload = {
  instansi_id: string;
  layanan_id: string;
  tanggal: string;
  pemohon: { name: string; phone: string; email?: string | null; nik?: string | null };
};

export type WalkInPayload = {
  instansi_id: string;
  layanan_id: string;
  pemohon: { name: string; phone: string; email?: string | null };
};

// ----------------------------------------------------------------------
// Query keys — di luar modul 'use client' supaya Server Component yang
// melakukan prefetch mendarat di entri cache yang sama.

export const mppKeys = {
  instansi: ['mpp', 'instansi'] as const,
  layanan: (instansiId: string) => ['mpp', 'layanan', instansiId] as const,
  availability: (instansiId: string, layananId: string, date: string) =>
    ['mpp', 'availability', instansiId, layananId, date] as const,
  booking: (id: string) => ['mpp', 'booking', id] as const,
  queue: (layananId: string) => ['mpp', 'queue', layananId] as const,
  loket: (instansiId: string) => ['mpp', 'loket', instansiId] as const,
  display: (instansiId: string) => ['mpp', 'display', instansiId] as const,
};

// ----------------------------------------------------------------------
// Fetchers

type FetchOptions = { signal?: AbortSignal };

export async function getInstansiList(options: FetchOptions = {}): Promise<Instansi[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.instansi.list, options);
  return nullableList(instansiSchema).parse(data);
}

export async function getLayananList(
  instansiId: string,
  options: FetchOptions = {}
): Promise<Layanan[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.instansi.layanan(instansiId), options);
  return nullableList(layananSchema).parse(data);
}

export async function getAvailability(
  params: { instansi_id: string; layanan_id?: string; date: string },
  options: FetchOptions = {}
): Promise<Availability> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.availability, { params, ...options });
  return availabilitySchema.parse(data);
}

export async function createBooking(payload: CreateBookingPayload): Promise<Booking> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.booking.create, {
    method: 'post',
    body: payload,
  });
  return bookingSchema.parse(data);
}

export async function getBooking(id: string, options: FetchOptions = {}): Promise<BookingDetail> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.booking.details(id), options);
  return bookingDetailSchema.parse(data);
}

/** Check-in kiosk — memakai kunci perangkat, bukan token staf. */
export async function checkIn(token: string): Promise<CheckInResult> {
  const { data } = await deviceFetch<unknown>('kiosk', endpoints.mpp.checkin, {
    method: 'post',
    body: { token },
  });
  return checkInResultSchema.parse(data);
}

/** Pendaftaran walk-in di kiosk. */
export async function registerWalkIn(payload: WalkInPayload): Promise<Antrian> {
  const { data } = await deviceFetch<unknown>('kiosk', endpoints.mpp.walkin, {
    method: 'post',
    body: payload,
  });
  return antrianSchema.parse(data);
}

/** Katalog dari sisi kiosk (kunci perangkat, tanpa login). */
export async function getKioskInstansiList(options: FetchOptions = {}): Promise<Instansi[]> {
  const { data } = await deviceFetch<unknown>('kiosk', endpoints.mpp.instansi.list, options);
  return nullableList(instansiSchema).parse(data);
}

export async function getKioskLayananList(
  instansiId: string,
  options: FetchOptions = {}
): Promise<Layanan[]> {
  const { data } = await deviceFetch<unknown>(
    'kiosk',
    endpoints.mpp.instansi.layanan(instansiId),
    options
  );
  return nullableList(layananSchema).parse(data);
}

export async function getQueue(
  layananId: string,
  options: FetchOptions = {}
): Promise<QueueItem[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.queue, {
    params: { layanan_id: layananId, limit: 50 },
    ...options,
  });
  return nullableList(queueItemSchema).parse(data);
}

export async function getLoketList(
  instansiId: string,
  options: FetchOptions = {}
): Promise<Loket[]> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.loket.list, {
    params: { instansi_id: instansiId },
    ...options,
  });
  return nullableList(loketSchema).parse(data);
}

export async function setLoketSession(
  loketId: string,
  action: 'open' | 'close'
): Promise<LoketSession> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.loket.session(loketId), {
    method: 'post',
    body: { action },
  });
  return loketSessionSchema.parse(data);
}

/**
 * Panggil berikutnya. `null` berarti antrean kosong — itu jawaban yang
 * sah, bukan kegagalan, jadi UI menampilkannya sebagai pesan biasa.
 */
export async function callNext(loketId: string): Promise<AntrianAction | null> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.antrian.next, {
    method: 'post',
    body: { loket_id: loketId },
  });

  if (data === null || data === undefined) return null;
  return antrianActionSchema.parse(data);
}

export type AntrianActionKind = 'recall' | 'start' | 'skip' | 'done';

export async function actOnAntrian(id: string, kind: AntrianActionKind): Promise<AntrianAction> {
  const { data } = await apiFetch<unknown>(endpoints.mpp.antrian[kind](id), { method: 'post' });
  return antrianActionSchema.parse(data);
}

/** Snapshot TV — memakai kunci perangkat TV. */
export async function getDisplaySnapshot(
  instansiId: string,
  options: FetchOptions = {}
): Promise<DisplaySnapshot> {
  const { data } = await deviceFetch<unknown>('tv', endpoints.mpp.display, {
    params: { instansi_id: instansiId },
    ...options,
  });
  return displaySnapshotSchema.parse(data);
}
