'use client';

// ----------------------------------------------------------------------
// Antrean suara TV (BR-18).
//
// Satu mini-PC menyalakan tiga jendela display yang berbagi SATU
// speaker. Kalau setiap jendela bicara sendiri, tiga pengumuman
// bertumpuk dan tidak ada yang terdengar jelas. Karena itu:
//
//   1. Satu jendela dipilih sebagai LEADER lewat BroadcastChannel
//      (fallback: kunci di localStorage bila API itu tak tersedia).
//   2. Hanya leader yang memutar audio.
//   3. Leader memutar SATU pengumuman pada satu waktu — berikutnya
//      menunggu yang sekarang selesai.
//   4. Pengumuman yang sama (antrian + jumlah panggilan) tidak diputar
//      dua kali, karena pengiriman event bersifat at-least-once.

const CHANNEL_NAME = 'mpp-tv-audio';
const LOCK_KEY = 'mpp.tv.audio.leader';
const LOCK_TTL_MS = 5_000;
const HEARTBEAT_MS = 2_000;

export type Utterance = {
  /** Kunci dedupe, mis. `${antrian_id}:${call_count}`. */
  id: string;
  text: string;
};

type LeaderState = { id: string; at: number };

function now() {
  return Date.now();
}

/**
 * Pemilihan leader.
 *
 * localStorage adalah wasit sebenarnya (satu nilai, terlihat semua
 * jendela); BroadcastChannel hanya mempercepat pemberitahuan. Klaim
 * kedaluwarsa setelah LOCK_TTL_MS, jadi jendela leader yang ditutup
 * tidak membuat gedung kehilangan suara secara permanen.
 */
export class AudioLeader {
  private readonly id = Math.random().toString(36).slice(2);

  private channel: BroadcastChannel | null = null;

  private timer: ReturnType<typeof setInterval> | null = null;

  private leading = false;

  private onChange?: (leading: boolean) => void;

  start(onChange?: (leading: boolean) => void) {
    this.onChange = onChange;

    if (typeof window !== 'undefined' && 'BroadcastChannel' in window) {
      this.channel = new BroadcastChannel(CHANNEL_NAME);
      this.channel.onmessage = () => this.evaluate();
    }

    this.evaluate();
    this.timer = setInterval(() => this.evaluate(), HEARTBEAT_MS);
  }

  stop() {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;

    if (this.leading) {
      // Lepaskan kunci supaya jendela lain langsung bisa mengambil alih.
      try {
        const raw = window.localStorage.getItem(LOCK_KEY);
        if (raw && (JSON.parse(raw) as LeaderState).id === this.id) {
          window.localStorage.removeItem(LOCK_KEY);
        }
      } catch {
        // Storage tidak tersedia — kunci akan kedaluwarsa sendiri.
      }
      this.channel?.postMessage('released');
    }

    this.channel?.close();
    this.channel = null;
    this.leading = false;
  }

  isLeading() {
    return this.leading;
  }

  private evaluate() {
    if (typeof window === 'undefined') return;

    let state: LeaderState | null = null;
    try {
      const raw = window.localStorage.getItem(LOCK_KEY);
      state = raw ? (JSON.parse(raw) as LeaderState) : null;
    } catch {
      state = null;
    }

    const expired = !state || now() - state.at > LOCK_TTL_MS;
    const mine = state?.id === this.id;

    if (mine || expired) {
      try {
        window.localStorage.setItem(LOCK_KEY, JSON.stringify({ id: this.id, at: now() }));
      } catch {
        // Tanpa storage, satu-satunya pilihan aman adalah tetap bisu:
        // lebih baik tidak ada suara daripada tiga suara bertumpuk.
        return;
      }
      this.setLeading(true);
      return;
    }

    this.setLeading(false);
  }

  private setLeading(value: boolean) {
    if (this.leading === value) return;
    this.leading = value;
    this.onChange?.(value);
  }
}

/**
 * Antrean FIFO yang memutar satu pengumuman pada satu waktu.
 *
 * `speak` diinjeksi supaya mesin suara bisa diganti tanpa menyentuh
 * logika antrean: potongan audio pra-render (pilihan utama, offline),
 * agen TTS lokal, atau `speechSynthesis` sebagai upaya terakhir.
 */
export class AudioQueue {
  private queue: Utterance[] = [];

  private spoken = new Set<string>();

  private playing = false;

  constructor(private readonly speak: (text: string) => Promise<void>) {}

  /** Masukkan pengumuman; duplikat diabaikan. */
  push(utterance: Utterance) {
    if (this.spoken.has(utterance.id)) return;

    this.spoken.add(utterance.id);
    this.queue.push(utterance);
    void this.drain();
  }

  clear() {
    this.queue = [];
  }

  private async drain() {
    if (this.playing) return;
    this.playing = true;

    while (this.queue.length > 0) {
      const next = this.queue.shift();
      if (!next) break;

      try {
        await this.speak(next.text);
      } catch {
        // Satu pengumuman gagal tidak boleh menghentikan yang berikutnya.
      }
    }

    this.playing = false;
  }
}

/**
 * Mesin suara offline berbasis `speechSynthesis`.
 *
 * Urutan yang direkomendasikan dokumen adalah potongan audio pra-render
 * di `public/audio/` (deterministik, tanpa mesin apa pun), lalu agen TTS
 * lokal, lalu ini. Yang terakhir dipakai di sini karena tidak menuntut
 * aset yang belum ada di repo — dan semuanya tetap lokal, tanpa jaringan.
 *
 * ponytail: ganti implementasi ini dengan pemutar potongan audio saat
 * aset `public/audio/*.mp3` sudah tersedia; kontraknya cuma
 * `(text) => Promise<void>`.
 */
export function createSpeechEngine(lang = 'id-ID'): (text: string) => Promise<void> {
  return (text: string) =>
    new Promise<void>((resolve) => {
      if (typeof window === 'undefined' || !('speechSynthesis' in window)) {
        resolve();
        return;
      }

      const utterance = new SpeechSynthesisUtterance(text);
      utterance.lang = lang;
      utterance.rate = 0.95;

      const indonesian = window.speechSynthesis
        .getVoices()
        .find((voice) => voice.lang?.toLowerCase().startsWith('id'));
      if (indonesian) utterance.voice = indonesian;

      utterance.onend = () => resolve();
      // Suara yang gagal tetap harus melepas antrean, kalau tidak seluruh
      // sisa pengumuman ikut terkunci.
      utterance.onerror = () => resolve();

      window.speechSynthesis.speak(utterance);
    });
}
