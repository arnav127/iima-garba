import { useEffect, useState } from 'preact/hooks';
import PocketBase, { ClientResponseError } from 'pocketbase';
import * as kit from '@iima/kit';
import type { MeResponse } from '../../shared/types.ts';
import { syncClock } from './qr.ts';

// ---------- API ----------

/**
 * The app lives under a sub-path, e.g. https://students.iima.ac.in/garba2026/ (set VITE_BASE at build time).
 * BASE is that prefix without the trailing slash ('' at the root). App routes below are written without it.
 * Sign-in is the platform's: the session cookie reaches the backend as a verified token.
 */
kit.setBase(import.meta.env.BASE_URL);
export const BASE = kit.BASE;
export const asset = kit.asset;
export const ApiError = kit.ApiError;
export type ApiError = kit.ApiError;

/** The PocketBase SDK, for realtime updates and file uploads only; it holds no token (the cookie does the work). */
export const pb = new PocketBase(location.origin + BASE);
pb.autoCancellation(false);

function toApiError(e: unknown): kit.ApiError {
  if (e instanceof kit.ApiError) return e;
  if (e instanceof ClientResponseError) {
    if (e.status === 0) return new kit.ApiError(0, "You're offline. Check your connection and try again.");
    return new kit.ApiError(e.status, e.response?.message || e.message || 'Something went wrong');
  }
  return new kit.ApiError(500, e instanceof Error ? e.message : 'Something went wrong');
}

/** Calls a custom /api/garba route. */
export async function api<T>(path: string, init?: { method?: string; body?: unknown; timeoutMs?: number }): Promise<T> {
  return kit.api<T>('/api/garba' + path, {
    method: init?.method,
    body: init?.body,
    signal: init?.timeoutMs ? AbortSignal.timeout(init.timeoutMs) : undefined,
  });
}

/** Runs any PocketBase SDK call with our error shape. */
export async function pbCall<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (e) {
    throw toApiError(e);
  }
}

// ---------- sign-in (the platform's; works on mobile browsers) ----------

/** After the platform sign-in, the browser comes back here; see SignedIn. */
export const SIGNED_IN = '/signed-in';

/** Sends the browser to the platform sign-in. switchAccount shows Google's account chooser. */
export function startSignIn(switchAccount = false) {
  kit.login(SIGNED_IN, switchAccount);
}

/** Who is signed in to the platform, even if they are not on the Garba list (for the login page). */
export const platformUser = () => kit.me('garba2026');

export async function afterSignIn(): Promise<MeResponse> {
  const me = await api<MeResponse>('/me');
  syncClock(me.serverTime);
  setMe(me);
  return me;
}

export function signOut() {
  pb.realtime.unsubscribe().catch(() => {});
  setMe(null);
  kit.logout('/');
}

// ---------- tiny store ----------

type Listener = () => void;

export function createStore<T>(initial: T) {
  let value = initial;
  const listeners = new Set<Listener>();
  return {
    get: () => value,
    set(next: T) { value = next; listeners.forEach((l) => l()); },
    use(): T {
      const [, force] = useState(0);
      const seen = value;
      useEffect(() => {
        const l = () => force((n) => n + 1);
        listeners.add(l);
        // The value may have changed between this render and subscribing.
        if (value !== seen) force((n) => n + 1);
        return () => { listeners.delete(l); };
      }, []);
      return value;
    },
  };
}

const storage = kit.storage;
export { storage };

// The signed-in user's data is cached so the pass and QR show instantly, even offline at the gate.
export const meStore = createStore<MeResponse | null>(storage.get<MeResponse>('garba:me'));

export function setMe(me: MeResponse | null) {
  storage.set('garba:me', me);
  meStore.set(me);
}

/** Checks who is signed in. Offline, the cached pass stays; signed out or off the list clears it. */
export async function refreshMe(): Promise<MeResponse | null> {
  try {
    return await afterSignIn();
  } catch (e) {
    if (e instanceof kit.ApiError && (e.status === 401 || e.status === 403 || e.status === 404)) {
      pb.realtime.unsubscribe().catch(() => {});
      setMe(null);
      return null;
    }
    return meStore.get();
  }
}

/** Subscribes to pass changes (volunteers and admins). Returns an unsubscribe function. */
export function onPassesChange(cb: () => void): () => void {
  let unsub: (() => Promise<void>) | undefined;
  let closed = false;
  pb.collection('passes').subscribe('*', cb).then((u) => { if (closed) u(); else unsub = u; }).catch(() => {});
  return () => { closed = true; unsub?.().catch(() => {}); };
}

// ---------- router ----------

/** Current app route without the BASE prefix, e.g. '/home'. */
export const appPath = kit.appPath;
export const routeStore = createStore(appPath());

export function navigate(to: string, replace = false) {
  if (to === appPath() + location.search) return;
  if (replace) history.replaceState(null, '', BASE + to);
  else history.pushState(null, '', BASE + to);
  routeStore.set(appPath());
  window.scrollTo(0, 0);
}

addEventListener('popstate', () => routeStore.set(appPath()));

// ---------- toast ----------

export const toastStore = createStore<string | null>(null);
let toastTimer: ReturnType<typeof setTimeout> | undefined;
export function toast(msg: string) {
  clearTimeout(toastTimer);
  toastStore.set(msg);
  toastTimer = setTimeout(() => toastStore.set(null), 3500);
}
export const toastError = (e: unknown) => toast(e instanceof Error ? e.message : 'Something went wrong');

// ---------- helpers ----------

export const initials = (name: string) => name.split(/\s+/).filter(Boolean).map((w) => w[0]).join('').slice(0, 2).toUpperCase();
export const AVATAR_COLORS = ['#e8317a', '#1f9d55', '#f5872a', '#2a5bd7'];
export const firstName = (name: string) => name.split(' ')[0];

const timeFmt = new Intl.DateTimeFormat('en-IN', { hour: 'numeric', minute: '2-digit', hour12: true, timeZone: 'Asia/Kolkata' });
export const istTime = (ms: number) => timeFmt.format(ms).toUpperCase();

export async function share(url: string, text: string) {
  if (navigator.share) {
    try { await navigator.share({ title: 'Garba Night pass', text, url }); return; } catch (e) { if ((e as Error).name === 'AbortError') return; }
  }
  try {
    await navigator.clipboard.writeText(`${text} ${url}`);
    toast('Link copied. Paste it in WhatsApp or a message.');
  } catch {
    prompt('Copy this link', url);
  }
}
