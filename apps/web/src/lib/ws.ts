'use client';

import { useRef, useState, useEffect } from 'react';

import { CONFIG } from 'src/global-config';

import { endpoints } from './api/endpoints';
import { getAccessToken } from './api/token-store';

// ----------------------------------------------------------------------
// Klien WebSocket MPP.
//
// Satu socket per set kredensial, dipakai bersama oleh semua komponen —
// panel loket bisa mendengarkan beberapa channel tanpa membuka koneksi
// baru per komponen.
//
// Semantik pengiriman adalah at-least-once dan REST tetap sumber
// kebenaran, jadi klien ini sengaja tidak mencoba memutar ulang delta
// yang terlewat: saat tersambung kembali ia berlangganan lagi dan
// server mengirim `snapshot`.

export type WsEvent = {
  type: string;
  channel?: string;
  data?: Record<string, unknown>;
};

type Listener = (event: WsEvent) => void;

/**
 * Status koneksi, dipakai layar untuk memberi tahu operator bahwa yang
 * ia lihat memang hidup. Layar TV tidak berpenjaga: tanpa indikator,
 * socket yang mati terlihat persis seperti antrean yang sedang sepi.
 */
export type WsStatus = 'connecting' | 'open' | 'closed';

type StatusListener = (status: WsStatus) => void;

/** Kredensial socket: token staf, atau kunci perangkat kiosk/TV. */
export type WsAuth = { kind: 'staff' } | { kind: 'device'; apiKey: string };

const RECONNECT_MIN_MS = 1_000;
const RECONNECT_MAX_MS = 15_000;

function socketUrl(auth: WsAuth): string | null {
  const base = CONFIG.wsUrl || CONFIG.apiUrl;
  if (!base) return null;

  const url = new URL(endpoints.mpp.ws, `${base.replace(/\/+$/, '')}/`);
  url.protocol =
    url.protocol === 'https:' ? 'wss:' : url.protocol === 'http:' ? 'ws:' : url.protocol;

  // Browser tidak bisa menyetel header pada WebSocket, jadi kredensial
  // dikirim sebagai query — backend menaikkannya kembali menjadi header
  // sebelum middleware auth berjalan.
  if (auth.kind === 'device') {
    url.searchParams.set('api_key', auth.apiKey);
  } else {
    const token = getAccessToken();
    if (!token) return null;
    url.searchParams.set('token', token);
  }

  return url.toString();
}

class QueueSocket {
  private ws: WebSocket | null = null;

  private listeners = new Set<Listener>();

  private statusListeners = new Set<StatusListener>();

  private status: WsStatus = 'closed';

  private channels = new Set<string>();

  private retry = 0;

  private timer: ReturnType<typeof setTimeout> | null = null;

  private closed = false;

  constructor(private readonly auth: WsAuth) {}

  getStatus() {
    return this.status;
  }

  private setStatus(status: WsStatus) {
    if (this.status === status) return;
    this.status = status;
    this.statusListeners.forEach((listener) => listener(status));
  }

  connect() {
    if (this.ws || this.closed) return;

    const url = socketUrl(this.auth);
    if (!url) return;

    this.setStatus('connecting');

    const ws = new WebSocket(url);
    this.ws = ws;

    ws.onopen = () => {
      this.retry = 0;
      this.sendSubscribe();
      // Status diumumkan SETELAH subscribe terkirim, supaya konsumen yang
      // memuat ulang snapshot saat 'open' tidak mendahului langganannya.
      this.setStatus('open');
    };

    ws.onmessage = (message) => {
      try {
        const event = JSON.parse(message.data) as WsEvent;
        this.listeners.forEach((listener) => listener(event));
      } catch {
        // Frame yang tidak bisa diurai diabaikan: socket ini hanya
        // pelengkap REST, jangan sampai satu frame rusak memutus layar.
      }
    };

    ws.onclose = () => {
      this.ws = null;
      this.setStatus('closed');
      this.scheduleReconnect();
    };

    ws.onerror = () => ws.close();
  }

  private scheduleReconnect() {
    if (this.closed || this.timer) return;

    // Backoff eksponensial: TV di gedung publik bisa kehilangan jaringan
    // berjam-jam; jangan membanjiri server saat ia kembali.
    const delay = Math.min(RECONNECT_MIN_MS * 2 ** this.retry, RECONNECT_MAX_MS);
    this.retry += 1;

    this.timer = setTimeout(() => {
      this.timer = null;
      this.connect();
    }, delay);
  }

  private sendSubscribe() {
    if (this.ws?.readyState !== WebSocket.OPEN || this.channels.size === 0) return;

    this.ws.send(JSON.stringify({ type: 'subscribe', channels: [...this.channels] }));
  }

  subscribe(channels: string[], listener: Listener, onStatus: StatusListener): () => void {
    channels.forEach((channel) => this.channels.add(channel));
    this.listeners.add(listener);
    this.statusListeners.add(onStatus);

    // Beri tahu status saat ini segera: konsumen yang bergabung ke socket
    // yang SUDAH terbuka tidak akan pernah melihat transisi 'open'.
    onStatus(this.status);

    this.connect();
    this.sendSubscribe();

    return () => {
      this.listeners.delete(listener);
      this.statusListeners.delete(onStatus);
    };
  }

  close() {
    this.closed = true;
    if (this.timer) clearTimeout(this.timer);
    this.ws?.close();
    this.ws = null;
  }
}

// Satu socket per kredensial. Kunci perangkat dan token staf tidak
// pernah berbagi koneksi.
const sockets = new Map<string, QueueSocket>();

function socketFor(auth: WsAuth): QueueSocket {
  const key = auth.kind === 'device' ? `device:${auth.apiKey}` : 'staff';

  let socket = sockets.get(key);
  if (!socket) {
    socket = new QueueSocket(auth);
    sockets.set(key, socket);
  }

  return socket;
}

/**
 * Berlangganan channel antrian.
 *
 * Mengembalikan event terakhir dan status koneksi. Layar yang tidak
 * berpenjaga (TV) menampilkan status itu, karena socket mati terlihat
 * sama persis dengan antrean yang sedang sepi.
 */
export function useQueueSocket(
  channels: string[],
  auth: WsAuth,
  onEvent?: (event: WsEvent) => void
): { last: WsEvent | null; status: WsStatus } {
  const [last, setLast] = useState<WsEvent | null>(null);
  const [status, setStatus] = useState<WsStatus>('closed');

  // Callback disimpan di ref supaya penonton yang mendefinisikan handler
  // inline tidak memicu langganan ulang setiap render.
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  const key = channels.join(',');
  const authKey = auth.kind === 'device' ? `device:${auth.apiKey}` : 'staff';

  useEffect(() => {
    if (!key) return undefined;

    const socket = socketFor(
      auth.kind === 'device' ? { kind: 'device', apiKey: auth.apiKey } : auth
    );

    return socket.subscribe(
      key.split(','),
      (event) => {
        setLast(event);
        handlerRef.current?.(event);
      },
      setStatus
    );
    // `auth` sengaja diringkas jadi authKey: objek literal baru setiap
    // render akan membuat langganan lepas-pasang tanpa henti.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, authKey]);

  return { last, status };
}
