import { useEffect, useState } from 'preact/hooks';
import PocketBase, { ClientResponseError } from 'pocketbase';
import type { MeResponse } from '../../shared/types.ts';

// ---------- API ----------

/** Same origin on campus (PocketBase serves the app); set VITE_PB_URL when the app is hosted elsewhere, e.g. Vercel. */
export const pb = new PocketBase(import.meta.env.VITE_PB_URL || location.origin);
pb.autoCancellation(false);

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function toApiError(e: unknown): ApiError {
  if (e instanceof ClientResponseError) {
    if (e.status === 0) return new ApiError(0, "You're offline. Check your connection and try again.");
    return new ApiError(e.status, e.response?.message || e.message || 'Something went wrong');
  }
  return new ApiError(500, e instanceof Error ? e.message : 'Something went wrong');
}

/** Calls a custom /api/garba route. */
export async function api<T>(path: string, init?: { method?: string; body?: unknown }): Promise<T> {
  try {
    return await pb.send<T>('/api/garba' + path, {
      method: init?.method ?? (init?.body !== undefined ? 'POST' : 'GET'),
      body: init?.body,
    });
  } catch (e) {
    throw toApiError(e);
  }
}

/** Runs any PocketBase SDK call with our error shape. */
export async function pbCall<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (e) {
    throw toApiError(e);
  }
}

// ---------- Google sign-in (redirect flow, works on mobile browsers) ----------

const OAUTH_KEY = 'garba:oauth';
export const oauthRedirect = () => `${location.origin}/auth/callback`;

export async function startGoogleSignIn() {
  const methods = await pbCall(() => pb.collection('users').listAuthMethods());
  const google = methods.oauth2?.providers?.find((p) => p.name === 'google');
  if (!methods.oauth2?.enabled || !google) throw new ApiError(503, 'Google sign-in is not set up yet. Use the email code instead.');
  storage.set(OAUTH_KEY, { state: google.state, verifier: google.codeVerifier });
  location.href = google.authURL + encodeURIComponent(oauthRedirect());
}

export async function finishGoogleSignIn(params: URLSearchParams): Promise<MeResponse> {
  const saved = storage.get<{ state: string; verifier: string }>(OAUTH_KEY);
  storage.set(OAUTH_KEY, null);
  if (params.get('error')) throw new ApiError(400, 'Google sign-in was cancelled.');
  if (!saved || saved.state !== params.get('state')) throw new ApiError(400, 'Sign-in expired. Please try again.');
  await pbCall(() => pb.collection('users').authWithOAuth2Code('google', params.get('code') ?? '', saved.verifier, oauthRedirect()));
  return afterSignIn();
}

export async function afterSignIn(): Promise<MeResponse> {
  const me = await api<MeResponse>('/me');
  setMe(me);
  return me;
}

export function signOut() {
  pb.authStore.clear();
  pb.realtime.unsubscribe().catch(() => {});
  setMe(null);
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
      useEffect(() => { const l = () => force((n) => n + 1); listeners.add(l); return () => { listeners.delete(l); }; }, []);
      return value;
    },
  };
}

const storage = {
  get<T>(key: string): T | null {
    try { const v = localStorage.getItem(key); return v ? (JSON.parse(v) as T) : null; } catch { return null; }
  },
  set(key: string, v: unknown) {
    try { if (v === null) localStorage.removeItem(key); else localStorage.setItem(key, JSON.stringify(v)); } catch { /* private mode */ }
  },
};
export { storage };

// The signed-in user's data is cached so the pass and QR show instantly, even offline at the gate.
export const meStore = createStore<MeResponse | null>(pb.authStore.isValid ? storage.get<MeResponse>('garba:me') : null);

export function setMe(me: MeResponse | null) {
  storage.set('garba:me', me);
  meStore.set(me);
}

export async function refreshMe(): Promise<MeResponse | null> {
  if (!pb.authStore.isValid) {
    setMe(null);
    return null;
  }
  try {
    return await afterSignIn();
  } catch (e) {
    if (e instanceof ApiError && (e.status === 401 || e.status === 403 || e.status === 404)) signOut();
    return meStore.get();
  }
}

/** Keeps the session fresh: PocketBase tokens are renewed on app start. */
export async function renewSession() {
  if (!pb.authStore.isValid) return;
  try {
    await pb.collection('users').authRefresh();
  } catch (e) {
    if (e instanceof ClientResponseError && (e.status === 401 || e.status === 403 || e.status === 404)) signOut();
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

export const routeStore = createStore(location.pathname);

export function navigate(to: string, replace = false) {
  if (to === location.pathname + location.search) return;
  if (replace) history.replaceState(null, '', to);
  else history.pushState(null, '', to);
  routeStore.set(location.pathname);
  window.scrollTo(0, 0);
}

addEventListener('popstate', () => routeStore.set(location.pathname));

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
