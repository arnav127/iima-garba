import { useEffect, useRef, useState } from 'preact/hooks';
import type { MeResponse, ScanResult } from '../../../shared/types.ts';
import { Btn, Logo } from '../components/ui.tsx';
import { api, onPassesChange, storage } from '../lib.ts';
import { AccountSheet } from './Home.tsx';

const LOOK = {
  allowed: { bg: 'var(--ok)', icon: '✓' },
  used: { bg: 'var(--bad)', icon: '✕' },
  revoked: { bg: 'var(--bad)', icon: '✕' },
  invalid: { bg: 'var(--bad)', icon: '✕' },
  unclaimed: { bg: 'var(--orange)', icon: '!' },
  idle: { bg: 'var(--yellow)', icon: '◎' },
} as const;

type CamState = 'starting' | 'on' | 'blocked' | 'none';

export function Scan({ me }: { me: MeResponse }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [gate, setGate] = useState(() => Math.min(storage.get<number>('garba:gate') ?? 1, me.event.gates));
  const [count, setCount] = useState<number | null>(null);
  const [result, setResult] = useState<ScanResult | null>(null);
  const [cam, setCam] = useState<CamState>('starting');
  const [manual, setManual] = useState(false);
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [account, setAccount] = useState(false);
  const gateRef = useRef(gate);
  const last = useRef({ payload: '', at: 0, busy: false });
  gateRef.current = gate;

  async function check(payload: string) {
    const l = last.current;
    if (l.busy || (payload === l.payload && Date.now() - l.at < 3000)) return;
    l.busy = true; l.payload = payload; l.at = Date.now();
    setBusy(true);
    try {
      const r = await api<ScanResult>('/scan', { body: { payload, gate: gateRef.current } });
      setResult(r);
      setCount(r.entered);
      navigator.vibrate?.(r.outcome === 'allowed' ? 80 : [120, 80, 120]);
    } catch (e) {
      setResult({ outcome: 'invalid', title: 'Could not check', sub: (e as Error).message, name: '—', meta: 'Try again', entered: count ?? 0 });
      l.payload = '';
    } finally {
      l.busy = false;
      setBusy(false);
    }
  }

  // Camera + QR decoding (loaded only on this screen).
  useEffect(() => {
    let scanner: { start: () => Promise<void>; stop: () => void; destroy: () => void } | undefined;
    let dead = false;
    import('qr-scanner').then(async ({ default: QrScanner }) => {
      if (dead || !videoRef.current) return;
      if (!(await QrScanner.hasCamera())) { setCam('none'); return; }
      const s = new QrScanner(videoRef.current, (r) => check(r.data), {
        preferredCamera: 'environment', maxScansPerSecond: 12, returnDetailedScanResult: true,
        highlightScanRegion: false, highlightCodeOutline: false,
      });
      scanner = s;
      try { await s.start(); if (!dead) setCam('on'); } catch { if (!dead) setCam('blocked'); }
    }).catch(() => setCam('none'));
    return () => { dead = true; scanner?.stop(); scanner?.destroy(); };
  }, []);

  // Live entry count across all gates.
  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | undefined;
    const load = () => api<{ entered: number }>('/scan/count').then((r) => setCount(r.entered)).catch(() => {});
    const debounced = () => { clearTimeout(t); t = setTimeout(load, 300); };
    load();
    const unsub = onPassesChange(debounced);
    const poll = setInterval(load, 10_000);
    return () => { unsub(); clearInterval(poll); clearTimeout(t); };
  }, []);

  const nextGate = () => {
    const g = (gate % me.event.gates) + 1;
    setGate(g);
    storage.set('garba:gate', g);
  };

  const look = LOOK[result?.outcome ?? 'idle'];
  const camMsg = { starting: 'Starting camera…', on: '', blocked: 'Camera blocked. Allow camera access for this site, or type the pass code.', none: 'No camera found. Type the pass code instead.' }[cam];

  return (
    <div class="screen" style={{ background: 'repeating-linear-gradient(45deg,#1d1d1d 0 10px,#181818 10px 20px)', minHeight: '100dvh', overflow: 'hidden' }}>
      <video ref={videoRef} muted playsInline style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', opacity: cam === 'on' ? 1 : 0 }} />
      {camMsg && <div style={{ position: 'absolute', top: 'calc(var(--safe-t) + 106px)', left: '50%', marginLeft: -100, width: 200, height: 240, display: 'grid', placeItems: 'center', textAlign: 'center', font: '500 11px/1.6 ui-monospace, monospace', color: 'rgba(246,240,228,.55)' }}>{camMsg}</div>}

      <div style={{ position: 'relative', padding: 'calc(var(--safe-t) + 20px) 20px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'center', color: 'var(--ink)' }}>
        <button onClick={nextGate} style={{ padding: '8px 14px', borderRadius: 10, background: 'var(--yellow)', font: "700 13px var(--fb)" }} title="Tap to change gate">
          GATE {gate} · {me.user.role === 'admin' ? 'ADMIN' : 'VOLUNTEER'}
        </button>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <div style={{ padding: '8px 14px', borderRadius: 10, background: 'var(--ivory)', font: "700 13px var(--fb)" }}>{count ?? '…'} IN</div>
          <button onClick={() => setAccount(true)} aria-label="Menu"><Logo size={34} /></button>
        </div>
      </div>

      <div style={{ position: 'absolute', top: 'calc(var(--safe-t) + 106px)', left: '50%', marginLeft: -120, width: 240, height: 240, border: '3px solid var(--ivory)', borderRadius: 24, boxShadow: '0 0 0 6px var(--ink), 0 0 0 9px var(--pink)', pointerEvents: 'none' }} />

      <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, background: 'var(--ivory)', borderRadius: '28px 28px 0 0', overflow: 'hidden' }}>
        <div style={{ height: 14, background: `repeating-linear-gradient(90deg,${look.bg} 0 12px,var(--ink) 12px 16px)` }} />
        <div style={{ padding: '18px 22px calc(34px + var(--safe-b))' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14 }} aria-live="polite">
            <div style={{ width: 56, height: 56, flex: 'none', borderRadius: 14, background: look.bg, color: result ? '#fff' : 'var(--ink)', display: 'grid', placeItems: 'center', font: '800 28px var(--fd)', border: '2px solid var(--ink)' }}>{busy ? '…' : look.icon}</div>
            <div style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
              <span style={{ font: '800 30px/1 var(--fd)', fontStretch: '75%' }}>{(result?.title ?? 'Ready to scan').toUpperCase()}</span>
              <span style={{ font: "500 14px var(--fb)", color: 'var(--body)' }}>{result?.sub ?? 'Point the camera at the pass QR'}</span>
            </div>
          </div>
          {result && (
            <div style={{ marginTop: 16, borderTop: '2px solid var(--ink)', paddingTop: 12, display: 'flex', flexDirection: 'column' }}>
              <span style={{ font: "700 18px var(--fb)" }}>{result.name}</span>
              <span style={{ font: "400 13px var(--fb)", color: 'var(--muted)' }}>{result.meta}</span>
            </div>
          )}
          {manual && (
            <form style={{ marginTop: 14, display: 'flex', gap: 8 }} onSubmit={(e) => { e.preventDefault(); last.current.payload = ''; check(code.trim()); }}>
              <input class="input" value={code} onInput={(e) => setCode(e.currentTarget.value.toUpperCase())} placeholder="GRB-0417" autoCapitalize="characters" spellcheck={false} style={{ flex: 1 }} autoFocus />
              <button class="btn small" style={{ width: 'auto' }} disabled={busy || code.trim().length < 5}>Check</button>
            </form>
          )}
          <div style={{ marginTop: 14 }}>
            <Btn onClick={() => { setResult(null); setCode(''); last.current.payload = ''; }}>Scan next</Btn>
          </div>
          {!manual && (
            <button onClick={() => setManual(true)} style={{ marginTop: 12, width: '100%', textAlign: 'center', font: "500 13px var(--fb)", color: 'var(--muted)', textDecoration: 'underline' }}>
              QR not scanning? Type the pass code
            </button>
          )}
        </div>
      </div>
      {account && <AccountSheet me={me} onClose={() => setAccount(false)} />}
    </div>
  );
}
